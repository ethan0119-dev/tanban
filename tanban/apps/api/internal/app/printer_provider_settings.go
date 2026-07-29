package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

const printerProviderSettingKey = "printer_providers"

type printerProviderStored struct {
	Enabled      bool   `json:"enabled"`
	DeveloperID  string `json:"developer_id"`
	SecretCipher string `json:"secret_cipher"`
	BaseURL      string `json:"base_url"`
}

type printerProvidersStored struct {
	XPYun printerProviderStored `json:"xpyun"`
}

type printerProviderInput struct {
	Enabled     bool   `json:"enabled"`
	DeveloperID string `json:"developerId"`
	Secret      string `json:"secret"`
	BaseURL     string `json:"baseUrl"`
}

type printerProviderView struct {
	Provider     string `json:"provider"`
	DisplayName  string `json:"displayName"`
	Enabled      bool   `json:"enabled"`
	DeveloperID  string `json:"developerId"`
	SecretSet    bool   `json:"secretSet"`
	BaseURL      string `json:"baseUrl"`
	Configured   bool   `json:"configured"`
	AutoRegister bool   `json:"autoRegister"`
	Synced       int    `json:"synced,omitempty"`
	SyncFailed   int    `json:"syncFailed,omitempty"`
}

func (s *Server) getPlatformPrinterProviders(w http.ResponseWriter, r *http.Request) {
	stored, _ := s.loadPrinterProviders(r.Context())
	writeData(w, http.StatusOK, []printerProviderView{s.printerProviderView(stored.XPYun)})
}

func (s *Server) updatePlatformXPYun(w http.ResponseWriter, r *http.Request) {
	var input printerProviderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.DeveloperID = strings.TrimSpace(input.DeveloperID)
	input.Secret = strings.TrimSpace(input.Secret)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.BaseURL == "" {
		input.BaseURL = "https://open.xpyun.net/api/openapi/xprinter"
	}
	stored, err := s.loadPrinterProviders(r.Context())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	current := stored.XPYun
	if input.Enabled && input.DeveloperID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "developerId is required when XPYun is enabled")
		return
	}
	if input.Secret != "" {
		current.SecretCipher, err = s.encryptPrinterSecret(input.Secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ENCRYPTION_ERROR", "打印服务商密钥加密失败")
			return
		}
	}
	if input.Enabled && current.SecretCipher == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "secret is required when XPYun is enabled")
		return
	}
	current.Enabled = input.Enabled
	current.DeveloperID = input.DeveloperID
	current.BaseURL = input.BaseURL
	stored.XPYun = current
	body, _ := json.Marshal(stored)
	identity := currentIdentity(r.Context())
	if _, err = s.DB.ExecContext(r.Context(), `INSERT INTO platform_settings(setting_key,value_text,updated_by) VALUES(?,?,?) ON DUPLICATE KEY UPDATE value_text=VALUES(value_text),updated_by=VALUES(updated_by)`, printerProviderSettingKey, string(body), identity.UserID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = s.applyStoredPrinterProviders(stored); err != nil {
		writeError(w, http.StatusInternalServerError, "CONFIGURATION_ERROR", err.Error())
		return
	}
	synced, failed := 0, 0
	if current.Enabled {
		synced, failed = s.syncExistingXPYunPrinters(r.Context())
	}
	s.audit(r.Context(), identity, "settings.printer_provider.update", "settings", "xpyun", map[string]any{"enabled": current.Enabled, "developer_id": current.DeveloperID, "secret_updated": input.Secret != "", "synced": synced, "sync_failed": failed}, r)
	view := s.printerProviderView(current)
	view.Synced, view.SyncFailed = synced, failed
	writeData(w, http.StatusOK, view)
}

func (s *Server) testPlatformXPYun(w http.ResponseWriter, r *http.Request) {
	var name, sn string
	err := s.DB.QueryRowContext(r.Context(), `SELECT name,sn FROM printer_devices WHERE deleted_at IS NULL AND status='ACTIVE' AND LOWER(provider) IN ('xpyun','xprinter','x-printer','芯烨','芯烨云') ORDER BY id LIMIT 1`).Scan(&name, &sn)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "NO_DEVICE", "请先录入至少一台芯烨打印机")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	result := s.Printer.Status(ctx, provider.PrinterStatusRequest{Provider: "xpyun", DeviceSN: sn})
	writeData(w, http.StatusOK, map[string]any{"deviceName": name, "status": result.Status, "message": result.Message, "checkedAt": result.CheckedAt})
}

func (s *Server) printerProviderView(value printerProviderStored) printerProviderView {
	if value.BaseURL == "" {
		value.BaseURL = "https://open.xpyun.net/api/openapi/xprinter"
	}
	return printerProviderView{Provider: "xpyun", DisplayName: "芯烨云", Enabled: value.Enabled, DeveloperID: value.DeveloperID, SecretSet: value.SecretCipher != "", BaseURL: value.BaseURL, Configured: value.Enabled && value.DeveloperID != "" && value.SecretCipher != "", AutoRegister: true}
}

func (s *Server) loadPrinterProviders(ctx context.Context) (printerProvidersStored, error) {
	var body string
	err := s.DB.QueryRowContext(ctx, "SELECT value_text FROM platform_settings WHERE setting_key=?", printerProviderSettingKey).Scan(&body)
	if err != nil {
		return printerProvidersStored{}, err
	}
	var stored printerProvidersStored
	err = json.Unmarshal([]byte(body), &stored)
	return stored, err
}

func (s *Server) applyStoredPrinterProviders(stored printerProvidersStored) error {
	router, ok := s.Printer.(*provider.PrinterRouter)
	if !ok {
		return nil
	}
	value := stored.XPYun
	if !value.Enabled {
		router.ConfigureXPYun(provider.XPrinterConfig{BaseURL: value.BaseURL})
		return nil
	}
	secret, err := s.decryptPrinterSecret(value.SecretCipher)
	if err != nil {
		return fmt.Errorf("读取芯烨云密钥失败")
	}
	router.ConfigureXPYun(provider.XPrinterConfig{BaseURL: value.BaseURL, User: value.DeveloperID, UserKey: secret})
	return nil
}

func (s *Server) loadPrinterProviderRuntime(ctx context.Context) {
	stored, err := s.loadPrinterProviders(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil || s.applyStoredPrinterProviders(stored) != nil {
		if s.Logger != nil {
			s.Logger.Error("load printer provider settings", "error", err)
		}
	}
}

func (s *Server) syncExistingXPYunPrinters(ctx context.Context) (int, int) {
	rows, err := s.DB.QueryContext(ctx, `SELECT name,sn FROM printer_devices WHERE deleted_at IS NULL AND status='ACTIVE' AND LOWER(provider) IN ('xpyun','xprinter','x-printer','芯烨','芯烨云') ORDER BY id`)
	if err != nil {
		return 0, 1
	}
	defer rows.Close()
	synced, failed := 0, 0
	for rows.Next() {
		var name, sn string
		if rows.Scan(&name, &sn) != nil || s.syncPrinterRegistration(ctx, "xpyun", name, sn) != nil {
			failed++
		} else {
			synced++
		}
	}
	return synced, failed
}

func (s *Server) syncPrinterRegistration(ctx context.Context, providerName, name, sn string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	status := s.Printer.Status(checkCtx, provider.PrinterStatusRequest{Provider: providerName, DeviceSN: sn})
	if status.Status == "ONLINE" || status.Status == "OFFLINE" || status.Status == "PAPER_OUT" || status.Status == "SIMULATED" {
		return nil
	}
	if !strings.Contains(status.Message, "尚未绑定") {
		return errors.New(status.Message)
	}
	return s.Printer.Register(checkCtx, provider.PrinterRegistrationRequest{Provider: providerName, DeviceSN: sn, Name: name})
}

func (s *Server) printerSecretAEAD() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(s.Config.JWTSecret + ":printer-provider-credentials:v1"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) encryptPrinterSecret(value string) (string, error) {
	aead, err := s.printerSecretAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), []byte(printerProviderSettingKey))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (s *Server) decryptPrinterSecret(value string) (string, error) {
	aead, err := s.printerSecretAEAD()
	if err != nil {
		return "", err
	}
	sealed, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(sealed) < aead.NonceSize() {
		return "", errors.New("invalid encrypted secret")
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(printerProviderSettingKey))
	return string(plain), err
}
