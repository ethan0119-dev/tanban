package database

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func Open(dsn string) (*sql.DB, error) {
	dsn, err := beijingDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := verifyMySQL8(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// beijingDSN makes both sides of every MySQL connection agree on the time
// contract. Loc controls DATETIME parsing in the Go driver, while time_zone is
// applied by the driver whenever the pool opens a new MySQL session.
func beijingDSN(dsn string) (string, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("parse database DSN: %w", err)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", fmt.Errorf("load Beijing timezone: %w", err)
	}
	config.Loc = location
	config.ParseTime = true
	config.CheckConnLiveness = true
	if config.Collation == "" {
		config.Collation = "utf8mb4_unicode_ci"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.Params == nil {
		config.Params = make(map[string]string)
	}
	config.Params["charset"] = "utf8mb4"
	config.Params["time_zone"] = "'+08:00'"
	return config.FormatDSN(), nil
}

func verifyMySQL8(ctx context.Context, db *sql.DB) error {
	var version, versionComment, sqlMode, timezone, charset, collation, isolation, storageEngine string
	err := db.QueryRowContext(ctx, `SELECT VERSION(),@@version_comment,@@SESSION.sql_mode,
		@@SESSION.time_zone,@@SESSION.character_set_connection,@@SESSION.collation_connection,
		@@SESSION.transaction_isolation,@@default_storage_engine`).
		Scan(&version, &versionComment, &sqlMode, &timezone, &charset, &collation, &isolation, &storageEngine)
	if err != nil {
		return fmt.Errorf("inspect database compatibility: %w", err)
	}
	major := 0
	if _, err = fmt.Sscanf(version, "%d.", &major); err != nil || major < 8 || strings.Contains(strings.ToLower(version), "mariadb") || strings.Contains(strings.ToLower(versionComment), "mariadb") {
		return fmt.Errorf("unsupported database server %q (%s): MySQL 8 or newer is required", version, versionComment)
	}
	modes := make(map[string]bool)
	for _, mode := range strings.Split(sqlMode, ",") {
		modes[strings.TrimSpace(mode)] = true
	}
	if !modes["ONLY_FULL_GROUP_BY"] || (!modes["STRICT_TRANS_TABLES"] && !modes["STRICT_ALL_TABLES"]) || !modes["NO_ZERO_DATE"] {
		return fmt.Errorf("incompatible MySQL sql_mode %q: ONLY_FULL_GROUP_BY, strict mode and NO_ZERO_DATE are required", sqlMode)
	}
	if timezone != "+08:00" {
		return fmt.Errorf("incompatible MySQL session time_zone %q: +08:00 is required", timezone)
	}
	if charset != "utf8mb4" || !strings.HasPrefix(collation, "utf8mb4_") {
		return fmt.Errorf("incompatible MySQL connection encoding %s/%s: utf8mb4 is required", charset, collation)
	}
	if isolation != "REPEATABLE-READ" {
		return fmt.Errorf("incompatible MySQL transaction isolation %q: REPEATABLE-READ is required", isolation)
	}
	if !strings.EqualFold(storageEngine, "InnoDB") {
		return fmt.Errorf("incompatible MySQL storage engine %q: InnoDB is required", storageEngine)
	}
	return nil
}

func Migrate(ctx context.Context, db *sql.DB, dir string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
version VARCHAR(255) PRIMARY KEY, applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, statement := range splitSQL(string(body)) {
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				// MySQL auto-commits DDL even inside a transaction. If a process
				// stops halfway through a migration, replay already-applied ADD COLUMN
				// or ADD KEY statements and continue to the remaining statements.
				if isAlreadyAppliedDDL(err) {
					continue
				}
				_ = tx.Rollback()
				return fmt.Errorf("apply %s: %w", name, err)
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(?)", name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func isAlreadyAppliedDDL(err error) bool {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	return mysqlErr.Number == 1060 || mysqlErr.Number == 1061 || mysqlErr.Number == 1091
}

func splitSQL(input string) []string {
	var statements []string
	var current strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		if strings.HasSuffix(line, ";") {
			statement := strings.TrimSpace(strings.TrimSuffix(current.String(), ";\n"))
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		statements = append(statements, tail)
	}
	return statements
}

func IsDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
