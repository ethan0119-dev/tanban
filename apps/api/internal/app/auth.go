package app

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	RolePlatformAdmin    = "PLATFORM_ADMIN"
	RolePlatformOperator = "PLATFORM_OPERATOR"
	RoleMerchantOwner    = "MERCHANT_OWNER"
	RoleMerchantManager  = "MERCHANT_MANAGER"
	RoleMerchantStaff    = "MERCHANT_STAFF"
)

type identity struct {
	UserID           int64  `json:"user_id"`
	MembershipID     int64  `json:"membership_id,omitempty"`
	TenantID         int64  `json:"tenant_id"`
	StoreID          int64  `json:"store_id,omitempty"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	TenantName       string `json:"tenant_name,omitempty"`
	StoreName        string `json:"store_name,omitempty"`
	Role             string `json:"role"`
	ServiceExpiresAt string `json:"service_expires_at,omitempty"`
	ServiceExpired   bool   `json:"service_expired,omitempty"`
}

type claims struct {
	MembershipID int64  `json:"membership_id,omitempty"`
	TenantID     int64  `json:"tenant_id"`
	Role         string `json:"role"`
	Username     string `json:"username"`
	TokenKind    string `json:"token_kind,omitempty"`
	jwt.RegisteredClaims
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Portal   string `json:"portal"`
}

type tenantSelectionRequest struct {
	SelectionToken string `json:"selection_token"`
	TenantID       int64  `json:"tenant_id"`
}

type tenantSwitchRequest struct {
	TenantID int64 `json:"tenant_id"`
}

type merchantWorkspace struct {
	MembershipID     int64  `json:"membership_id"`
	TenantID         int64  `json:"tenant_id"`
	TenantName       string `json:"tenant_name"`
	StoreID          int64  `json:"store_id"`
	StoreName        string `json:"store_name"`
	StoreLogoURL     string `json:"store_logo_url"`
	Role             string `json:"role"`
	ServiceExpiresAt string `json:"service_expires_at,omitempty"`
	ServiceExpired   bool   `json:"service_expired"`
}

type changeMerchantPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) changeMerchantPassword(w http.ResponseWriter, r *http.Request) {
	var input changeMerchantPasswordRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请输入当前密码")
		return
	}
	// bcrypt accepts at most 72 bytes (not runes). Enforce the same boundary
	// before hashing so an oversized password is reported as a validation error.
	if len([]byte(input.NewPassword)) < 8 || len([]byte(input.NewPassword)) > 72 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "新密码须为 8 至 72 位")
		return
	}
	if input.NewPassword == input.CurrentPassword {
		writeError(w, http.StatusBadRequest, "PASSWORD_UNCHANGED", "新密码不能与当前密码相同")
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var currentHash string
	if err = tx.QueryRowContext(r.Context(), `SELECT a.password_hash FROM accounts a
		JOIN tenant_memberships m ON m.account_id=a.id AND m.tenant_id=? AND m.status='ACTIVE' AND m.deleted_at IS NULL
		WHERE a.id=? AND a.status='ACTIVE' AND a.deleted_at IS NULL FOR UPDATE`, actor.TenantID, actor.UserID).Scan(&currentHash); err != nil {
		handleSQLError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.CurrentPassword)) != nil {
		writeError(w, http.StatusBadRequest, "CURRENT_PASSWORD_INCORRECT", "当前密码不正确")
		return
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_ERROR", "密码更新失败")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE accounts SET password_hash=?,updated_at=NOW(3) WHERE id=? AND status='ACTIVE' AND deleted_at IS NULL`, string(nextHash), actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		writeError(w, http.StatusConflict, "ACCOUNT_CHANGED", "账号状态已变化，请重新登录后再试")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "account.password.change", "user", int64String(actor.UserID), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"changed": true})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Portal = strings.ToLower(strings.TrimSpace(input.Portal))
	if input.Portal != "" && input.Portal != "platform" && input.Portal != "merchant" {
		writeError(w, http.StatusBadRequest, "INVALID_PORTAL", "portal must be platform or merchant")
		return
	}
	rateKey := "login:" + r.RemoteAddr + ":" + strings.ToLower(input.Username)
	if raw, err := s.Cache.Get(r.Context(), rateKey); err == nil {
		attempts, _ := strconv.Atoi(string(raw))
		if attempts >= 5 {
			writeError(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "too many failed attempts; retry in five minutes")
			return
		}
	}
	var accountID int64
	var username, displayName, passwordHash, status, platformRole string
	err := s.DB.QueryRowContext(r.Context(), `SELECT id,username,display_name,password_hash,status,COALESCE(platform_role,'')
		FROM accounts WHERE username=? AND deleted_at IS NULL`, input.Username).
		Scan(&accountID, &username, &displayName, &passwordHash, &status, &platformRole)
	if err != nil || status != "ACTIVE" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		attempts := 0
		if raw, cacheErr := s.Cache.Get(r.Context(), rateKey); cacheErr == nil {
			attempts, _ = strconv.Atoi(string(raw))
		}
		_ = s.Cache.Set(r.Context(), rateKey, []byte(strconv.Itoa(attempts+1)), 5*time.Minute)
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "username or password is incorrect")
		return
	}
	_ = s.Cache.Delete(r.Context(), rateKey)
	if input.Portal == "platform" || input.Portal == "" && strings.HasPrefix(platformRole, "PLATFORM_") {
		if !strings.HasPrefix(platformRole, "PLATFORM_") {
			writeError(w, http.StatusForbidden, "PORTAL_FORBIDDEN", "account cannot sign in to this portal")
			return
		}
		s.writeAccessToken(w, r, identity{UserID: accountID, Username: username, DisplayName: displayName, Role: platformRole})
		return
	}
	if input.Portal != "merchant" && input.Portal != "" {
		writeError(w, http.StatusForbidden, "PORTAL_FORBIDDEN", "account cannot sign in to this portal")
		return
	}
	workspaces, err := s.loadMerchantWorkspaces(r.Context(), accountID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if len(workspaces) == 0 {
		writeError(w, http.StatusForbidden, "PORTAL_FORBIDDEN", "account has no active merchant workspace")
		return
	}
	if len(workspaces) == 1 {
		s.writeAccessToken(w, r, workspaceIdentity(accountID, username, displayName, workspaces[0]))
		return
	}
	for _, workspace := range workspaces {
		if workspace.Role != RoleMerchantOwner {
			writeError(w, http.StatusConflict, "MULTI_TENANT_STAFF_NOT_ALLOWED", "staff accounts can only belong to one store")
			return
		}
	}
	selectionToken, err := s.signSelectionToken(accountID, username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not issue tenant selection token")
		return
	}
	s.audit(r.Context(), identity{UserID: accountID, Username: username, DisplayName: displayName, Role: RoleMerchantOwner}, "auth.workspace_selection.required", "account", int64String(accountID), map[string]any{"workspace_count": len(workspaces)}, r)
	writeData(w, http.StatusOK, map[string]any{
		"selection_required": true, "selectionRequired": true,
		"selection_token": selectionToken, "selectionToken": selectionToken,
		"workspaces": workspaces,
	})
}

func workspaceIdentity(accountID int64, username, displayName string, workspace merchantWorkspace) identity {
	return identity{
		UserID: accountID, MembershipID: workspace.MembershipID, TenantID: workspace.TenantID, StoreID: workspace.StoreID,
		Username: username, DisplayName: displayName, TenantName: workspace.TenantName, StoreName: workspace.StoreName, Role: workspace.Role,
		ServiceExpiresAt: workspace.ServiceExpiresAt, ServiceExpired: workspace.ServiceExpired,
	}
}

func (s *Server) loadMerchantWorkspaces(ctx context.Context, accountID int64) ([]merchantWorkspace, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT m.id,m.tenant_id,t.name,s.id,s.name,s.logo_url,m.role,
		COALESCE(DATE_FORMAT(t.service_expires_at,'%Y-%m-%d'),''),(t.service_expires_at IS NOT NULL AND t.service_expires_at < CURRENT_DATE)
		FROM tenant_memberships m
		JOIN tenants t ON t.id=m.tenant_id AND t.status='ACTIVE' AND t.deleted_at IS NULL
		JOIN stores s ON s.id=(SELECT s2.id FROM stores s2 WHERE s2.tenant_id=t.id AND s2.deleted_at IS NULL ORDER BY s2.id LIMIT 1)
		WHERE m.account_id=? AND m.status='ACTIVE' AND m.deleted_at IS NULL
		ORDER BY t.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []merchantWorkspace{}
	for rows.Next() {
		var item merchantWorkspace
		if err = rows.Scan(&item.MembershipID, &item.TenantID, &item.TenantName, &item.StoreID, &item.StoreName, &item.StoreLogoURL, &item.Role, &item.ServiceExpiresAt, &item.ServiceExpired); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) loadMerchantWorkspace(ctx context.Context, accountID, tenantID int64) (merchantWorkspace, error) {
	var item merchantWorkspace
	err := s.DB.QueryRowContext(ctx, `SELECT m.id,m.tenant_id,t.name,s.id,s.name,s.logo_url,m.role,
		COALESCE(DATE_FORMAT(t.service_expires_at,'%Y-%m-%d'),''),(t.service_expires_at IS NOT NULL AND t.service_expires_at < CURRENT_DATE)
		FROM tenant_memberships m
		JOIN tenants t ON t.id=m.tenant_id AND t.status='ACTIVE' AND t.deleted_at IS NULL
		JOIN stores s ON s.id=(SELECT s2.id FROM stores s2 WHERE s2.tenant_id=t.id AND s2.deleted_at IS NULL ORDER BY s2.id LIMIT 1)
		WHERE m.account_id=? AND m.tenant_id=? AND m.status='ACTIVE' AND m.deleted_at IS NULL`, accountID, tenantID).
		Scan(&item.MembershipID, &item.TenantID, &item.TenantName, &item.StoreID, &item.StoreName, &item.StoreLogoURL, &item.Role, &item.ServiceExpiresAt, &item.ServiceExpired)
	return item, err
}

func (s *Server) loadActiveAccount(ctx context.Context, accountID int64) (string, string, error) {
	var username, displayName string
	err := s.DB.QueryRowContext(ctx, `SELECT username,display_name FROM accounts WHERE id=? AND status='ACTIVE' AND deleted_at IS NULL`, accountID).Scan(&username, &displayName)
	return username, displayName, err
}

func (s *Server) signSelectionToken(accountID int64, username string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Username: username, TokenKind: "tenant_selection",
		RegisteredClaims: jwt.RegisteredClaims{Subject: int64String(accountID), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)), Issuer: "tanban-api"},
	}).SignedString([]byte(s.Config.JWTSecret))
}

func (s *Server) parseSelectionToken(raw string) (int64, error) {
	parsed := &claims{}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(raw), parsed, func(token *jwt.Token) (any, error) {
		return []byte(s.Config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("tanban-api"))
	if err != nil || !token.Valid || parsed.TokenKind != "tenant_selection" {
		return 0, sql.ErrNoRows
	}
	return parseInt64(parsed.Subject)
}

func (s *Server) selectTenant(w http.ResponseWriter, r *http.Request) {
	var input tenantSelectionRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	accountID, err := s.parseSelectionToken(input.SelectionToken)
	if err != nil || input.TenantID <= 0 {
		writeError(w, http.StatusUnauthorized, "INVALID_SELECTION_TOKEN", "store selection has expired; please sign in again")
		return
	}
	username, displayName, err := s.loadActiveAccount(r.Context(), accountID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "account is unavailable")
		return
	}
	workspace, err := s.loadMerchantWorkspace(r.Context(), accountID, input.TenantID)
	if err != nil {
		writeError(w, http.StatusForbidden, "WORKSPACE_FORBIDDEN", "account cannot access this store")
		return
	}
	s.writeAccessToken(w, r, workspaceIdentity(accountID, username, displayName, workspace))
}

func (s *Server) listAuthWorkspaces(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	if !strings.HasPrefix(actor.Role, "MERCHANT_") {
		writeError(w, http.StatusForbidden, "PORTAL_FORBIDDEN", "platform accounts do not have merchant workspaces")
		return
	}
	items, err := s.loadMerchantWorkspaces(r.Context(), actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) switchTenant(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input tenantSwitchRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if actor.Role != RoleMerchantOwner {
		writeError(w, http.StatusForbidden, "WORKSPACE_SWITCH_FORBIDDEN", "only owner accounts can switch stores")
		return
	}
	workspace, err := s.loadMerchantWorkspace(r.Context(), actor.UserID, input.TenantID)
	if err != nil || workspace.Role != RoleMerchantOwner {
		writeError(w, http.StatusForbidden, "WORKSPACE_FORBIDDEN", "account cannot access this store")
		return
	}
	s.writeAccessToken(w, r, workspaceIdentity(actor.UserID, actor.Username, actor.DisplayName, workspace))
}

func (s *Server) writeAccessToken(w http.ResponseWriter, r *http.Request, user identity) {
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		MembershipID: user.MembershipID, TenantID: user.TenantID, Role: user.Role, Username: user.Username, TokenKind: "access",
		RegisteredClaims: jwt.RegisteredClaims{Subject: int64String(user.UserID), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.Config.JWTTTL)), Issuer: "tanban-api"},
	}).SignedString([]byte(s.Config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not issue access token")
		return
	}
	s.audit(r.Context(), user, "auth.login", "account", int64String(user.UserID), map[string]any{"username": user.Username, "tenant_id": user.TenantID}, r)
	writeData(w, http.StatusOK, map[string]any{"access_token": token, "accessToken": token, "token_type": "Bearer", "tokenType": "Bearer", "expires_in": int64(s.Config.JWTTTL.Seconds()), "expiresIn": int64(s.Config.JWTTTL.Seconds()), "user": user})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
			return
		}
		parsed := &claims{}
		token, err := jwt.ParseWithClaims(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), parsed, func(token *jwt.Token) (any, error) {
			return []byte(s.Config.JWTSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("tanban-api"))
		if err != nil || !token.Valid || parsed.TokenKind == "tenant_selection" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
			return
		}
		userID, err := parseInt64(parsed.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token subject")
			return
		}
		var user identity
		if parsed.TenantID == 0 && strings.HasPrefix(parsed.Role, "PLATFORM_") {
			var status string
			err = s.DB.QueryRowContext(r.Context(), `SELECT id,username,display_name,COALESCE(platform_role,''),status
				FROM accounts WHERE id=? AND deleted_at IS NULL`, userID).
				Scan(&user.UserID, &user.Username, &user.DisplayName, &user.Role, &status)
			if err != nil || status != "ACTIVE" || user.Role != parsed.Role {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "account is unavailable")
				return
			}
		} else {
			username, displayName, accountErr := s.loadActiveAccount(r.Context(), userID)
			workspace, workspaceErr := s.loadMerchantWorkspace(r.Context(), userID, parsed.TenantID)
			if accountErr != nil || workspaceErr != nil || workspace.Role != parsed.Role || parsed.MembershipID > 0 && workspace.MembershipID != parsed.MembershipID {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "account is unavailable")
				return
			}
			user = workspaceIdentity(userID, username, displayName, workspace)
		}
		if user.Role != parsed.Role || user.TenantID != parsed.TenantID {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "account is unavailable")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, user)))
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, currentIdentity(r.Context()))
}

func requireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed[currentIdentity(r.Context()).Role] {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) BootstrapAdmin(ctx context.Context) error {
	if s.Config.BootstrapAdminUser == "" || s.Config.BootstrapAdminPass == "" {
		return nil
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts WHERE platform_role=? AND status='ACTIVE' AND deleted_at IS NULL", RolePlatformAdmin).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.Config.BootstrapAdminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO accounts(username,password_hash,display_name,status,platform_role)
		VALUES(?,?,?,'ACTIVE',?)`, s.Config.BootstrapAdminUser, string(hash), "Platform Administrator", RolePlatformAdmin)
	return err
}

func (s *Server) audit(ctx context.Context, actor identity, action, resourceType, resourceID string, details any, r *http.Request) {
	payload, _ := jsonMarshal(details)
	ip := ""
	if r != nil {
		ip = r.RemoteAddr
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO audit_logs(tenant_id,actor_user_id,action,resource_type,resource_id,request_id,ip,details_text)
		VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, actor.UserID, action, resourceType, resourceID, requestID(ctx), ip, string(payload))
	if err != nil {
		s.Logger.Error("write audit log", "error", err, "action", action)
	}
}

type identityKey struct{}

func currentIdentity(ctx context.Context) identity {
	value, _ := ctx.Value(identityKey{}).(identity)
	return value
}

func int64String(value int64) string { return strconvFormatInt(value) }

// Tiny wrappers keep formatting/parsing dependencies in one file and make
// auth middleware easy to unit-test.
func parseInt64(value string) (int64, error) { return strconvParseInt(value) }

var _ = sql.ErrNoRows
