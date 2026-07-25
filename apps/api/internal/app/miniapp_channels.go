package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	publicMiniAppChannelKey = "tanban-public"
	miniAppSecretAAD        = "tanban:miniapp-channel-secret:v1"
)

type tenantMiniAppSettings struct {
	PrimaryMode          string `json:"primaryMode"`
	PublicEnabled        bool   `json:"publicEnabled"`
	PublicAppID          string `json:"publicAppId"`
	PublicConfigured     bool   `json:"publicConfigured"`
	DedicatedEnabled     bool   `json:"dedicatedEnabled"`
	DedicatedDisplayName string `json:"dedicatedDisplayName"`
	DedicatedChannelKey  string `json:"dedicatedChannelKey"`
	DedicatedAppID       string `json:"dedicatedAppId"`
	AppSecretConfigured  bool   `json:"appSecretConfigured"`
}

type tenantMiniAppSettingsInput struct {
	PrimaryMode          string `json:"primaryMode"`
	PublicEnabled        bool   `json:"publicEnabled"`
	DedicatedEnabled     bool   `json:"dedicatedEnabled"`
	DedicatedDisplayName string `json:"dedicatedDisplayName"`
	DedicatedAppID       string `json:"dedicatedAppId"`
	DedicatedAppSecret   string `json:"dedicatedAppSecret"`
}

type miniAppCredentials struct {
	TenantID   int64
	Mode       string
	ChannelKey string
	AppID      string
	AppSecret  string
}

func nullableMiniAppString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func validMiniAppID(value string) bool {
	if len(value) != 18 || !strings.HasPrefix(value, "wx") {
		return false
	}
	for _, char := range value[2:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

func (s *Server) miniAppSecretAEAD() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(s.Config.JWTSecret + ":" + miniAppSecretAAD))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) encryptMiniAppSecret(value string) (string, error) {
	aead, err := s.miniAppSecretAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), []byte(miniAppSecretAAD))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (s *Server) decryptMiniAppSecret(value string) (string, error) {
	aead, err := s.miniAppSecretAEAD()
	if err != nil {
		return "", err
	}
	sealed, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(sealed) < aead.NonceSize() {
		return "", errors.New("invalid encrypted miniapp secret")
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(miniAppSecretAAD))
	return string(plain), err
}

func (s *Server) miniAppSettingsForTenant(r *http.Request, tenantID int64) (tenantMiniAppSettings, error) {
	settings := tenantMiniAppSettings{
		PrimaryMode: "PUBLIC", PublicEnabled: true,
		PublicAppID:      s.Config.WeChatMiniApp.AppID,
		PublicConfigured: s.Config.WeChatMiniApp.AppID != "" && s.Config.WeChatMiniApp.AppSecret != "",
	}
	var tenantExists bool
	if err := s.DB.QueryRowContext(r.Context(),
		"SELECT EXISTS(SELECT 1 FROM tenants WHERE id=? AND deleted_at IS NULL)", tenantID,
	).Scan(&tenantExists); err != nil {
		return settings, err
	}
	if !tenantExists {
		return settings, sql.ErrNoRows
	}
	var secretCipher string
	err := s.DB.QueryRowContext(r.Context(), `SELECT primary_mode,public_enabled,dedicated_enabled,
		dedicated_display_name,COALESCE(dedicated_channel_key,''),COALESCE(dedicated_appid,''),dedicated_app_secret_cipher
		FROM tenant_miniapp_channels WHERE tenant_id=?`, tenantID).
		Scan(&settings.PrimaryMode, &settings.PublicEnabled, &settings.DedicatedEnabled,
			&settings.DedicatedDisplayName, &settings.DedicatedChannelKey, &settings.DedicatedAppID, &secretCipher)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	settings.AppSecretConfigured = secretCipher != ""
	return settings, err
}

func (s *Server) getTenantMiniAppSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	settings, err := s.miniAppSettingsForTenant(r, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func (s *Server) updateTenantMiniAppSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input tenantMiniAppSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.PrimaryMode = strings.ToUpper(strings.TrimSpace(input.PrimaryMode))
	input.DedicatedDisplayName = strings.TrimSpace(input.DedicatedDisplayName)
	input.DedicatedAppID = strings.TrimSpace(input.DedicatedAppID)
	input.DedicatedAppSecret = strings.TrimSpace(input.DedicatedAppSecret)
	if !validStatus(input.PrimaryMode, "PUBLIC", "DEDICATED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "primaryMode must be PUBLIC or DEDICATED")
		return
	}
	if input.PrimaryMode == "PUBLIC" && !input.PublicEnabled {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "the primary public miniapp channel must be enabled")
		return
	}
	if input.PrimaryMode == "DEDICATED" {
		input.DedicatedEnabled = true
	}
	if input.DedicatedEnabled && !validMiniAppID(input.DedicatedAppID) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "独立小程序 AppID 格式不正确")
		return
	}
	if len([]rune(input.DedicatedDisplayName)) > 120 || len(input.DedicatedAppSecret) > 256 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "小程序名称或 AppSecret 过长")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM tenants WHERE id=? AND deleted_at IS NULL)", tenantID).Scan(&exists); err != nil || !exists {
		if err != nil {
			handleSQLError(w, err)
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		}
		return
	}
	var channelKey, secretCipher string
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(dedicated_channel_key,''),dedicated_app_secret_cipher
		FROM tenant_miniapp_channels WHERE tenant_id=? FOR UPDATE`, tenantID).Scan(&channelKey, &secretCipher)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	if channelKey == "" {
		channelKey = strings.ToLower(newBusinessNo("WX"))
	}
	if input.DedicatedAppSecret != "" {
		secretCipher, err = s.encryptMiniAppSecret(input.DedicatedAppSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "CREDENTIAL_ENCRYPTION_FAILED", "小程序密钥保存失败")
			return
		}
	}
	if input.DedicatedEnabled && secretCipher == "" {
		writeError(w, http.StatusBadRequest, "APP_SECRET_REQUIRED", "首次启用独立小程序时必须填写 AppSecret")
		return
	}
	actor := currentIdentity(r.Context())
	_, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_miniapp_channels(
		tenant_id,primary_mode,public_enabled,dedicated_enabled,dedicated_display_name,dedicated_channel_key,
		dedicated_appid,dedicated_app_secret_cipher,created_by,updated_by
	) VALUES(?,?,?,?,?,?,?,?,?,?)
	ON DUPLICATE KEY UPDATE primary_mode=VALUES(primary_mode),public_enabled=VALUES(public_enabled),
		dedicated_enabled=VALUES(dedicated_enabled),dedicated_display_name=VALUES(dedicated_display_name),
		dedicated_channel_key=VALUES(dedicated_channel_key),dedicated_appid=VALUES(dedicated_appid),
		dedicated_app_secret_cipher=VALUES(dedicated_app_secret_cipher),updated_by=VALUES(updated_by)`,
		tenantID, input.PrimaryMode, input.PublicEnabled, input.DedicatedEnabled, input.DedicatedDisplayName,
		nullableMiniAppString(channelKey), nullableMiniAppString(input.DedicatedAppID), secretCipher, actor.UserID, actor.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			writeError(w, http.StatusConflict, "MINIAPP_ALREADY_BOUND", "该 AppID 已绑定到其他商户")
			return
		}
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "tenant.miniapp.update", "tenant", int64String(tenantID), map[string]any{
		"primary_mode": input.PrimaryMode, "public_enabled": input.PublicEnabled,
		"dedicated_enabled": input.DedicatedEnabled, "appid": input.DedicatedAppID,
		"secret_updated": input.DedicatedAppSecret != "",
	}, r)
	settings, err := s.miniAppSettingsForTenant(r, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func (s *Server) resolveMiniAppCredentials(r *http.Request, store storeDTO, requestedChannel string) (miniAppCredentials, error) {
	channelKey := strings.TrimSpace(requestedChannel)
	if channelKey == "" || channelKey == publicMiniAppChannelKey {
		if s.Config.WeChatMiniApp.AppID == "" || s.Config.WeChatMiniApp.AppSecret == "" {
			return miniAppCredentials{}, errors.New("public mini-program login is not configured")
		}
		var publicEnabled bool
		err := s.DB.QueryRowContext(r.Context(), `SELECT COALESCE((
			SELECT public_enabled FROM tenant_miniapp_channels WHERE tenant_id=?
		),1)`, store.TenantID).Scan(&publicEnabled)
		if err != nil {
			return miniAppCredentials{}, err
		}
		if !publicEnabled {
			return miniAppCredentials{}, errors.New("this store no longer accepts the public mini-program channel")
		}
		return miniAppCredentials{TenantID: store.TenantID, Mode: "PUBLIC", ChannelKey: publicMiniAppChannelKey, AppID: s.Config.WeChatMiniApp.AppID, AppSecret: s.Config.WeChatMiniApp.AppSecret}, nil
	}
	var credentials miniAppCredentials
	var secretCipher string
	err := s.DB.QueryRowContext(r.Context(), `SELECT tenant_id,COALESCE(dedicated_channel_key,''),COALESCE(dedicated_appid,''),dedicated_app_secret_cipher
		FROM tenant_miniapp_channels
		WHERE dedicated_channel_key=? AND dedicated_enabled=1`, channelKey).
		Scan(&credentials.TenantID, &credentials.ChannelKey, &credentials.AppID, &secretCipher)
	if err != nil {
		return miniAppCredentials{}, err
	}
	if credentials.TenantID != store.TenantID {
		return miniAppCredentials{}, errors.New("the dedicated mini-program is not allowed to access this store")
	}
	credentials.Mode = "DEDICATED"
	credentials.AppSecret, err = s.decryptMiniAppSecret(secretCipher)
	return credentials, err
}
