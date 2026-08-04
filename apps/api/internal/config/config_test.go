package config

import (
	"os"
	"path/filepath"
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

func TestLoadRejectsPartialWeChatOfficialAccountCredentials(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("TB_WECHAT_MINIAPP_APP_ID", "")
	t.Setenv("TB_WECHAT_MINIAPP_APP_SECRET", "")
	t.Setenv("TB_WECHAT_OFFICIAL_ACCOUNT_APP_ID", "wx_official_app_id")
	t.Setenv("TB_WECHAT_OFFICIAL_ACCOUNT_APP_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must be configured together") {
		t.Fatalf("expected paired official-account credential validation error, got %v", err)
	}
}

func TestLoadRejectsUnknownPaymentProvider(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("TB_PAYMENT_PROVIDER", "unknown-provider")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TB_PAYMENT_PROVIDER must be") {
		t.Fatalf("expected payment provider validation error, got %v", err)
	}
}

func TestLoadReadsWeChatPayKeysFromFiles(t *testing.T) {
	setRequiredConfig(t)
	directory := t.TempDir()
	v2Path := filepath.Join(directory, "api-v2.key")
	v3Path := filepath.Join(directory, "api-v3.key")
	if err := os.WriteFile(v2Path, []byte(strings.Repeat("2", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v3Path, []byte(strings.Repeat("3", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TB_WECHAT_PAY_API_V2_KEY", "file:"+v2Path)
	t.Setenv("TB_WECHAT_PAY_API_V3_KEY", "file:"+v3Path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WeChatPayPartner.APIV2Key != strings.Repeat("2", 32) {
		t.Fatal("APIv2 key file was not loaded")
	}
	if cfg.WeChatPayPartner.APIV3Key != strings.Repeat("3", 32) {
		t.Fatal("APIv3 key file was not loaded")
	}
}

func TestLoadRejectsMissingWeChatPayKeyFile(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("TB_WECHAT_PAY_API_V3_KEY", "file:/missing/tanban-api-v3.key")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TB_WECHAT_PAY_API_V3_KEY") {
		t.Fatalf("expected missing secret file error, got %v", err)
	}
}
