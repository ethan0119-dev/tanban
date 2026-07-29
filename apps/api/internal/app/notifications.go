package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

const (
	announcementStatusDraft     = "DRAFT"
	announcementStatusPublished = "PUBLISHED"
	announcementStatusWithdrawn = "WITHDRAWN"
)

type announcementInput struct {
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	Content      string  `json:"content"`
	Category     string  `json:"category"`
	Severity     string  `json:"severity"`
	AudienceType string  `json:"audience_type"`
	TenantIDs    []int64 `json:"tenant_ids"`
}

type platformAnnouncementDTO struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	Content      string  `json:"content"`
	Category     string  `json:"category"`
	Severity     string  `json:"severity"`
	AudienceType string  `json:"audience_type"`
	Status       string  `json:"status"`
	TenantIDs    []int64 `json:"tenant_ids"`
	TargetCount  int     `json:"target_count"`
	ReadCount    int     `json:"read_count"`
	CreatedBy    int64   `json:"created_by"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	PublishedAt  string  `json:"published_at"`
	WithdrawnAt  string  `json:"withdrawn_at"`
}

type merchantNotificationDTO struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	PublishedAt string `json:"published_at"`
	ReadAt      string `json:"read_at"`
	IsRead      bool   `json:"is_read"`
}

func normalizeAnnouncementInput(input *announcementInput) string {
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Content = strings.TrimSpace(input.Content)
	input.Category = strings.ToUpper(strings.TrimSpace(input.Category))
	input.Severity = strings.ToUpper(strings.TrimSpace(input.Severity))
	input.AudienceType = strings.ToUpper(strings.TrimSpace(input.AudienceType))
	if input.Category == "" {
		input.Category = "SYSTEM_UPDATE"
	}
	if input.Severity == "" {
		input.Severity = "INFO"
	}
	if input.AudienceType == "" {
		input.AudienceType = "ALL"
	}
	if input.Title == "" || len([]rune(input.Title)) > 160 {
		return "title is required and must not exceed 160 characters"
	}
	if len([]rune(input.Summary)) > 300 {
		return "summary must not exceed 300 characters"
	}
	if input.Content == "" || len([]rune(input.Content)) > 20000 {
		return "content is required and must not exceed 20000 characters"
	}
	if !validStatus(input.Category, "SYSTEM_UPDATE", "BUG_FIX", "NEW_FEATURE", "NOTICE", "ACTION_REQUIRED") {
		return "category is invalid"
	}
	if !validStatus(input.Severity, "INFO", "IMPORTANT", "URGENT") {
		return "severity is invalid"
	}
	if !validStatus(input.AudienceType, "ALL", "SELECTED") {
		return "audience_type must be ALL or SELECTED"
	}
	input.TenantIDs = uniquePositiveIDs(input.TenantIDs)
	if input.AudienceType == "SELECTED" && len(input.TenantIDs) == 0 {
		return "tenant_ids is required for selected audience"
	}
	if input.AudienceType == "ALL" {
		input.TenantIDs = nil
	}
	return ""
}

func uniquePositiveIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *Server) listPlatformAnnouncements(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validStatus(status, announcementStatusDraft, announcementStatusPublished, announcementStatusWithdrawn) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status is invalid")
		return
	}
	search := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_announcements WHERE (?='' OR status=?) AND (title LIKE ? OR summary LIKE ?)`, status, status, search, search).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), platformAnnouncementListQuery+` WHERE (?='' OR a.status=?) AND (a.title LIKE ? OR a.summary LIKE ?) ORDER BY a.id DESC LIMIT ? OFFSET ?`, status, status, search, search, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []platformAnnouncementDTO{}
	for rows.Next() {
		item, scanErr := scanPlatformAnnouncement(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

const platformAnnouncementListQuery = `SELECT a.id,a.title,a.summary,a.content,a.category,a.severity,a.audience_type,a.status,a.created_by,
	DATE_FORMAT(a.created_at,'%Y-%m-%d %H:%i:%s'),DATE_FORMAT(a.updated_at,'%Y-%m-%d %H:%i:%s'),
	COALESCE(DATE_FORMAT(a.published_at,'%Y-%m-%d %H:%i:%s'),''),COALESCE(DATE_FORMAT(a.withdrawn_at,'%Y-%m-%d %H:%i:%s'),''),
	CASE WHEN a.status='DRAFT' AND a.audience_type='ALL' THEN (SELECT COUNT(*) FROM tenants t WHERE t.deleted_at IS NULL)
		 WHEN a.status='DRAFT' THEN (SELECT COUNT(*) FROM platform_announcement_targets pat WHERE pat.announcement_id=a.id)
		 ELSE (SELECT COUNT(*) FROM merchant_notification_recipients mnr WHERE mnr.announcement_id=a.id) END,
	(SELECT COUNT(DISTINCT mread.tenant_id) FROM merchant_notification_reads mread WHERE mread.announcement_id=a.id)
	FROM platform_announcements a`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPlatformAnnouncement(scanner rowScanner) (platformAnnouncementDTO, error) {
	var item platformAnnouncementDTO
	err := scanner.Scan(&item.ID, &item.Title, &item.Summary, &item.Content, &item.Category, &item.Severity, &item.AudienceType, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.PublishedAt, &item.WithdrawnAt, &item.TargetCount, &item.ReadCount)
	return item, err
}

func (s *Server) createPlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	var input announcementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if message := normalizeAnnouncementInput(&input); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO platform_announcements(title,summary,content,category,severity,audience_type,status,created_by,updated_by) VALUES(?,?,?,?,?,?,'DRAFT',?,?)`, input.Title, input.Summary, input.Content, input.Category, input.Severity, input.AudienceType, actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if err = replaceAnnouncementTargets(r, tx, id, input.TenantIDs); err != nil {
		writeAnnouncementTargetError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "announcement.create", "platform_announcement", int64String(id), map[string]any{"audience_type": input.AudienceType}, r)
	s.getPlatformAnnouncementByID(w, r, id)
}

func (s *Server) getPlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "announcementID")
	if !ok {
		return
	}
	s.getPlatformAnnouncementByID(w, r, id)
}

func (s *Server) getPlatformAnnouncementByID(w http.ResponseWriter, r *http.Request, id int64) {
	item, err := s.fetchPlatformAnnouncement(r, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) fetchPlatformAnnouncement(r *http.Request, id int64) (platformAnnouncementDTO, error) {
	item, err := scanPlatformAnnouncement(s.DB.QueryRowContext(r.Context(), platformAnnouncementListQuery+" WHERE a.id=?", id))
	if err != nil {
		return item, err
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT tenant_id FROM platform_announcement_targets WHERE announcement_id=? ORDER BY tenant_id", id)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	item.TenantIDs = []int64{}
	for rows.Next() {
		var tenantID int64
		if err = rows.Scan(&tenantID); err != nil {
			return item, err
		}
		item.TenantIDs = append(item.TenantIDs, tenantID)
	}
	return item, rows.Err()
}

func (s *Server) updatePlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "announcementID")
	if !ok {
		return
	}
	var input announcementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if message := normalizeAnnouncementInput(&input); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(r.Context(), "SELECT status FROM platform_announcements WHERE id=? FOR UPDATE", id).Scan(&status); err != nil {
		handleSQLError(w, err)
		return
	}
	if status != announcementStatusDraft {
		writeError(w, http.StatusConflict, "ANNOUNCEMENT_LOCKED", "only draft announcements can be edited")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE platform_announcements SET title=?,summary=?,content=?,category=?,severity=?,audience_type=?,updated_by=? WHERE id=?`, input.Title, input.Summary, input.Content, input.Category, input.Severity, input.AudienceType, actor.UserID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = replaceAnnouncementTargets(r, tx, id, input.TenantIDs); err != nil {
		writeAnnouncementTargetError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "announcement.update", "platform_announcement", int64String(id), map[string]any{"audience_type": input.AudienceType}, r)
	s.getPlatformAnnouncementByID(w, r, id)
}

var errAnnouncementTargetNotFound = errors.New("announcement target tenant not found")

func replaceAnnouncementTargets(r *http.Request, tx *sql.Tx, announcementID int64, tenantIDs []int64) error {
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM platform_announcement_targets WHERE announcement_id=?", announcementID); err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		result, err := tx.ExecContext(r.Context(), `INSERT INTO platform_announcement_targets(announcement_id,tenant_id) SELECT ?,id FROM tenants WHERE id=? AND deleted_at IS NULL`, announcementID, tenantID)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errAnnouncementTargetNotFound
		}
	}
	return nil
}

func writeAnnouncementTargetError(w http.ResponseWriter, err error) {
	if errors.Is(err, errAnnouncementTargetNotFound) {
		writeError(w, http.StatusBadRequest, "INVALID_TENANT", "one or more target tenants do not exist")
		return
	}
	handleSQLError(w, err)
}

func (s *Server) publishPlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "announcementID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var status, audienceType string
	if err = tx.QueryRowContext(r.Context(), "SELECT status,audience_type FROM platform_announcements WHERE id=? FOR UPDATE", id).Scan(&status, &audienceType); err != nil {
		handleSQLError(w, err)
		return
	}
	if status != announcementStatusDraft {
		writeError(w, http.StatusConflict, "ANNOUNCEMENT_LOCKED", "only draft announcements can be published")
		return
	}
	var result sql.Result
	if audienceType == "ALL" {
		result, err = tx.ExecContext(r.Context(), `INSERT IGNORE INTO merchant_notification_recipients(announcement_id,tenant_id) SELECT ?,id FROM tenants WHERE deleted_at IS NULL`, id)
	} else {
		result, err = tx.ExecContext(r.Context(), `INSERT IGNORE INTO merchant_notification_recipients(announcement_id,tenant_id) SELECT ?,pat.tenant_id FROM platform_announcement_targets pat JOIN tenants t ON t.id=pat.tenant_id AND t.deleted_at IS NULL WHERE pat.announcement_id=?`, id, id)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusBadRequest, "EMPTY_AUDIENCE", "announcement has no available recipient tenant")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE platform_announcements SET status='PUBLISHED',published_at=NOW(3),withdrawn_at=NULL,updated_by=? WHERE id=?`, actor.UserID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "announcement.publish", "platform_announcement", int64String(id), map[string]any{"recipient_tenants": affected}, r)
	s.getPlatformAnnouncementByID(w, r, id)
}

func (s *Server) withdrawPlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "announcementID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE platform_announcements SET status='WITHDRAWN',withdrawn_at=NOW(3),updated_by=? WHERE id=? AND status='PUBLISHED'`, actor.UserID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		writeError(w, http.StatusConflict, "ANNOUNCEMENT_LOCKED", "only published announcements can be withdrawn")
		return
	}
	s.audit(r.Context(), actor, "announcement.withdraw", "platform_announcement", int64String(id), nil, r)
	s.getPlatformAnnouncementByID(w, r, id)
}

func (s *Server) listMerchantNotifications(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	unreadOnly := strings.EqualFold(r.URL.Query().Get("unread_only"), "true") || r.URL.Query().Get("unread_only") == "1"
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' LEFT JOIN merchant_notification_reads mread ON mread.announcement_id=a.id AND mread.user_id=? WHERE recipient.tenant_id=? AND (?=0 OR mread.read_at IS NULL)`, identity.UserID, identity.TenantID, unreadOnly).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), merchantNotificationQuery+` WHERE recipient.tenant_id=? AND (?=0 OR mread.read_at IS NULL) ORDER BY a.published_at DESC,a.id DESC LIMIT ? OFFSET ?`, identity.UserID, identity.TenantID, unreadOnly, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []merchantNotificationDTO{}
	for rows.Next() {
		item, scanErr := scanMerchantNotification(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

const merchantNotificationQuery = `SELECT a.id,a.title,a.summary,a.content,a.category,a.severity,DATE_FORMAT(a.published_at,'%Y-%m-%d %H:%i:%s'),COALESCE(DATE_FORMAT(mread.read_at,'%Y-%m-%d %H:%i:%s'),'') FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' LEFT JOIN merchant_notification_reads mread ON mread.announcement_id=a.id AND mread.user_id=?`

func scanMerchantNotification(scanner rowScanner) (merchantNotificationDTO, error) {
	var item merchantNotificationDTO
	err := scanner.Scan(&item.ID, &item.Title, &item.Summary, &item.Content, &item.Category, &item.Severity, &item.PublishedAt, &item.ReadAt)
	item.IsRead = item.ReadAt != ""
	return item, err
}

func (s *Server) merchantNotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var count int
	err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' LEFT JOIN merchant_notification_reads mread ON mread.announcement_id=a.id AND mread.user_id=? WHERE recipient.tenant_id=? AND mread.read_at IS NULL`, identity.UserID, identity.TenantID).Scan(&count)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]int{"count": count})
}

func (s *Server) getMerchantNotification(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "notificationID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	item, err := scanMerchantNotification(s.DB.QueryRowContext(r.Context(), merchantNotificationQuery+" WHERE recipient.tenant_id=? AND a.id=?", identity.UserID, identity.TenantID, id))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) markMerchantNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "notificationID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var exists int
	err := s.DB.QueryRowContext(r.Context(), `SELECT 1 FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' WHERE recipient.tenant_id=? AND a.id=?`, identity.TenantID, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "notification not found")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = s.DB.ExecContext(r.Context(), `INSERT IGNORE INTO merchant_notification_reads(announcement_id,tenant_id,user_id) VALUES(?,?,?)`, id, identity.TenantID, identity.UserID); err != nil {
		handleSQLError(w, err)
		return
	}
	s.getMerchantNotification(w, r)
}

func (s *Server) markAllMerchantNotificationsRead(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `INSERT IGNORE INTO merchant_notification_reads(announcement_id,tenant_id,user_id) SELECT a.id,recipient.tenant_id,? FROM merchant_notification_recipients recipient JOIN platform_announcements a ON a.id=recipient.announcement_id AND a.status='PUBLISHED' LEFT JOIN merchant_notification_reads mread ON mread.announcement_id=a.id AND mread.user_id=? WHERE recipient.tenant_id=? AND mread.read_at IS NULL`, identity.UserID, identity.UserID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	writeData(w, http.StatusOK, map[string]int64{"read_count": affected})
}
