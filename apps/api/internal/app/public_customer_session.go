package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const customerSessionTTL = 30 * 24 * time.Hour

type publicCustomerSessionInput struct {
	Code      string `json:"code"`
	GuestKey  string `json:"guestKey"`
	StoreCode string `json:"storeCode"`
}

type wechatCodeSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type publicCustomerSession struct {
	CustomerID int64
	TenantID   int64
	StoreID    int64
	OpenID     string
}

func (s *Server) publicCreateCustomerSession(w http.ResponseWriter, r *http.Request) {
	var input publicCustomerSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Code = strings.TrimSpace(input.Code)
	input.GuestKey = strings.TrimSpace(input.GuestKey)
	input.StoreCode = strings.TrimSpace(input.StoreCode)
	if input.Code == "" || len(input.Code) > 256 || len(input.GuestKey) < 12 || len(input.GuestKey) > 128 || input.StoreCode == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "valid code, guestKey and storeCode are required")
		return
	}
	if s.Config.WeChatMiniApp.AppID == "" || s.Config.WeChatMiniApp.AppSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "WECHAT_LOGIN_NOT_CONFIGURED", "WeChat mini-program login is not configured")
		return
	}
	store, err := s.findPublicStore(r.Context(), input.StoreCode)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rateKey := "wechat-login:" + publicClientHost(r)
	attempts := 0
	if raw, cacheErr := s.Cache.Get(r.Context(), rateKey); cacheErr == nil {
		attempts, _ = strconv.Atoi(string(raw))
	}
	if attempts >= 30 {
		writeError(w, http.StatusTooManyRequests, "WECHAT_LOGIN_RATE_LIMITED", "too many login attempts; retry in one minute")
		return
	}
	_ = s.Cache.Set(r.Context(), rateKey, []byte(strconv.Itoa(attempts+1)), time.Minute)
	codeSession, err := s.exchangeWechatLoginCode(r.Context(), input.Code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "WECHAT_LOGIN_FAILED", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	customerID, err := upsertVerifiedWechatCustomer(r.Context(), tx, store, input.GuestKey, codeSession.OpenID, codeSession.UnionID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = ensureAutoMembershipTx(r.Context(), tx, store.TenantID, customerID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		TenantID:  store.TenantID,
		Role:      "CUSTOMER",
		TokenKind: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: int64String(customerID), IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(customerSessionTTL)), Issuer: "tanban-api",
		},
	}).SignedString([]byte(s.Config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not issue customer token")
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"accessToken": token, "tokenType": "Bearer", "expiresIn": int64(customerSessionTTL.Seconds()),
		"customerId": customerID, "storeCode": store.Code,
	})
}

func (s *Server) exchangeWechatLoginCode(ctx context.Context, code string) (wechatCodeSession, error) {
	endpoint, err := url.Parse(strings.TrimRight(s.Config.WeChatMiniApp.APIBaseURL, "/") + "/sns/jscode2session")
	if err != nil {
		return wechatCodeSession{}, err
	}
	query := endpoint.Query()
	query.Set("appid", s.Config.WeChatMiniApp.AppID)
	query.Set("secret", s.Config.WeChatMiniApp.AppSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return wechatCodeSession{}, err
	}
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return wechatCodeSession{}, fmt.Errorf("微信登录服务暂时不可用")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return wechatCodeSession{}, fmt.Errorf("微信登录服务返回异常状态")
	}
	var result wechatCodeSession
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		return wechatCodeSession{}, fmt.Errorf("微信登录响应无法解析")
	}
	if result.ErrCode != 0 || strings.TrimSpace(result.OpenID) == "" {
		return wechatCodeSession{}, fmt.Errorf("微信登录凭证无效，请重新进入小程序")
	}
	return result, nil
}

func upsertVerifiedWechatCustomer(ctx context.Context, tx *sql.Tx, store storeDTO, guestKey, openID, unionID string) (int64, error) {
	var guestID int64
	var guestOpenID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id,wechat_openid FROM customers
		WHERE tenant_id=? AND guest_key=? AND deleted_at IS NULL FOR UPDATE`, store.TenantID, guestKey).Scan(&guestID, &guestOpenID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var openIDCustomerID int64
	openErr := tx.QueryRowContext(ctx, `SELECT id FROM customers
		WHERE tenant_id=? AND wechat_openid=? AND deleted_at IS NULL FOR UPDATE`, store.TenantID, openID).Scan(&openIDCustomerID)
	if openErr != nil && !errors.Is(openErr, sql.ErrNoRows) {
		return 0, openErr
	}
	customerID := guestID
	switch {
	case guestID > 0 && (openIDCustomerID == 0 || openIDCustomerID == guestID):
		if _, err = tx.ExecContext(ctx, `UPDATE customers SET wechat_openid=?,unionid=IF(?='',unionid,?),
			source='MINIPROGRAM',status='ACTIVE',deleted_at=NULL,last_seen_at=NOW(3)
			WHERE tenant_id=? AND id=?`, openID, unionID, unionID, store.TenantID, guestID); err != nil {
			return 0, err
		}
	case guestID > 0 && openIDCustomerID != guestID:
		// The verified OpenID is authoritative. Never grant a signed session for
		// a different guest row merely because the caller supplied its guest key.
		customerID = openIDCustomerID
		if _, err = tx.ExecContext(ctx, `UPDATE customers SET source='MINIPROGRAM',status='ACTIVE',
			deleted_at=NULL,last_seen_at=NOW(3) WHERE tenant_id=? AND id=?`, store.TenantID, customerID); err != nil {
			return 0, err
		}
	case openIDCustomerID > 0:
		customerID = openIDCustomerID
		if _, err = tx.ExecContext(ctx, `UPDATE customers SET guest_key=IF(guest_key IS NULL OR guest_key='',?,guest_key),
			unionid=IF(?='',unionid,?),source='MINIPROGRAM',status='ACTIVE',deleted_at=NULL,last_seen_at=NOW(3)
			WHERE tenant_id=? AND id=?`, guestKey, unionID, unionID, store.TenantID, customerID); err != nil {
			return 0, err
		}
	default:
		publicID := "CU" + strings.ToUpper(requestFingerprint(map[string]any{"tenantId": store.TenantID, "openid": openID})[:32])
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO customers(tenant_id,source_store_id,public_id,wechat_openid,guest_key,unionid,
			name,source,status,last_seen_at) VALUES(?,?,?,?,?,?,?,'MINIPROGRAM','ACTIVE',NOW(3))`,
			store.TenantID, store.ID, publicID, openID, guestKey, nullableCustomerString(unionID), "微信顾客")
		if insertErr != nil {
			return 0, insertErr
		}
		customerID, _ = result.LastInsertId()
	}
	if customerID <= 0 {
		return 0, sql.ErrNoRows
	}
	return customerID, nil
}

func nullableCustomerString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (s *Server) optionalPublicCustomerSession(ctx context.Context, r *http.Request, tenantID int64) (publicCustomerSession, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return publicCustomerSession{}, false
	}
	parsed := &claims{}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), parsed, func(token *jwt.Token) (any, error) {
		return []byte(s.Config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("tanban-api"))
	if err != nil || !token.Valid || parsed.TokenKind != "customer" || parsed.TenantID != tenantID {
		return publicCustomerSession{}, false
	}
	customerID, err := parseInt64(parsed.Subject)
	if err != nil || customerID <= 0 {
		return publicCustomerSession{}, false
	}
	var session publicCustomerSession
	session.CustomerID, session.TenantID = customerID, tenantID
	err = s.DB.QueryRowContext(ctx, `SELECT COALESCE(source_store_id,0),COALESCE(wechat_openid,'')
		FROM customers WHERE tenant_id=? AND id=? AND status='ACTIVE' AND deleted_at IS NULL`,
		tenantID, customerID).Scan(&session.StoreID, &session.OpenID)
	return session, err == nil
}

func (s *Server) requirePublicCustomerSession(w http.ResponseWriter, r *http.Request, tenantID int64) (publicCustomerSession, bool) {
	session, ok := s.optionalPublicCustomerSession(r.Context(), r, tenantID)
	if !ok {
		writeError(w, http.StatusUnauthorized, "CUSTOMER_SESSION_REQUIRED", "请重新进入小程序完成微信登录")
	}
	return session, ok
}
