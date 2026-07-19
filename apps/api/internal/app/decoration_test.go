package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestDefaultDecorationConfigIsValid(t *testing.T) {
	t.Parallel()
	config := defaultDecorationConfig(storeDTO{BannerURL: "https://cdn.example.com/banner.jpg"})
	if err := validateDecorationConfig(config); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if len(config.Home.Modules) != 4 {
		t.Fatalf("expected 4 default modules, got %d", len(config.Home.Modules))
	}
	var hero decorationHeroConfig
	if err := json.Unmarshal(config.Home.Modules[0].Config, &hero); err != nil {
		t.Fatal(err)
	}
	if len(hero.Items) != 1 || hero.Items[0].ImageURL != "https://cdn.example.com/banner.jpg" {
		t.Fatalf("legacy banner was not used: %#v", hero.Items)
	}
}

func TestDefaultDecorationRejectsInsecureLegacyBanner(t *testing.T) {
	t.Parallel()
	config := defaultDecorationConfig(storeDTO{BannerURL: "http://cdn.example.com/banner.jpg"})
	var hero decorationHeroConfig
	if err := json.Unmarshal(config.Home.Modules[0].Config, &hero); err != nil {
		t.Fatal(err)
	}
	if len(hero.Items) != 0 {
		t.Fatalf("insecure legacy banner must not be exposed: %#v", hero.Items)
	}
}

func TestNormalizeDecorationFillsSafeDefaults(t *testing.T) {
	t.Parallel()
	var config DecorationConfig
	normalizeDecorationConfig(&config)
	if config.SchemaVersion != decorationSchemaVersion || config.Theme.PrimaryColor == "" {
		t.Fatalf("defaults were not filled: %#v", config)
	}
	if len(config.Navigation.Items) != 4 {
		t.Fatalf("expected default navigation, got %d", len(config.Navigation.Items))
	}
	if err := validateDecorationConfig(config); err != nil {
		t.Fatalf("normalized config should validate: %v", err)
	}
}

func TestNormalizeDecorationPreservesManualSplashClose(t *testing.T) {
	t.Parallel()
	config := defaultDecorationConfig(storeDTO{})
	config.Splash.AutoCloseSeconds = 0
	normalizeDecorationConfig(&config)
	if config.Splash.AutoCloseSeconds != 0 {
		t.Fatalf("zero auto-close must remain a manual-close setting, got %d", config.Splash.AutoCloseSeconds)
	}
}

func TestDecorationValidationRejectsUnsafeCapabilities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*DecorationConfig)
		want   string
	}{
		{
			name: "arbitrary route action",
			mutate: func(config *DecorationConfig) {
				config.Splash.Action = DecorationAction{Type: "OPEN_PATH"}
			},
			want: "unsupported action type",
		},
		{
			name: "javascript image URL",
			mutate: func(config *DecorationConfig) {
				config.Home.Modules = append(config.Home.Modules, decorationModule("unsafe-image", "IMAGE", 50, decorationImageConfig{ImageURL: "javascript:alert(1)", Action: DecorationAction{Type: "NONE"}}))
			},
			want: "HTTPS",
		},
		{
			name: "unknown module",
			mutate: func(config *DecorationConfig) {
				config.Home.Modules = append(config.Home.Modules, decorationModule("custom-code", "CUSTOM_HTML", 50, map[string]string{"html": "<script>"}))
			},
			want: "unsupported module",
		},
		{
			name: "duplicated navigation",
			mutate: func(config *DecorationConfig) {
				config.Navigation.Items[1].Key = "home"
			},
			want: "duplicated",
		},
		{
			name: "invalid color",
			mutate: func(config *DecorationConfig) {
				config.Theme.PrimaryColor = "red"
			},
			want: "#RRGGBB",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := defaultDecorationConfig(storeDTO{})
			test.mutate(&config)
			err := validateDecorationConfig(config)
			if err == nil || !regexp.MustCompile(test.want).MatchString(err.Error()) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestDecorationModuleRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	module := decorationModule("text", "TEXT", 10, map[string]any{"title": "标题", "body": "正文", "align": "LEFT", "script": "alert(1)"})
	if err := validateDecorationModule(module); err == nil {
		t.Fatal("unknown module config fields must be rejected")
	}
}

func TestMediaAssetValidation(t *testing.T) {
	t.Parallel()
	if err := validateMediaAssetInput(mediaAssetInput{Name: "首页图", URL: "https://cdn.example.com/a.jpg", MimeType: "image/jpeg", Width: 800, Height: 600}); err != nil {
		t.Fatalf("valid asset rejected: %v", err)
	}
	for _, input := range []mediaAssetInput{
		{Name: "", URL: "https://cdn.example.com/a.jpg"},
		{Name: "图片", URL: "http://cdn.example.com/a.jpg"},
		{Name: "图片", URL: "https://cdn.example.com/a.jpg", MimeType: "text/html"},
	} {
		if err := validateMediaAssetInput(input); err == nil {
			t.Fatalf("invalid asset accepted: %#v", input)
		}
	}
}

func TestPublicDecorationUsesLegacyFallbackAndNeverDraft(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	store := storeDTO{ID: 8, TenantID: 3, BannerURL: "https://cdn.example.com/legacy.jpg"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT v.version_no,v.config_json FROM store_decorations d JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`)).
		WithArgs(int64(3), int64(8)).
		WillReturnError(sql.ErrNoRows)
	got, version := server.publicDecorationConfig(context.Background(), store)
	if version != 0 {
		t.Fatalf("fallback must not claim a published version: %d", version)
	}
	var hero decorationHeroConfig
	if err := json.Unmarshal(got.Home.Modules[0].Config, &hero); err != nil {
		t.Fatal(err)
	}
	if len(hero.Items) != 1 || hero.Items[0].ImageURL != store.BannerURL {
		t.Fatalf("legacy fallback not returned: %#v", hero.Items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishDecorationVersionCreatesImmutableSnapshot(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	config := defaultDecorationConfig(storeDTO{})
	body, _ := json.Marshal(config)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT draft_json,draft_revision FROM store_decorations WHERE tenant_id=? AND store_id=? FOR UPDATE`)).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"draft_json", "draft_revision"}).AddRow(string(body), int64(4)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(version_no),0)+1 FROM store_decoration_versions WHERE tenant_id=? AND store_id=?`)).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"version_no"}).AddRow(3))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO store_decoration_versions(tenant_id,store_id,version_no,schema_version,config_json,publish_note,source_version_id,published_by) VALUES(?,?,?,?,?,?,?,?)`)).
		WithArgs(int64(5), int64(9), 3, 1, sqlmock.AnyArg(), "上线首页", nil, int64(12)).
		WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE store_decorations SET published_version_id=?,updated_by=? WHERE tenant_id=? AND store_id=? AND draft_revision=?`)).
		WithArgs(int64(44), int64(12), int64(5), int64(9), int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	published, err := server.publishDecorationVersion(context.Background(), identity{TenantID: 5, UserID: 12}, 9, 4, "上线首页", 0, "")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if published.ID != 44 || published.VersionNo != 3 {
		t.Fatalf("unexpected published view: %#v", published)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishDecorationVersionRejectsStaleRevision(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	body, _ := json.Marshal(defaultDecorationConfig(storeDTO{}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT draft_json,draft_revision FROM store_decorations WHERE tenant_id=? AND store_id=? FOR UPDATE`)).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"draft_json", "draft_revision"}).AddRow(string(body), int64(5)))
	mock.ExpectRollback()
	_, err = server.publishDecorationVersion(context.Background(), identity{TenantID: 5, UserID: 12}, 9, 4, "stale", 0, "")
	if !errors.Is(err, errDecorationConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
