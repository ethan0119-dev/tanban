package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 3))
	canvas.Set(0, 0, color.RGBA{R: 32, G: 96, B: 160, A: 255})
	var body bytes.Buffer
	if err := png.Encode(&body, canvas); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestInspectUploadedImageUsesContentAndDimensions(t *testing.T) {
	t.Parallel()
	imageFile, err := inspectUploadedImage(bytes.NewReader(testPNG(t)))
	if err != nil {
		t.Fatalf("valid PNG rejected: %v", err)
	}
	if imageFile.MimeType != "image/png" || imageFile.Extension != ".png" || imageFile.Width != 2 || imageFile.Height != 3 {
		t.Fatalf("unexpected image metadata: %#v", imageFile)
	}
	if _, err = inspectUploadedImage(bytes.NewBufferString("<svg onload=alert(1)></svg>")); !errors.Is(err, errUnsupportedMediaType) {
		t.Fatalf("non-raster content must be rejected, got %v", err)
	}
	if _, err = inspectUploadedImage(bytes.NewReader(make([]byte, mediaMaxUploadBytes+1))); !errors.Is(err, errMediaUploadTooLarge) {
		t.Fatalf("oversized content must be rejected, got %v", err)
	}
}

func TestLocalMediaStorageKeyAndPathAreTraversalSafe(t *testing.T) {
	t.Parallel()
	key := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	tenantID, storeID, ok := parseLocalMediaStorageKey(key)
	if !ok || tenantID != 5 || storeID != 9 {
		t.Fatalf("valid key rejected: tenant=%d store=%d ok=%v", tenantID, storeID, ok)
	}
	root := t.TempDir()
	target, err := localMediaPath(root, key)
	if err != nil {
		t.Fatal(err)
	}
	if relative, err := filepath.Rel(root, target); err != nil || relative == ".." || filepath.IsAbs(relative) {
		t.Fatalf("target escaped root: target=%q relative=%q err=%v", target, relative, err)
	}
	for _, unsafe := range []string{
		"../uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png",
		"uploads/t5/s9/2026/07/../../00112233445566778899aabbccddeeff.png",
		"uploads/t5/s9/2026/07/not-random.png",
		"uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.svg",
	} {
		if _, _, ok := parseLocalMediaStorageKey(unsafe); ok {
			t.Fatalf("unsafe key accepted: %q", unsafe)
		}
		if _, err := localMediaPath(root, unsafe); err == nil {
			t.Fatalf("unsafe path accepted: %q", unsafe)
		}
	}
}

func TestUploadMediaAssetPersistsAuthenticatedMerchantImage(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storageRoot := t.TempDir()
	server := New(db, config.Config{
		MediaStorageDir:    storageRoot,
		MediaPublicBaseURL: "https://tbapi.example.com/api/v1/public/media",
	}, slog.Default())
	imageBody := testPNG(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM stores WHERE tenant_id=? AND deleted_at IS NULL ORDER BY id LIMIT 1")).
		WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\),COALESCE\\(SUM\\(size_bytes\\),0\\) FROM media_assets").WithArgs(int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"count", "bytes"}).AddRow(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO media_assets(tenant_id,store_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`)).
		WithArgs(int64(5), int64(9), "首页主图", sqlmock.AnyArg(), sqlmock.AnyArg(), "image/png", 2, 3, len(imageBody), int64(12)).
		WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	file, err := writer.CreateFormFile("file", "../../evil-name.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(imageBody); err != nil {
		t.Fatal(err)
	}
	if err = writer.WriteField("name", "首页主图"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/merchant/media-assets/upload", &multipartBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	server.uploadMediaAsset(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data mediaAssetView `json:"data"`
	}
	if err = json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Data.ID != 44 || response.Data.Name != "首页主图" || response.Data.MimeType != "image/png" || response.Data.Width != 2 || response.Data.Height != 3 {
		t.Fatalf("unexpected response: %#v", response.Data)
	}
	if response.Data.URL != "https://tbapi.example.com/api/v1/public/media/"+response.Data.StorageKey {
		t.Fatalf("unexpected public URL: %q", response.Data.URL)
	}
	if filepath.Base(response.Data.StorageKey) == "evil-name.png" {
		t.Fatalf("client filename was used as storage filename: %q", response.Data.StorageKey)
	}
	target, err := localMediaPath(storageRoot, response.Data.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(persisted, imageBody) {
		t.Fatal("persisted bytes do not match upload")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadMediaAssetCleansFileWhenDatabaseWriteFails(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storageRoot := t.TempDir()
	server := New(db, config.Config{MediaStorageDir: storageRoot, MediaPublicBaseURL: "https://tbapi.example.com/api/v1/public/media"}, slog.Default())
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\),COALESCE\\(SUM\\(size_bytes\\),0\\) FROM media_assets").WithArgs(int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"count", "bytes"}).AddRow(0, 0))
	mock.ExpectExec("INSERT INTO media_assets").WillReturnError(errors.New("database unavailable"))
	mock.ExpectRollback()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write(testPNG(t))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/merchant/media-assets/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	server.uploadMediaAsset(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	err = filepath.WalkDir(storageRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			t.Fatalf("orphan file remains after database failure: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestServeMediaAssetOnlyReturnsActiveRegisteredImage(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storageRoot := t.TempDir()
	server := New(db, config.Config{MediaStorageDir: storageRoot}, slog.Default())
	key := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	imageBody := testPNG(t)
	target, err := persistUploadedImage(storageRoot, key, imageBody)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(target, time.Unix(1_700_000_000, 0), time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT mime_type FROM media_assets WHERE tenant_id=? AND store_id=? AND storage_key=? AND kind IN ('IMAGE','TENANT_DOCUMENT') AND status='ACTIVE' AND deleted_at IS NULL LIMIT 1`)).
		WithArgs(int64(5), int64(9), key).
		WillReturnRows(sqlmock.NewRows([]string{"mime_type"}).AddRow("image/png"))

	router := chi.NewRouter()
	router.Get("/media/*", server.serveMediaAsset)
	request := httptest.NewRequest(http.MethodGet, "/media/"+key, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), imageBody) {
		t.Fatalf("unexpected public media response %d: %q", recorder.Code, recorder.Body.Bytes())
	}
	if recorder.Header().Get("Content-Type") != "image/png" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("missing safe immutable headers: %#v", recorder.Header())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestServeMediaAssetHidesDeletedOrUnknownRecords(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{MediaStorageDir: t.TempDir()}, slog.Default())
	key := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	mock.ExpectQuery("SELECT mime_type FROM media_assets").WithArgs(int64(5), int64(9), key).WillReturnError(sql.ErrNoRows)
	router := chi.NewRouter()
	router.Get("/media/*", server.serveMediaAsset)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/media/"+key, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUploadedMediaAssetRenamesWithoutLosingManagedMetadata(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	key := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	assetURL := "https://tbapi.example.com/api/v1/public/media/" + key
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM media_assets WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`)).
		WithArgs(int64(44), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "kind", "url", "storage_key", "mime_type", "width", "height", "size_bytes", "status", "created_at"}).
			AddRow(44, "旧名称", "IMAGE", assetURL, key, "image/png", 800, 600, 12345, "ACTIVE", "2026-07-20T01:02:03Z"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE media_assets SET name=? WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`)).
		WithArgs("新名称", int64(44), int64(5), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	request := httptest.NewRequest(http.MethodPut, "/media-assets/44", bytes.NewBufferString(`{"name":"新名称","url":"","storageKey":"","mimeType":"","width":0,"height":0,"sizeBytes":0}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/media-assets/{assetID}", server.updateMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data mediaAssetView `json:"data"`
	}
	if err = json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Name != "新名称" || response.Data.URL != assetURL || response.Data.StorageKey != key || response.Data.MimeType != "image/png" || response.Data.Width != 800 || response.Data.Height != 600 || response.Data.SizeBytes != 12345 {
		t.Fatalf("server-managed metadata was not preserved: %#v", response.Data)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUploadedMediaAssetRejectsManagedMetadataReplacement(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	key := "uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	assetURL := "https://tbapi.example.com/api/v1/public/media/" + key
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT id,name,kind,url,storage_key").WithArgs(int64(44), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "kind", "url", "storage_key", "mime_type", "width", "height", "size_bytes", "status", "created_at"}).
			AddRow(44, "旧名称", "IMAGE", assetURL, key, "image/png", 800, 600, 12345, "ACTIVE", "2026-07-20T01:02:03Z"))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodPut, "/media-assets/44", bytes.NewBufferString(`{"name":"新名称","url":"https://evil.example.com/replacement.png"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Put("/media-assets/{assetID}", server.updateMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !bytes.Contains(recorder.Body.Bytes(), []byte("IMMUTABLE_MEDIA_METADATA")) {
		t.Fatalf("expected immutable metadata rejection, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMediaAssetRejectsDecorationReference(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	assetURL := "https://tbapi.example.com/api/v1/public/media/uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	draft := `{"home":{"modules":[{"type":"HOTSPOT_IMAGE","config":{"imageUrl":"` + assetURL + `","hotspots":[]}}]}}`
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT url,storage_key FROM media_assets WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`)).
		WithArgs(int64(44), int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"url", "storage_key"}).AddRow(assetURL, ""))
	mock.ExpectQuery("SELECT d.draft_json,COALESCE").WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"draft_json", "published_json"}).AddRow(draft, ""))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodDelete, "/media-assets/44", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Delete("/media-assets/{assetID}", server.deleteMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte("MEDIA_ASSET_IN_USE")) {
		t.Fatalf("expected in-use conflict, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMediaAssetRejectsHistoricalDecorationVersionReference(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	assetURL := "https://tbapi.example.com/api/v1/public/media/uploads/t5/s9/2026/07/00112233445566778899aabbccddeeff.png"
	historical := `{"home":{"modules":[{"type":"IMAGE","config":{"imageUrl":"` + assetURL + `"}}]}}`
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT url,storage_key FROM media_assets").WithArgs(int64(44), int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"url", "storage_key"}).AddRow(assetURL, ""))
	mock.ExpectQuery("SELECT d.draft_json,COALESCE").WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"draft_json", "published_json"}).AddRow(`{"home":{"modules":[]}}`, `{"home":{"modules":[]}}`))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT config_json FROM store_decoration_versions WHERE tenant_id=? AND store_id=? ORDER BY id`)).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"config_json"}).AddRow(`{"home":{"modules":[]}}`).AddRow(historical))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodDelete, "/media-assets/44", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Delete("/media-assets/{assetID}", server.deleteMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte("MEDIA_ASSET_IN_USE")) {
		t.Fatalf("expected historical in-use conflict, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUnreferencedMediaAssetStillSucceeds(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	assetURL := "https://cdn.example.com/not-used.png"
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT url,storage_key FROM media_assets").WithArgs(int64(44), int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"url", "storage_key"}).AddRow(assetURL, ""))
	mock.ExpectQuery("SELECT d.draft_json,COALESCE").WithArgs(int64(5), int64(9)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT config_json FROM store_decoration_versions").WithArgs(int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"config_json"}))
	mock.ExpectQuery("SELECT CASE").WithArgs(
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL,
	).WillReturnRows(sqlmock.NewRows([]string{"reference_kind"}).AddRow(""))
	mock.ExpectExec("UPDATE media_assets SET status='DELETED'").WithArgs(int64(44), int64(5), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	request := httptest.NewRequest(http.MethodDelete, "/media-assets/44", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Delete("/media-assets/{assetID}", server.deleteMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteMediaAssetRejectsActiveProductImageReference(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	assetURL := "https://cdn.example.com/product.png"
	mock.ExpectQuery("SELECT id FROM stores").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM stores WHERE id=").WithArgs(int64(9), int64(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT url,storage_key FROM media_assets").WithArgs(int64(44), int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"url", "storage_key"}).AddRow(assetURL, ""))
	mock.ExpectQuery("SELECT d.draft_json,COALESCE").WithArgs(int64(5), int64(9)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT config_json FROM store_decoration_versions").WithArgs(int64(5), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"config_json"}))
	mock.ExpectQuery("SELECT CASE").WithArgs(
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL,
	).WillReturnRows(sqlmock.NewRows([]string{"reference_kind"}).AddRow("product images"))
	mock.ExpectRollback()

	request := httptest.NewRequest(http.MethodDelete, "/media-assets/44", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 12, TenantID: 5, Role: RoleMerchantManager}))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Delete("/media-assets/{assetID}", server.deleteMediaAsset)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte("MEDIA_ASSET_IN_USE")) {
		t.Fatalf("expected product in-use conflict, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentMediaAssetReferenceIncludesCustomerServiceQRCode(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assetURL := "https://cdn.example.com/customer-service.png"
	mock.ExpectQuery("SELECT CASE").WithArgs(
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), int64(44), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), int64(9), assetURL, assetURL,
		int64(5), assetURL,
		int64(5), int64(9), assetURL,
		int64(5), int64(9), assetURL,
	).WillReturnRows(sqlmock.NewRows([]string{"reference_kind"}).AddRow("customer service QR code"))

	referenceKind, err := currentMediaAssetReference(context.Background(), db, 5, 9, 44, assetURL)
	if err != nil {
		t.Fatal(err)
	}
	if referenceKind != "customer service QR code" {
		t.Fatalf("unexpected reference kind %q", referenceKind)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
