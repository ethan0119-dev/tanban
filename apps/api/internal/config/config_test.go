package config

import (
	"strings"
	"testing"
)

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("TB_DATABASE_DSN", "tanban:test@tcp(127.0.0.1:3306)/tanban")
	t.Setenv("TB_JWT_SECRET", strings.Repeat("x", 32))
}

func TestLoadWeChatMiniAppCredentials(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("TB_WECHAT_MINIAPP_APP_ID", "wx_test_app_id")
	t.Setenv("TB_WECHAT_MINIAPP_APP_SECRET", "test_app_secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WeChatMiniApp.AppID != "wx_test_app_id" {
		t.Fatalf("unexpected app id %q", cfg.WeChatMiniApp.AppID)
	}
	if cfg.WeChatMiniApp.AppSecret != "test_app_secret" {
		t.Fatal("wechat app secret was not loaded")
	}
}

func TestLoadRejectsPartialWeChatMiniAppCredentials(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("TB_WECHAT_MINIAPP_APP_ID", "wx_test_app_id")
	t.Setenv("TB_WECHAT_MINIAPP_APP_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must be configured together") {
		t.Fatalf("expected paired credential validation error, got %v", err)
	}
}
