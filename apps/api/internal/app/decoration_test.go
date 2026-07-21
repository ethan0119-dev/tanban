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

func TestDecorationURLAllowsOnlyHTTPSOrLoopbackHTTP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://cdn.example.com/banner.png", true},
		{"http://127.0.0.1:18090/api/v1/public/media/uploads/t1/s1/2026/07/00112233445566778899aabbccddeeff.png", true},
		{"http://localhost:18090/banner.png", true},
		{"http://[::1]:18090/banner.png", true},
		{"http://cdn.example.com/banner.png", false},
		{"javascript:alert(1)", false},
	}
	for _, test := range tests {
		if got := validDecorationURL(test.url); got != test.want {
			t.Fatalf("validDecorationURL(%q)=%v, want %v", test.url, got, test.want)
		}
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

func TestHotspotImageAllowsEmptyDraftAndValidPercentHotspots(t *testing.T) {
	t.Parallel()
	for _, hotspots := range [][]decorationHotspot{
		{},
		{
			{ID: "menu", X: 5, Y: 10, Width: 40, Height: 30, Label: "开始点单", Action: DecorationAction{Type: "OPEN_MENU"}},
			{ID: "phone", X: 50, Y: 60, Width: 45, Height: 35, Label: "联系门店", Action: DecorationAction{Type: "CALL_PHONE", Phone: "186-0000-0000"}},
		},
	} {
		config := defaultDecorationConfig(storeDTO{})
		config.Home.Modules = append(config.Home.Modules, decorationModule("hotspot", "HOTSPOT_IMAGE", 50, decorationHotspotImageConfig{
			ImageURL: "https://cdn.example.com/home.png",
			Alt:      "首页活动图",
			Hotspots: hotspots,
		}))
		if err := validateDecorationConfig(config); err != nil {
			t.Fatalf("valid hotspot image rejected: %v", err)
		}
	}
}

func TestManagedDecorationStorageKeysCollectsUploadedImages(t *testing.T) {
	t.Parallel()
	baseURL := "https://tbapi.example.com/api/v1/public/media"
	first := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	second := "uploads/t5/s9/2026/07/ffeeddccbbaa99887766554433221100.jpg"
	config := defaultDecorationConfig(storeDTO{})
	config.Home.Modules = append(config.Home.Modules,
		decorationModule("local", "HOTSPOT_IMAGE", 50, decorationHotspotImageConfig{ImageURL: baseURL + "/" + first, Alt: "本地素材"}),
		decorationModule("external", "IMAGE", 60, decorationImageConfig{ImageURL: "https://cdn.example.com/banner.png", Alt: "外部素材", Action: DecorationAction{Type: "NONE"}}),
	)
	config.Splash.ImageURL = baseURL + "/" + second
	keys, err := managedDecorationStorageKeys(config, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != first || keys[1] != second {
		t.Fatalf("unexpected managed keys: %#v", keys)
	}
}

func TestHotspotImageRejectsInvalidRegionsAndActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		hotspots []decorationHotspot
		want     string
	}{
		{
			name: "outside image",
			hotspots: []decorationHotspot{
				{ID: "outside", X: 80, Y: 0, Width: 21, Height: 20, Label: "越界", Action: DecorationAction{Type: "NONE"}},
			},
			want: "0-100 percent",
		},
		{
			name: "zero width",
			hotspots: []decorationHotspot{
				{ID: "zero", X: 10, Y: 10, Width: 0, Height: 20, Label: "无宽度", Action: DecorationAction{Type: "NONE"}},
			},
			want: "0-100 percent",
		},
		{
			name: "duplicated id",
			hotspots: []decorationHotspot{
				{ID: "same", X: 0, Y: 0, Width: 10, Height: 10, Label: "一", Action: DecorationAction{Type: "NONE"}},
				{ID: "same", X: 10, Y: 10, Width: 10, Height: 10, Label: "二", Action: DecorationAction{Type: "NONE"}},
			},
			want: "duplicated",
		},
		{
			name: "unsupported action",
			hotspots: []decorationHotspot{
				{ID: "route", X: 0, Y: 0, Width: 10, Height: 10, Label: "任意路由", Action: DecorationAction{Type: "OPEN_PATH"}},
			},
			want: "unsupported action",
		},
	}
	tooMany := make([]decorationHotspot, decorationMaxHotspots+1)
	for index := range tooMany {
		tooMany[index] = decorationHotspot{ID: "spot-" + int64String(int64(index)), X: 0, Y: 0, Width: 1, Height: 1, Label: "热区", Action: DecorationAction{Type: "NONE"}}
	}
	tests = append(tests, struct {
		name     string
		hotspots []decorationHotspot
		want     string
	}{name: "too many", hotspots: tooMany, want: "more than 20"})

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			module := decorationModule("hotspot", "HOTSPOT_IMAGE", 50, decorationHotspotImageConfig{
				ImageURL: "https://cdn.example.com/home.png",
				Alt:      "首页活动图",
				Hotspots: test.hotspots,
			})
			err := validateDecorationModule(module)
			if err == nil || !regexp.MustCompile(test.want).MatchString(err.Error()) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
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
		{Name: "图片", URL: "https://cdn.example.com/a.jpg", StorageKey: "uploads/t1/s2/2026/07/00112233445566778899aabbccddeeff.jpg"},
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

func TestPublicDecorationNormalizesOlderPublishedSnapshots(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	store := storeDTO{ID: 8, TenantID: 3}
	published := defaultDecorationConfig(storeDTO{})
	published.TemplateKey = "warm-bakery"
	published.Theme.PrimaryColor = "#9A5F3D"
	published.Theme.FontScale = ""
	published.Theme.SurfaceStyle = ""
	published.Theme.ButtonShape = ""
	published.Navigation.TemplateKey = ""
	body, _ := json.Marshal(published)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT v.version_no,v.config_json FROM store_decorations d JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`)).
		WithArgs(int64(3), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"version_no", "config_json"}).AddRow(2, string(body)))
	got, version := server.publicDecorationConfig(context.Background(), store)
	if version != 2 || got.TemplateKey != "warm-bakery" || got.Theme.PrimaryColor != "#9A5F3D" {
		t.Fatalf("published theme was not preserved: version=%d config=%#v", version, got)
	}
	if got.Navigation.TemplateKey != "classic" || got.Theme.FontScale != "STANDARD" || got.Theme.SurfaceStyle != "ELEVATED" || got.Theme.ButtonShape != "ROUNDED" {
		t.Fatalf("published snapshot was not normalized: %#v", got)
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
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(9), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
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
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(9), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
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
