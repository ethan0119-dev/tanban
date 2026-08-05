package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysql "github.com/go-sql-driver/mysql"
)

func TestAlreadyAppliedDDL(t *testing.T) {
	t.Parallel()
	for _, code := range []uint16{1060, 1061, 1091} {
		if !isAlreadyAppliedDDL(&mysql.MySQLError{Number: code, Message: "already exists"}) {
			t.Fatalf("expected MySQL error %d to be replay-safe", code)
		}
	}
	if isAlreadyAppliedDDL(errors.New("network error")) {
		t.Fatal("unrelated errors must not be ignored")
	}
}

func TestBeijingDSNForcesDriverAndMySQLSessionTimezone(t *testing.T) {
	t.Parallel()
	dsn, err := beijingDSN("tanban:test@tcp(127.0.0.1:3306)/tanban?parseTime=true&loc=Local")
	if err != nil {
		t.Fatal(err)
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Loc.String(); got != "Asia/Shanghai" {
		t.Fatalf("driver location=%q", got)
	}
	if got := config.Params["time_zone"]; got != "'+08:00'" {
		t.Fatalf("MySQL session timezone=%q", got)
	}
	if !config.ParseTime {
		t.Fatal("MySQL DATETIME parsing must always be enabled")
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Fatalf("MySQL connection charset is missing from DSN: %s", dsn)
	}
	if got := config.Collation; got != "utf8mb4_unicode_ci" {
		t.Fatalf("MySQL connection collation=%q", got)
	}
	if config.Timeout != 10*time.Second || config.ReadTimeout != 30*time.Second || config.WriteTimeout != 30*time.Second {
		t.Fatalf("unexpected MySQL timeouts: dial=%s read=%s write=%s", config.Timeout, config.ReadTimeout, config.WriteTimeout)
	}
	if !config.CheckConnLiveness {
		t.Fatal("connection liveness checks must be enabled")
	}
}

func TestVerifyMySQL8Compatibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		version        string
		versionComment string
		sqlMode        string
		timezone       string
		charset        string
		collation      string
		isolation      string
		storageEngine  string
		wantError      string
	}{
		{name: "production contract", version: "8.0.46", versionComment: "Source distribution", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", isolation: "REPEATABLE-READ", storageEngine: "InnoDB"},
		{name: "reject mysql 5", version: "5.7.44", versionComment: "MySQL Community Server", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", wantError: "MySQL 8 or newer"},
		{name: "reject mariadb", version: "10.11.8-MariaDB", versionComment: "MariaDB Server", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", wantError: "MySQL 8 or newer"},
		{name: "reject permissive mode", version: "8.0.46", versionComment: "Source distribution", sqlMode: "NO_ENGINE_SUBSTITUTION", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", wantError: "sql_mode"},
		{name: "reject utc session", version: "8.0.46", versionComment: "Source distribution", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_ALL_TABLES,NO_ZERO_DATE", timezone: "+00:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", wantError: "time_zone"},
		{name: "reject legacy encoding", version: "8.0.46", versionComment: "Source distribution", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8", collation: "utf8_general_ci", wantError: "utf8mb4"},
		{name: "reject read committed", version: "8.0.46", versionComment: "Source distribution", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", isolation: "READ-COMMITTED", storageEngine: "InnoDB", wantError: "REPEATABLE-READ"},
		{name: "reject myisam default", version: "8.0.46", versionComment: "Source distribution", sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_DATE", timezone: "+08:00", charset: "utf8mb4", collation: "utf8mb4_unicode_ci", isolation: "REPEATABLE-READ", storageEngine: "MyISAM", wantError: "InnoDB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isolation == "" {
				tt.isolation = "REPEATABLE-READ"
			}
			if tt.storageEngine == "" {
				tt.storageEngine = "InnoDB"
			}
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT VERSION\\(\\),@@version_comment,@@SESSION.sql_mode").WillReturnRows(
				sqlmock.NewRows([]string{"version", "version_comment", "sql_mode", "time_zone", "character_set_connection", "collation_connection", "transaction_isolation", "default_storage_engine"}).
					AddRow(tt.version, tt.versionComment, tt.sqlMode, tt.timezone, tt.charset, tt.collation, tt.isolation, tt.storageEngine),
			)
			err = verifyMySQL8(context.Background(), db)
			if tt.wantError == "" && err != nil {
				t.Fatalf("verifyMySQL8() error = %v", err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Fatalf("verifyMySQL8() error = %v, want substring %q", err, tt.wantError)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIsDuplicateUsesTypedMySQLError(t *testing.T) {
	t.Parallel()
	if !IsDuplicate(&mysql.MySQLError{Number: 1062, Message: "duplicate"}) {
		t.Fatal("MySQL 1062 must be classified as duplicate")
	}
	if IsDuplicate(errors.New("request contains text 1062 but is not a MySQL error")) {
		t.Fatal("plain error text must not be classified as duplicate")
	}
}

func TestCustomerOpaqueIdentifiersUseBinaryCollation(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/006_member_crm.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, column := range []string{"public_id VARCHAR(40)", "wechat_openid VARCHAR(128)", "guest_key VARCHAR(128)", "unionid VARCHAR(128)"} {
		if !strings.Contains(schema, column+" COLLATE utf8mb4_bin") {
			t.Fatalf("opaque identifier %q must use a case-sensitive binary collation", column)
		}
	}
	if !strings.Contains(schema, "MODIFY customer_openid VARCHAR(128) COLLATE utf8mb4_bin") {
		t.Fatal("historical order OpenID source must use binary collation before customer backfill")
	}
}
