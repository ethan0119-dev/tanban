package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
)

func TestHandleSQLErrorUsesTypedMySQLErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "duplicate", err: &mysql.MySQLError{Number: 1062, Message: "duplicate"}, wantStatus: http.StatusConflict, wantCode: "ALREADY_EXISTS"},
		{name: "deadlock", err: &mysql.MySQLError{Number: 1213, Message: "deadlock"}, wantStatus: http.StatusServiceUnavailable, wantCode: "DATABASE_BUSY"},
		{name: "lock timeout", err: &mysql.MySQLError{Number: 1205, Message: "lock timeout"}, wantStatus: http.StatusServiceUnavailable, wantCode: "DATABASE_BUSY"},
		{name: "connection lost", err: &mysql.MySQLError{Number: 2013, Message: "connection lost"}, wantStatus: http.StatusServiceUnavailable, wantCode: "DATABASE_UNAVAILABLE"},
		{name: "invalid pooled connection", err: mysql.ErrInvalidConn, wantStatus: http.StatusServiceUnavailable, wantCode: "DATABASE_UNAVAILABLE"},
		{name: "plain text is not a mysql code", err: errors.New("text contains 1062 but is not typed"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handleSQLError(recorder, tt.err)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			var response envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error == nil || response.Error.Code != tt.wantCode {
				t.Fatalf("error=%+v want code=%s", response.Error, tt.wantCode)
			}
		})
	}
}
