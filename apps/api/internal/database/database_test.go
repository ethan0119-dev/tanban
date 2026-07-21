package database

import (
	"errors"
	"os"
	"strings"
	"testing"

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
