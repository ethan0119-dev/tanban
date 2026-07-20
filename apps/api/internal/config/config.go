package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type TianQue struct {
	BaseURL    string
	OrgID      string
	PrivateKey string
	PublicKey  string
	NotifyURL  string
}

type WeChatMiniApp struct {
	AppID     string
	AppSecret string
}

type Config struct {
	HTTPAddr              string
	DatabaseDSN           string
	JWTSecret             string
	JWTTTL                time.Duration
	MigrationsDir         string
	BootstrapAdminUser    string
	BootstrapAdminPass    string
	PaymentProvider       string
	PrinterProvider       string
	MediaStorageDir       string
	MediaPublicBaseURL    string
	RedisAddr             string
	AutoMigrate           bool
	AllowMockConfirmation bool
	CORSAllowedOrigins    []string
	SeedDemo              bool
	DemoMerchantUser      string
	DemoMerchantPass      string
	TianQue               TianQue
	WeChatMiniApp         WeChatMiniApp
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:              env("TB_HTTP_ADDR", ":18090"),
		DatabaseDSN:           os.Getenv("TB_DATABASE_DSN"),
		JWTSecret:             os.Getenv("TB_JWT_SECRET"),
		MigrationsDir:         env("TB_MIGRATIONS_DIR", "./migrations"),
		BootstrapAdminUser:    os.Getenv("TB_BOOTSTRAP_ADMIN_USERNAME"),
		BootstrapAdminPass:    os.Getenv("TB_BOOTSTRAP_ADMIN_PASSWORD"),
		PaymentProvider:       strings.ToLower(env("TB_PAYMENT_PROVIDER", "mock")),
		PrinterProvider:       strings.ToLower(env("TB_PRINTER_PROVIDER", "mock")),
		MediaStorageDir:       env("TB_MEDIA_STORAGE_DIR", "./data/media"),
		MediaPublicBaseURL:    strings.TrimRight(env("TB_MEDIA_PUBLIC_BASE_URL", "http://127.0.0.1:18090/api/v1/public/media"), "/"),
		RedisAddr:             os.Getenv("TB_REDIS_ADDR"),
		AutoMigrate:           envBool("TB_AUTO_MIGRATE", true),
		AllowMockConfirmation: envBool("TB_ALLOW_MOCK_CONFIRMATION", false),
		CORSAllowedOrigins:    splitCSV(env("TB_CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://127.0.0.1:5173,http://127.0.0.1:5174")),
		SeedDemo:              envBool("TB_SEED_DEMO", false),
		DemoMerchantUser:      os.Getenv("TB_DEMO_MERCHANT_USERNAME"),
		DemoMerchantPass:      os.Getenv("TB_DEMO_MERCHANT_PASSWORD"),
		TianQue: TianQue{
			BaseURL:    os.Getenv("TB_TIANQUE_BASE_URL"),
			OrgID:      os.Getenv("TB_TIANQUE_ORG_ID"),
			PrivateKey: os.Getenv("TB_TIANQUE_PRIVATE_KEY"),
			PublicKey:  os.Getenv("TB_TIANQUE_PUBLIC_KEY"),
			NotifyURL:  os.Getenv("TB_TIANQUE_NOTIFY_URL"),
		},
		WeChatMiniApp: WeChatMiniApp{
			AppID:     strings.TrimSpace(os.Getenv("TB_WECHAT_MINIAPP_APP_ID")),
			AppSecret: strings.TrimSpace(os.Getenv("TB_WECHAT_MINIAPP_APP_SECRET")),
		},
	}
	if cfg.DatabaseDSN == "" {
		return Config{}, fmt.Errorf("TB_DATABASE_DSN is required")
	}
	if cfg.JWTSecret == "" || len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("TB_JWT_SECRET must contain at least 32 characters")
	}
	if (cfg.WeChatMiniApp.AppID == "") != (cfg.WeChatMiniApp.AppSecret == "") {
		return Config{}, fmt.Errorf("TB_WECHAT_MINIAPP_APP_ID and TB_WECHAT_MINIAPP_APP_SECRET must be configured together")
	}
	var err error
	cfg.JWTTTL, err = time.ParseDuration(env("TB_JWT_TTL", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse TB_JWT_TTL: %w", err)
	}
	return cfg, nil
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
