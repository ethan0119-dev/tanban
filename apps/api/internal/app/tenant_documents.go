package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type tenantDocumentDefinition struct {
	Column    string
	Name      string
	AuditType string
}

func resolveTenantDocumentDefinition(value string) (tenantDocumentDefinition, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "business-license":
		return tenantDocumentDefinition{Column: "business_license_media_id", Name: "营业执照", AuditType: "BUSINESS_LICENSE"}, true
	case "food-business-license":
		return tenantDocumentDefinition{Column: "food_business_license_media_id", Name: "食品经营许可证", AuditType: "FOOD_BUSINESS_LICENSE"}, true
	default:
		return tenantDocumentDefinition{}, false
	}
}

func (s *Server) uploadTenantDocument(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	document, ok := resolveTenantDocumentDefinition(chi.URLParam(r, "documentType"))
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_DOCUMENT_TYPE", "documentType must be business-license or food-business-license")
		return
	}

	var storeID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT s.id FROM tenants t JOIN stores s ON s.tenant_id=t.id AND s.deleted_at IS NULL WHERE t.id=? AND t.deleted_at IS NULL ORDER BY s.id LIMIT 1`, tenantID).Scan(&storeID); err != nil {
		handleSQLError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, mediaMaxMultipartBytes)
	if err := r.ParseMultipartForm(mediaMaxUploadBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", fmt.Sprintf("image cannot exceed %d MiB", mediaMaxUploadBytes/(1024*1024)))
		} else {
			writeError(w, http.StatusBadRequest, "INVALID_MULTIPART", "a multipart form with file is required")
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

	storageKey, err := newMediaStorageKey(tenantID, storeID, imageFile.Extension, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to allocate document storage")
		return
	}
	publicURL, err := mediaPublicURL(s.Config.MediaPublicBaseURL, storageKey)
	if err != nil {
		s.Logger.Error("build tenant document public url", "error", err)
		writeError(w, http.StatusServiceUnavailable, "MEDIA_STORAGE_NOT_CONFIGURED", "media public URL is not configured")
		return
	}
	storedPath, err := persistUploadedImage(s.Config.MediaStorageDir, storageKey, imageFile.Data)
	if err != nil {
		s.Logger.Error("persist tenant document", "error", err, "tenant_id", tenantID)
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to persist document")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		_ = os.Remove(storedPath)
		handleSQLError(w, err)
		return
	}
	removeNewFile := true
	defer func() {
		_ = tx.Rollback()
		if removeNewFile {
			_ = os.Remove(storedPath)
		}
	}()

	var previousID int64
	var previousStorageKey string
	lockQuery := fmt.Sprintf(`SELECT COALESCE(t.%s,0),COALESCE(a.storage_key,'') FROM tenants t LEFT JOIN media_assets a ON a.id=t.%s AND a.tenant_id=t.id WHERE t.id=? AND t.deleted_at IS NULL FOR UPDATE`, document.Column, document.Column)
	if err = tx.QueryRowContext(r.Context(), lockQuery, tenantID).Scan(&previousID, &previousStorageKey); err != nil {
		handleSQLError(w, err)
		return
	}
	identity := currentIdentity(r.Context())
	result, err := tx.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,'TENANT_DOCUMENT',?,?,?,?,?,?,'ACTIVE',?)`, tenantID, storeID, document.Name, publicURL, storageKey, imageFile.MimeType, imageFile.Width, imageFile.Height, len(imageFile.Data), identity.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	assetID, err := result.LastInsertId()
	if err != nil {
		handleSQLError(w, err)
		return
	}
	updateQuery := fmt.Sprintf(`UPDATE tenants SET %s=? WHERE id=? AND deleted_at IS NULL`, document.Column)
	if _, err = tx.ExecContext(r.Context(), updateQuery, assetID, tenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if previousID > 0 {
		if _, err = tx.ExecContext(r.Context(), `UPDATE media_assets SET status='DELETED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND kind='TENANT_DOCUMENT' AND deleted_at IS NULL`, previousID, tenantID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	removeNewFile = false
	if isLocalMediaStorageKey(previousStorageKey) {
		if previousPath, pathErr := localMediaPath(s.Config.MediaStorageDir, previousStorageKey); pathErr != nil {
			s.Logger.Error("resolve replaced tenant document path", "error", pathErr, "tenant_id", tenantID, "asset_id", previousID)
		} else if removeErr := os.Remove(previousPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			s.Logger.Error("remove replaced tenant document", "error", removeErr, "tenant_id", tenantID, "asset_id", previousID)
		}
	}
	s.audit(r.Context(), identity, "tenant.document.upload", "tenant", int64String(tenantID), map[string]any{
		"document_type": document.AuditType,
		"asset_id":      assetID,
		"mime_type":     imageFile.MimeType,
		"size_bytes":    len(imageFile.Data),
	}, r)
	s.getTenantByID(w, r, tenantID)
}
