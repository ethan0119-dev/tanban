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
	UserID      int64  `json:"user_id"`
	TenantID    int64  `json:"tenant_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

type claims struct {
	TenantID int64  `json:"tenant_id"`
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Portal   string `json:"portal"`
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
	if err = tx.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, actor.UserID, actor.TenantID).Scan(&currentHash); err != nil {
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
	result, err := tx.ExecContext(r.Context(), `UPDATE users SET password_hash=?,updated_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, string(nextHash), actor.UserID, actor.TenantID)
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
	rateKey := "login:" + r.RemoteAddr + ":" + strings.ToLower(input.Username)
	if raw, err := s.Cache.Get(r.Context(), rateKey); err == nil {
		attempts, _ := strconv.Atoi(string(raw))
		if attempts >= 5 {
			writeError(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "too many failed attempts; retry in five minutes")
			return
		}
	}
	var user identity
	var passwordHash, status string
	err := s.DB.QueryRowContext(r.Context(), `SELECT u.id,u.tenant_id,u.username,u.display_name,u.role,u.password_hash,u.status
		FROM users u LEFT JOIN tenants t ON t.id=u.tenant_id
		WHERE u.username=? AND u.deleted_at IS NULL
		AND (u.tenant_id=0 OR (t.status='ACTIVE' AND t.deleted_at IS NULL))`, input.Username).
		Scan(&user.UserID, &user.TenantID, &user.Username, &user.DisplayName, &user.Role, &passwordHash, &status)
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
	if input.Portal == "platform" && !strings.HasPrefix(user.Role, "PLATFORM_") || input.Portal == "merchant" && !strings.HasPrefix(user.Role, "MERCHANT_") {
		writeError(w, http.StatusForbidden, "PORTAL_FORBIDDEN", "account cannot sign in to this portal")
		return
	}
	if input.Portal != "" && input.Portal != "platform" && input.Portal != "merchant" {
		writeError(w, http.StatusBadRequest, "INVALID_PORTAL", "portal must be platform or merchant")
		return
	}
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		TenantID: user.TenantID, Role: user.Role, Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{Subject: int64String(user.UserID), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.Config.JWTTTL)), Issuer: "tanban-api"},
	}).SignedString([]byte(s.Config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not issue access token")
		return
	}
	s.audit(r.Context(), user, "auth.login", "user", int64String(user.UserID), map[string]any{"username": user.Username}, r)
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
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
			return
		}
		userID, err := parseInt64(parsed.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token subject")
			return
		}
		var user identity
		var status string
		err = s.DB.QueryRowContext(r.Context(), `SELECT u.id,u.tenant_id,u.username,u.display_name,u.role,u.status
			FROM users u LEFT JOIN tenants t ON t.id=u.tenant_id
			WHERE u.id=? AND u.deleted_at IS NULL
			AND (u.tenant_id=0 OR (t.status='ACTIVE' AND t.deleted_at IS NULL))`, userID).
			Scan(&user.UserID, &user.TenantID, &user.Username, &user.DisplayName, &user.Role, &status)
		if err != nil || status != "ACTIVE" || user.TenantID != parsed.TenantID || user.Role != parsed.Role {
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
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE tenant_id=0 AND role=? AND status='ACTIVE' AND deleted_at IS NULL", RolePlatformAdmin).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.Config.BootstrapAdminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO users(tenant_id,username,password_hash,display_name,role,status)
		VALUES(0,?,?,?,?, 'ACTIVE')`, s.Config.BootstrapAdminUser, string(hash), "Platform Administrator", RolePlatformAdmin)
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
