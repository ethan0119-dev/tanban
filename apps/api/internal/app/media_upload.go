package app

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	mediaMaxUploadBytes    = 8 * 1024 * 1024
	mediaMaxMultipartBytes = mediaMaxUploadBytes + 1024*1024
	mediaMaxDimension      = 12000
	mediaMaxPixels         = 80 * 1024 * 1024
	mediaMaxAssetsPerStore = 1000
	mediaMaxBytesPerStore  = int64(1024 * 1024 * 1024)
)

var (
	errMediaUploadTooLarge  = errors.New("image exceeds the upload size limit")
	errUnsupportedMediaType = errors.New("only valid JPEG, PNG and GIF images are supported")
)

type uploadedImage struct {
	Data      []byte
	MimeType  string
	Extension string
	Width     int
	Height    int
}

var uploadedImageTypes = map[string]struct {
	Format    string
	Extension string
}{
	"image/jpeg": {Format: "jpeg", Extension: ".jpg"},
	"image/png":  {Format: "png", Extension: ".png"},
	"image/gif":  {Format: "gif", Extension: ".gif"},
}

func inspectUploadedImage(reader io.Reader) (uploadedImage, error) {
	body, err := io.ReadAll(io.LimitReader(reader, mediaMaxUploadBytes+1))
	if err != nil {
		return uploadedImage{}, err
	}
	if len(body) == 0 {
		return uploadedImage{}, errors.New("image file is empty")
	}
	if len(body) > mediaMaxUploadBytes {
		return uploadedImage{}, errMediaUploadTooLarge
	}
	mimeType := http.DetectContentType(body)
	allowed, ok := uploadedImageTypes[mimeType]
	if !ok {
		return uploadedImage{}, errUnsupportedMediaType
	}
	dimensions, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || format != allowed.Format {
		return uploadedImage{}, errUnsupportedMediaType
	}
	if dimensions.Width < 1 || dimensions.Height < 1 || dimensions.Width > mediaMaxDimension || dimensions.Height > mediaMaxDimension ||
		int64(dimensions.Width)*int64(dimensions.Height) > mediaMaxPixels {
		return uploadedImage{}, fmt.Errorf("image dimensions exceed %dx%d or %d pixels", mediaMaxDimension, mediaMaxDimension, mediaMaxPixels)
	}
	return uploadedImage{Data: body, MimeType: mimeType, Extension: allowed.Extension, Width: dimensions.Width, Height: dimensions.Height}, nil
}

func newMediaStorageKey(tenantID, storeID int64, extension string, now time.Time) (string, error) {
	if tenantID <= 0 || storeID <= 0 {
		return "", errors.New("tenant and store are required")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("uploads/t%d/s%d/%s/%s%s", tenantID, storeID, now.UTC().Format("2006/01"), hex.EncodeToString(random), extension), nil
}

func parseLocalMediaStorageKey(value string) (tenantID, storeID int64, ok bool) {
	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "\\\x00") {
		return 0, 0, false
	}
	parts := strings.Split(value, "/")
	if len(parts) != 6 || parts[0] != "uploads" || len(parts[1]) < 2 || parts[1][0] != 't' || len(parts[2]) < 2 || parts[2][0] != 's' {
		return 0, 0, false
	}
	tenantID, tenantErr := strconv.ParseInt(parts[1][1:], 10, 64)
	storeID, storeErr := strconv.ParseInt(parts[2][1:], 10, 64)
	if tenantErr != nil || storeErr != nil || tenantID <= 0 || storeID <= 0 || len(parts[3]) != 4 || len(parts[4]) != 2 {
		return 0, 0, false
	}
	if _, err := time.Parse("2006/01", parts[3]+"/"+parts[4]); err != nil {
		return 0, 0, false
	}
	extension := strings.ToLower(filepath.Ext(parts[5]))
	name := strings.TrimSuffix(parts[5], extension)
	if len(name) != 32 || (extension != ".jpg" && extension != ".png" && extension != ".gif") {
		return 0, 0, false
	}
	if _, err := hex.DecodeString(name); err != nil {
		return 0, 0, false
	}
	return tenantID, storeID, true
}

func isLocalMediaStorageKey(value string) bool {
	_, _, ok := parseLocalMediaStorageKey(value)
	return ok
}

func localMediaPath(root, storageKey string) (string, error) {
	if !isLocalMediaStorageKey(storageKey) {
		return "", errors.New("invalid media storage key")
	}
	rootPath, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil || strings.TrimSpace(root) == "" {
		return "", errors.New("media storage directory is not configured")
	}
	target := filepath.Join(rootPath, filepath.FromSlash(storageKey))
	relative, err := filepath.Rel(rootPath, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("media path escapes the storage directory")
	}
	return target, nil
}

func mediaPublicURL(baseURL, storageKey string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("media public base URL must be an absolute HTTP(S) URL without query, fragment or credentials")
	}
	if !isLocalMediaStorageKey(storageKey) {
		return "", errors.New("invalid media storage key")
	}
	return baseURL + "/" + storageKey, nil
}

func persistUploadedImage(root, storageKey string, body []byte) (string, error) {
	target, err := localMediaPath(root, storageKey)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return "", err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(target)
		}
	}()
	if _, err = file.Write(body); err != nil {
		return "", err
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	complete = true
	return target, nil
}

func (s *Server) uploadMediaAsset(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, mediaMaxMultipartBytes)
	if err = r.ParseMultipartForm(mediaMaxUploadBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", fmt.Sprintf("image cannot exceed %d MiB", mediaMaxUploadBytes/(1024*1024)))
		} else {
			writeError(w, http.StatusBadRequest, "INVALID_MULTIPART", "a multipart form with file and optional name fields is required")
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "FILE_REQUIRED", "multipart field file is required")
		return
	}
	defer file.Close()
	if header.Size > mediaMaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", fmt.Sprintf("image cannot exceed %d MiB", mediaMaxUploadBytes/(1024*1024)))
		return
	}
	imageFile, err := inspectUploadedImage(file)
	if errors.Is(err, errMediaUploadTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", fmt.Sprintf("image cannot exceed %d MiB", mediaMaxUploadBytes/(1024*1024)))
		return
	}
	if err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_IMAGE", err.Error())
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strings.TrimSpace(filepath.Base(header.Filename))
	}
	if name == "" {
		name = "装修图片"
	}
	if !validRequiredText(name, 120) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required and limited to 120 characters")
		return
	}
	var groupID int64
	if rawGroupID := strings.TrimSpace(r.FormValue("group_id")); rawGroupID != "" {
		groupID, err = strconv.ParseInt(rawGroupID, 10, 64)
		if err != nil || groupID < 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "group_id must be a non-negative integer")
			return
		}
	}
	storageKey, err := newMediaStorageKey(identity.TenantID, storeID, imageFile.Extension, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to allocate image storage")
		return
	}
	publicURL, err := mediaPublicURL(s.Config.MediaPublicBaseURL, storageKey)
	if err != nil {
		s.Logger.Error("build media public url", "error", err)
		writeError(w, http.StatusServiceUnavailable, "MEDIA_STORAGE_NOT_CONFIGURED", "media public URL is not configured")
		return
	}
	storedPath, err := persistUploadedImage(s.Config.MediaStorageDir, storageKey, imageFile.Data)
	if err != nil {
		s.Logger.Error("persist uploaded media", "error", err, "tenant_id", identity.TenantID, "store_id", storeID)
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to persist image")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		_ = os.Remove(storedPath)
		handleSQLError(w, err)
		return
	}
	removeStoredFile := true
	defer func() {
		_ = tx.Rollback()
		if removeStoredFile {
			_ = os.Remove(storedPath)
		}
	}()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = validateMediaGroupID(r.Context(), tx, identity.TenantID, storeID, groupID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MEDIA_GROUP", err.Error())
		return
	}
	var activeAssets int
	var activeBytes int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(size_bytes),0) FROM media_assets WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL`, identity.TenantID, storeID).Scan(&activeAssets, &activeBytes); err != nil {
		handleSQLError(w, err)
		return
	}
	if activeAssets >= mediaMaxAssetsPerStore || activeBytes+int64(len(imageFile.Data)) > mediaMaxBytesPerStore {
		writeError(w, http.StatusInsufficientStorage, "MEDIA_QUOTA_EXCEEDED", "the store image library quota has been reached; delete unused images before uploading")
		return
	}
	var result sql.Result
	if groupID > 0 {
		result, err = tx.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,group_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`, identity.TenantID, storeID, groupID, name, publicURL, storageKey, imageFile.MimeType, imageFile.Width, imageFile.Height, len(imageFile.Data), identity.UserID)
	} else {
		result, err = tx.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`, identity.TenantID, storeID, name, publicURL, storageKey, imageFile.MimeType, imageFile.Width, imageFile.Height, len(imageFile.Data), identity.UserID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	removeStoredFile = false
	createdAt := time.Now().UTC().Format(time.RFC3339)
	s.audit(r.Context(), identity, "media_asset.upload", "media_asset", int64String(id), map[string]any{"name": name, "storage_key": storageKey, "mime_type": imageFile.MimeType, "size_bytes": len(imageFile.Data)}, r)
	view := mediaAssetView{ID: id, Name: name, Kind: "IMAGE", URL: publicURL, StorageKey: storageKey, MimeType: imageFile.MimeType, Width: imageFile.Width, Height: imageFile.Height, SizeBytes: int64(len(imageFile.Data)), Status: "ACTIVE", CreatedAt: createdAt}
	if groupID > 0 {
		view.GroupID = &groupID
	}
	writeData(w, http.StatusCreated, view)
}

func (s *Server) serveMediaAsset(w http.ResponseWriter, r *http.Request) {
	storageKey := strings.TrimPrefix(strings.TrimSpace(chi.URLParam(r, "*")), "/")
	tenantID, storeID, ok := parseLocalMediaStorageKey(storageKey)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	var mimeType string
	err := s.DB.QueryRowContext(r.Context(), `SELECT mime_type FROM media_assets WHERE tenant_id=? AND store_id=? AND storage_key=? AND kind='IMAGE' AND status='ACTIVE' AND deleted_at IS NULL LIMIT 1`, tenantID, storeID, storageKey).Scan(&mimeType)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	allowed, ok := uploadedImageTypes[mimeType]
	if !ok || allowed.Extension != strings.ToLower(filepath.Ext(storageKey)) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	target, err := localMediaPath(s.Config.MediaStorageDir, storageKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	file, err := os.Open(target)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	if err != nil {
		s.Logger.Error("open public media", "error", err, "storage_key", storageKey)
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to read image")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, filepath.Base(storageKey), info.ModTime(), file)
}
