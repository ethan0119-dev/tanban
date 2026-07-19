package database

import (
	"errors"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
)

func TestAlreadyAppliedDDL(t *testing.T) {
	t.Parallel()
	for _, code := range []uint16{1060, 1061} {
		if !isAlreadyAppliedDDL(&mysql.MySQLError{Number: code, Message: "already exists"}) {
			t.Fatalf("expected MySQL error %d to be replay-safe", code)
		}
	}
	if isAlreadyAppliedDDL(errors.New("network error")) {
		t.Fatal("unrelated errors must not be ignored")
	}
}
