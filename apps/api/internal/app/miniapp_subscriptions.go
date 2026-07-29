package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	miniAppNotificationPickupReady     = "PICKUP_READY"
	miniAppNotificationRechargeSuccess = "RECHARGE_SUCCESS"
	miniAppNotificationBalanceConsumed = "BALANCE_CONSUMED"

	miniAppNotificationBatchSize   = 20
	miniAppNotificationMaxAttempts = 6
)

type miniAppNotificationTemplate struct {
	Scene      string `json:"scene"`
	TemplateID string `json:"templateId"`
	Title      string `json:"title"`
}

type customerSubscriptionResultInput struct {
	Scene      string `json:"scene"`
	TemplateID string `json:"templateId"`
	Result     string `json:"result"`
}

type customerSubscriptionResultsInput struct {
	RequestID      string                            `json:"requestId"`
	RequestContext string                            `json:"requestContext"`
	BusinessNo     string                            `json:"businessNo"`
	Results        []customerSubscriptionResultInput `json:"results"`
}

type miniAppTemplateValue struct {
	Value string `json:"value"`
}

type miniAppNotificationEvent struct {
	TenantID, StoreID, CustomerID                 int64
	ChannelKey, AppID, OpenID                     string
	Scene, BusinessType                           string
	BusinessNo, AuthorizationBusinessNo, PagePath string
	Data                                          map[string]miniAppTemplateValue
}

type miniAppNotificationOutboxItem struct {
	ID, TenantID int64
	ChannelKey   string
	AppID        string
	OpenID       string
	TemplateID   string
	PagePath     string
	PayloadJSON  string
	Attempts     int
}

type wechatTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatSubscribeSendResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type wechatSubscribeSendError struct {
	Code int
	Msg  string
}

func (e wechatSubscribeSendError) Error() string {
	return fmt.Sprintf("wechat subscribe send failed (%d): %s", e.Code, e.Msg)
}

func validMiniAppNotificationScene(scene string) bool {
	return validStatus(scene, miniAppNotificationPickupReady, miniAppNotificationRechargeSuccess, miniAppNotificationBalanceConsumed)
}

func (s *Server) publicMiniAppNotificationTemplates(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	session, ok := s.requirePublicCustomerSession(w, r, store.TenantID)
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT scene,template_id,title
		FROM miniapp_notification_templates
		WHERE channel_key=? AND appid=? AND enabled=1
		ORDER BY FIELD(scene,'PICKUP_READY','RECHARGE_SUCCESS','BALANCE_CONSUMED'),id`,
		session.MiniAppChannelKey, session.MiniAppID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	templates := []miniAppNotificationTemplate{}
	for rows.Next() {
		var item miniAppNotificationTemplate
		if err = rows.Scan(&item.Scene, &item.TemplateID, &item.Title); err != nil {
			handleSQLError(w, err)
			return
		}
		templates = append(templates, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	var onboardingRequested bool
	if err = s.DB.QueryRowContext(r.Context(), `SELECT EXISTS(
		SELECT 1 FROM customer_subscription_results
		WHERE tenant_id=? AND customer_id=? AND channel_key=?
	)`, store.TenantID, session.CustomerID, session.MiniAppChannelKey).Scan(&onboardingRequested); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"available":           len(templates) > 0,
		"onboardingRequested": onboardingRequested,
		"templates":           templates,
	})
}

func (s *Server) publicRecordMiniAppSubscriptionResults(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	session, ok := s.requirePublicCustomerSession(w, r, store.TenantID)
	if !ok {
		return
	}
	var input customerSubscriptionResultsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.RequestContext = strings.ToUpper(strings.TrimSpace(input.RequestContext))
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.BusinessNo = strings.TrimSpace(input.BusinessNo)
	if len(input.Results) == 0 || len(input.Results) > 3 || input.RequestID == "" || len(input.RequestID) > 80 ||
		!validStatus(input.RequestContext, "ORDER", "RECHARGE") || len(input.BusinessNo) > 128 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "one to three subscription results and valid context are required")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	for _, item := range input.Results {
		item.Scene = strings.ToUpper(strings.TrimSpace(item.Scene))
		item.TemplateID = strings.TrimSpace(item.TemplateID)
		item.Result = strings.ToLower(strings.TrimSpace(item.Result))
		if !validMiniAppNotificationScene(item.Scene) || !validStatus(item.Result, "ACCEPT", "REJECT", "BAN", "FILTER") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "subscription scene or result is invalid")
			return
		}
		var configured bool
		if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(
			SELECT 1 FROM miniapp_notification_templates
			WHERE channel_key=? AND appid=? AND scene=? AND template_id=? AND enabled=1
		)`, session.MiniAppChannelKey, session.MiniAppID, item.Scene, item.TemplateID).Scan(&configured); err != nil {
			handleSQLError(w, err)
			return
		}
		if !configured {
			writeError(w, http.StatusBadRequest, "UNKNOWN_NOTIFICATION_TEMPLATE", "通知模板与当前小程序不匹配")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO customer_subscription_results(
			tenant_id,customer_id,channel_key,appid,scene,template_id,result,request_id,request_context,business_no
		) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE id=id`, store.TenantID, session.CustomerID, session.MiniAppChannelKey,
			session.MiniAppID, item.Scene, item.TemplateID, item.Result, input.RequestID, input.RequestContext, input.BusinessNo); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"recorded": len(input.Results)})
}

func enqueueMiniAppNotificationTx(ctx context.Context, tx *sql.Tx, event miniAppNotificationEvent) error {
	if event.TenantID <= 0 || event.StoreID <= 0 || event.CustomerID <= 0 || event.OpenID == "" ||
		event.ChannelKey == "" || event.AppID == "" || !validMiniAppNotificationScene(event.Scene) ||
		event.BusinessType == "" || event.BusinessNo == "" {
		return nil
	}
	var existing bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM miniapp_notification_outbox
		WHERE tenant_id=? AND channel_key=? AND scene=? AND business_type=? AND business_no=?
	)`, event.TenantID, event.ChannelKey, event.Scene, event.BusinessType, event.BusinessNo).Scan(&existing); err != nil {
		return err
	}
	if existing {
		return nil
	}
	var templateID, basePage string
	err := tx.QueryRowContext(ctx, `SELECT template_id,page_path FROM miniapp_notification_templates
		WHERE channel_key=? AND appid=? AND scene=? AND enabled=1`,
		event.ChannelKey, event.AppID, event.Scene).Scan(&templateID, &basePage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var subscriptionResultID int64
	authorizationBusinessNo := event.AuthorizationBusinessNo
	if authorizationBusinessNo == "" {
		authorizationBusinessNo = event.BusinessNo
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM customer_subscription_results
		WHERE tenant_id=? AND customer_id=? AND channel_key=? AND appid=? AND scene=? AND template_id=?
		  AND result='accept' AND claimed_at IS NULL
		ORDER BY (business_no=? AND business_no<>'') DESC,id DESC LIMIT 1 FOR UPDATE`,
		event.TenantID, event.CustomerID, event.ChannelKey, event.AppID, event.Scene, templateID, authorizationBusinessNo).
		Scan(&subscriptionResultID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	pagePath := event.PagePath
	if pagePath == "" {
		pagePath = basePage
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO miniapp_notification_outbox(
		tenant_id,store_id,customer_id,subscription_result_id,channel_key,appid,openid,scene,template_id,
		business_type,business_no,page_path,payload_json,status,available_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'PENDING',NOW(3))`,
		event.TenantID, event.StoreID, event.CustomerID, subscriptionResultID, event.ChannelKey, event.AppID,
		event.OpenID, event.Scene, templateID, event.BusinessType, event.BusinessNo, pagePath, string(payload))
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			return nil
		}
		return err
	}
	if inserted, _ := result.RowsAffected(); inserted != 1 {
		return nil
	}
	claimed, err := tx.ExecContext(ctx, `UPDATE customer_subscription_results SET claimed_at=NOW(3)
		WHERE id=? AND claimed_at IS NULL`, subscriptionResultID)
	if err != nil {
		return err
	}
	if affected, _ := claimed.RowsAffected(); affected != 1 {
		return errors.New("miniapp subscription result was claimed concurrently")
	}
	return nil
}

func (s *Server) enqueuePickupReadyNotificationTx(ctx context.Context, tx *sql.Tx, tenantID, orderID int64) error {
	var event miniAppNotificationEvent
	var orderNo, pickupCode, orderType, storeCode, storeName string
	err := tx.QueryRowContext(ctx, `SELECT o.tenant_id,o.store_id,COALESCE(o.customer_id,0),
		o.source_miniapp_channel_key,o.source_miniapp_appid,o.customer_openid,o.order_no,o.pickup_code,o.order_type,
		s.code,s.name
		FROM orders o JOIN stores s ON s.id=o.store_id AND s.tenant_id=o.tenant_id
		WHERE o.tenant_id=? AND o.id=?`, tenantID, orderID).
		Scan(&event.TenantID, &event.StoreID, &event.CustomerID, &event.ChannelKey, &event.AppID, &event.OpenID,
			&orderNo, &pickupCode, &orderType, &storeCode, &storeName)
	if err != nil {
		return err
	}
	if orderType != orderTypeTakeout {
		return nil
	}
	event.Scene = miniAppNotificationPickupReady
	event.BusinessType = "ORDER"
	event.BusinessNo = orderNo
	event.AuthorizationBusinessNo = orderNo
	event.PagePath = "pages/order-detail/index?orderNo=" + url.QueryEscape(orderNo) + "&storeCode=" + url.QueryEscape(storeCode)
	event.Data = map[string]miniAppTemplateValue{
		"character_string4": {Value: wechatTemplateText(pickupCode, 32)},
		"thing2":            {Value: wechatTemplateText(storeName, 20)},
		"character_string1": {Value: wechatTemplateText(orderNo, 32)},
		"phrase19":          {Value: "请取餐"},
		"thing11":           {Value: "餐品已制作完成，请及时到店取餐"},
	}
	return enqueueMiniAppNotificationTx(ctx, tx, event)
}

func (s *Server) enqueueRechargeSuccessNotificationTx(ctx context.Context, tx *sql.Tx, intent publicAccountPaymentIntent, paidAt time.Time) error {
	if intent.BusinessType != accountPaymentStoredValue {
		return nil
	}
	var storeCode, storeName, channelKey, appID string
	var balanceCents int64
	err := tx.QueryRowContext(ctx, `SELECT s.code,s.name,
		COALESCE(NULLIF(i.source_miniapp_channel_key,''),'tanban-public'),i.source_miniapp_appid,
		COALESCE(ba.principal_cents+ba.bonus_cents,0)
		FROM customer_account_payment_intents i
		JOIN stores s ON s.id=i.store_id AND s.tenant_id=i.tenant_id
		LEFT JOIN balance_accounts ba ON ba.tenant_id=i.tenant_id AND ba.customer_id=i.customer_id
		WHERE i.id=?`, intent.ID).Scan(&storeCode, &storeName, &channelKey, &appID, &balanceCents)
	if err != nil {
		return err
	}
	event := miniAppNotificationEvent{
		TenantID: intent.TenantID, StoreID: intent.StoreID, CustomerID: intent.CustomerID,
		ChannelKey: channelKey, AppID: appID, OpenID: intent.OpenID,
		Scene: miniAppNotificationRechargeSuccess, BusinessType: "ACCOUNT_PAYMENT",
		BusinessNo: int64String(intent.ID), AuthorizationBusinessNo: intent.IdempotencyKey,
		PagePath: "pages/recharge/index?storeCode=" + url.QueryEscape(storeCode),
		Data: map[string]miniAppTemplateValue{
			"amount4":  {Value: wechatTemplateAmount(intent.AmountCents)},
			"amount14": {Value: wechatTemplateAmount(intent.GiftCents)},
			"amount5":  {Value: wechatTemplateAmount(balanceCents)},
			"date6":    {Value: formatBeijingDateTime(paidAt)},
			"thing8":   {Value: wechatTemplateText(storeName, 20)},
		},
	}
	return enqueueMiniAppNotificationTx(ctx, tx, event)
}

func (s *Server) enqueueBalanceConsumedNotificationTx(ctx context.Context, tx *sql.Tx, tenantID, orderID, balancePaymentID, amountCents int64, paidAt time.Time) error {
	var event miniAppNotificationEvent
	var orderNo, storeCode, storeName string
	var balanceCents int64
	err := tx.QueryRowContext(ctx, `SELECT o.tenant_id,o.store_id,COALESCE(o.customer_id,0),
		o.source_miniapp_channel_key,o.source_miniapp_appid,o.customer_openid,o.order_no,s.code,s.name,
		COALESCE(ba.principal_cents+ba.bonus_cents,0)
		FROM orders o JOIN stores s ON s.id=o.store_id AND s.tenant_id=o.tenant_id
		LEFT JOIN balance_accounts ba ON ba.tenant_id=o.tenant_id AND ba.customer_id=o.customer_id
		WHERE o.tenant_id=? AND o.id=?`, tenantID, orderID).
		Scan(&event.TenantID, &event.StoreID, &event.CustomerID, &event.ChannelKey, &event.AppID, &event.OpenID,
			&orderNo, &storeCode, &storeName, &balanceCents)
	if err != nil {
		return err
	}
	event.Scene = miniAppNotificationBalanceConsumed
	event.BusinessType = "ORDER_BALANCE_PAYMENT"
	event.BusinessNo = int64String(balancePaymentID)
	event.AuthorizationBusinessNo = orderNo
	event.PagePath = "pages/order-detail/index?orderNo=" + url.QueryEscape(orderNo) + "&storeCode=" + url.QueryEscape(storeCode)
	event.Data = map[string]miniAppTemplateValue{
		"thing3":  {Value: wechatTemplateText(storeName, 20)},
		"amount4": {Value: wechatTemplateAmount(amountCents)},
		"amount1": {Value: wechatTemplateAmount(balanceCents)},
		"time6":   {Value: formatBeijingDateTime(paidAt)},
		"thing2":  {Value: "本次已使用储值余额支付"},
	}
	return enqueueMiniAppNotificationTx(ctx, tx, event)
}

func wechatTemplateText(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(value)
	if limit > 0 && len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

func wechatTemplateAmount(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	return fmt.Sprintf("%.2f元", float64(cents)/100)
}

func (s *Server) StartMiniAppNotificationWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processPendingMiniAppNotifications(ctx)
			}
		}
	}()
}

func (s *Server) processPendingMiniAppNotifications(ctx context.Context) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE miniapp_notification_outbox
		SET status='PENDING',last_error='worker interrupted; retrying'
		WHERE status='PROCESSING' AND updated_at<DATE_SUB(NOW(3),INTERVAL 2 MINUTE)`)
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM miniapp_notification_outbox
		WHERE status='PENDING' AND available_at<=NOW(3)
		ORDER BY available_at,id LIMIT ?`, miniAppNotificationBatchSize)
	if err != nil {
		s.Logger.Error("list miniapp notification outbox", "error", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	_ = rows.Close()
	for _, id := range ids {
		if err = s.dispatchMiniAppNotification(ctx, id); err != nil {
			s.Logger.Error("dispatch miniapp notification", "outbox_id", id, "error", err)
		}
	}
}

func (s *Server) dispatchMiniAppNotification(ctx context.Context, id int64) error {
	claimed, err := s.DB.ExecContext(ctx, `UPDATE miniapp_notification_outbox
		SET status='PROCESSING',attempts=attempts+1
		WHERE id=? AND status='PENDING' AND available_at<=NOW(3)`, id)
	if err != nil {
		return err
	}
	if count, _ := claimed.RowsAffected(); count != 1 {
		return nil
	}
	var item miniAppNotificationOutboxItem
	err = s.DB.QueryRowContext(ctx, `SELECT id,tenant_id,channel_key,appid,openid,template_id,page_path,payload_json,attempts
		FROM miniapp_notification_outbox WHERE id=? AND status='PROCESSING'`, id).
		Scan(&item.ID, &item.TenantID, &item.ChannelKey, &item.AppID, &item.OpenID, &item.TemplateID, &item.PagePath, &item.PayloadJSON, &item.Attempts)
	if err != nil {
		return err
	}
	credentials, err := s.miniAppCredentialsForChannel(ctx, item.TenantID, item.ChannelKey)
	if err == nil && credentials.AppID != item.AppID {
		err = errors.New("miniapp notification AppID no longer matches its channel")
	}
	var providerResponse string
	if err == nil {
		providerResponse, err = s.sendMiniAppSubscriptionMessage(ctx, credentials, item)
	}
	if err == nil {
		_, updateErr := s.DB.ExecContext(ctx, `UPDATE miniapp_notification_outbox
			SET status='DONE',provider_response=?,last_error='',processed_at=NOW(3)
			WHERE id=? AND status='PROCESSING'`, providerResponse, id)
		return updateErr
	}
	errorText := wechatTemplateText(err.Error(), 500)
	if miniAppNotificationPermanentError(err) || item.Attempts >= miniAppNotificationMaxAttempts {
		status := "DEAD"
		var sendErr wechatSubscribeSendError
		if errors.As(err, &sendErr) && sendErr.Code == 43101 {
			status = "SKIPPED"
		}
		_, updateErr := s.DB.ExecContext(ctx, `UPDATE miniapp_notification_outbox
			SET status=?,provider_response=?,last_error=?,processed_at=NOW(3)
			WHERE id=? AND status='PROCESSING'`, status, providerResponse, errorText, id)
		return updateErr
	}
	nextAttempt := time.Now().Add(miniAppNotificationBackoff(item.Attempts))
	_, updateErr := s.DB.ExecContext(ctx, `UPDATE miniapp_notification_outbox
		SET status='PENDING',available_at=?,provider_response=?,last_error=?
		WHERE id=? AND status='PROCESSING'`, nextAttempt, providerResponse, errorText, id)
	return updateErr
}

func (s *Server) miniAppCredentialsForChannel(ctx context.Context, tenantID int64, channelKey string) (miniAppCredentials, error) {
	if channelKey == publicMiniAppChannelKey {
		if s.Config.WeChatMiniApp.AppID == "" || s.Config.WeChatMiniApp.AppSecret == "" {
			return miniAppCredentials{}, errors.New("public mini-program credentials are not configured")
		}
		return miniAppCredentials{
			TenantID: tenantID, Mode: "PUBLIC", ChannelKey: channelKey,
			AppID: s.Config.WeChatMiniApp.AppID, AppSecret: s.Config.WeChatMiniApp.AppSecret,
		}, nil
	}
	var credentials miniAppCredentials
	var secretCipher string
	err := s.DB.QueryRowContext(ctx, `SELECT tenant_id,COALESCE(dedicated_channel_key,''),COALESCE(dedicated_appid,''),dedicated_app_secret_cipher
		FROM tenant_miniapp_channels
		WHERE tenant_id=? AND dedicated_channel_key=? AND dedicated_enabled=1`, tenantID, channelKey).
		Scan(&credentials.TenantID, &credentials.ChannelKey, &credentials.AppID, &secretCipher)
	if err != nil {
		return miniAppCredentials{}, err
	}
	credentials.Mode = "DEDICATED"
	credentials.AppSecret, err = s.decryptMiniAppSecret(secretCipher)
	return credentials, err
}

func (s *Server) sendMiniAppSubscriptionMessage(ctx context.Context, credentials miniAppCredentials, item miniAppNotificationOutboxItem) (string, error) {
	var data map[string]miniAppTemplateValue
	if err := json.Unmarshal([]byte(item.PayloadJSON), &data); err != nil {
		return "", err
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := s.miniAppAccessToken(ctx, credentials, attempt > 0)
		if err != nil {
			return "", err
		}
		endpoint := strings.TrimRight(s.Config.WeChatMiniApp.APIBaseURL, "/") + "/cgi-bin/message/subscribe/send?access_token=" + url.QueryEscape(token)
		body, _ := json.Marshal(map[string]any{
			"touser": item.OpenID, "template_id": item.TemplateID, "page": item.PagePath,
			"lang": "zh_CN", "data": data,
		})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := s.HTTPClient.Do(request)
		if err != nil {
			return "", err
		}
		var result wechatSubscribeSendResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&result)
		_ = response.Body.Close()
		raw, _ := json.Marshal(result)
		if decodeErr != nil {
			return string(raw), decodeErr
		}
		if result.ErrCode == 0 {
			return string(raw), nil
		}
		if attempt == 0 && validStatus(fmt.Sprint(result.ErrCode), "40001", "40014", "42001") {
			_ = s.Cache.Delete(ctx, miniAppTokenCacheKey(credentials.AppID))
			continue
		}
		return string(raw), wechatSubscribeSendError{Code: result.ErrCode, Msg: result.ErrMsg}
	}
	return "", errors.New("wechat access token refresh did not recover")
}

func (s *Server) miniAppAccessToken(ctx context.Context, credentials miniAppCredentials, forceRefresh bool) (string, error) {
	cacheKey := miniAppTokenCacheKey(credentials.AppID)
	if !forceRefresh {
		if cached, err := s.Cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
			return string(cached), nil
		}
	}
	endpoint := strings.TrimRight(s.Config.WeChatMiniApp.APIBaseURL, "/") + "/cgi-bin/stable_token"
	body, _ := json.Marshal(map[string]any{
		"grant_type": "client_credential", "appid": credentials.AppID,
		"secret": credentials.AppSecret, "force_refresh": forceRefresh,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var result wechatTokenResponse
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 || result.AccessToken == "" {
		return "", fmt.Errorf("wechat access token failed (%d): %s", result.ErrCode, result.ErrMsg)
	}
	ttl := time.Duration(result.ExpiresIn-300) * time.Second
	if ttl < time.Minute {
		ttl = time.Minute
	}
	_ = s.Cache.Set(ctx, cacheKey, []byte(result.AccessToken), ttl)
	return result.AccessToken, nil
}

func miniAppTokenCacheKey(appID string) string {
	return "wechat-miniapp-access-token:" + appID
}

func miniAppNotificationPermanentError(err error) bool {
	var sendErr wechatSubscribeSendError
	if !errors.As(err, &sendErr) {
		return false
	}
	switch sendErr.Code {
	case 40003, 40037, 41030, 43101, 47003:
		return true
	default:
		return false
	}
}

func miniAppNotificationBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<attempt) * time.Second
}
