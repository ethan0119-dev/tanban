package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestNormalizeAnnouncementInputDefaultsAndDeduplicatesTargets(t *testing.T) {
	input := announcementInput{
		Title:        "  新功能上线  ",
		Content:      "  已支持通知收件箱。  ",
		AudienceType: "selected",
		TenantIDs:    []int64{3, 3, 0, -1, 9},
	}
	if message := normalizeAnnouncementInput(&input); message != "" {
		t.Fatalf("unexpected validation error: %s", message)
	}
	if input.Title != "新功能上线" || input.Content != "已支持通知收件箱。" {
		t.Fatalf("input was not trimmed: %#v", input)
	}
	if input.Category != "SYSTEM_UPDATE" || input.Severity != "INFO" || input.AudienceType != "SELECTED" {
		t.Fatalf("defaults were not applied: %#v", input)
	}
	if len(input.TenantIDs) != 2 || input.TenantIDs[0] != 3 || input.TenantIDs[1] != 9 {
		t.Fatalf("tenant ids were not normalized: %#v", input.TenantIDs)
	}
}

func TestNormalizeAnnouncementInputRequiresSelectedAudience(t *testing.T) {
	input := announcementInput{Title: "提示", Content: "正文", AudienceType: "SELECTED"}
	if message := normalizeAnnouncementInput(&input); message == "" {
		t.Fatal("expected selected audience validation error")
	}
}

func TestPlatformOperatorCannotCreateAnnouncement(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.platformRoutes(router)
	request := httptest.NewRequest(http.MethodPost, "/announcements", bytes.NewBufferString(`{"title":"test","content":"body"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 2, Role: RolePlatformOperator}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMarkMerchantNotificationReadIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' WHERE recipient.tenant_id=? AND a.id=?")).
		WithArgs(int64(7), int64(12)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO merchant_notification_reads(announcement_id,tenant_id,user_id) VALUES(?,?,?)")).
		WithArgs(int64(12), int64(7), int64(21)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT a.id,a.title,a.summary").
		WithArgs(int64(21), int64(7), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "summary", "content", "category", "severity", "published_at", "read_at"}).
			AddRow(12, "系统更新", "摘要", "正文", "SYSTEM_UPDATE", "INFO", "2026-07-21T10:00:00Z", "2026-07-21T10:10:00Z"))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.merchantRoutes(router)
	request := httptest.NewRequest(http.MethodPost, "/notifications/12/read", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 21, TenantID: 7, Role: RoleMerchantStaff}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
