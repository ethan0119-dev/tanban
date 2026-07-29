package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
	"github.com/go-chi/chi/v5"
)

const (
	accountPaymentMembership  = "MEMBER_LEVEL"
	accountPaymentStoredValue = "STORED_VALUE"
)

type publicAccountPaymentInput struct {
	LevelID int64 `json:"levelId"`
	RuleID  int64 `json:"ruleId"`
}

type publicAccountPaymentIntent struct {
	ID, TenantID, StoreID, CustomerID, BusinessID, AmountCents, GiftCents int64
	BusinessType, SnapshotJSON, Provider, MerchantNo, SubAppID            string
	OpenID, ProviderRequestNo, ProviderOrderNo, Status, RawResponse       string
	SourceMiniAppChannelKey, SourceMiniAppID                              string
	IdempotencyKey, RequestFingerprint                                    string
	FulfilledAt                                                           sql.NullTime
}

type publicMembershipPaymentSnapshot struct {
	Name         string `json:"name"`
	AcquireType  string `json:"acquireType"`
	ValidDays    int    `json:"validDays"`
	BenefitsJSON string `json:"benefitsJson"`
}

func (s *Server) publicCreateMembershipOrder(w http.ResponseWriter, r *http.Request) {
	s.publicCreateAccountPayment(w, r, accountPaymentMembership)
}

func (s *Server) publicCreateStoredValueOrder(w http.ResponseWriter, r *http.Request) {
	s.publicCreateAccountPayment(w, r, accountPaymentStoredValue)
}

func (s *Server) publicCreateAccountPayment(w http.ResponseWriter, r *http.Request, businessType string) {
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input publicAccountPaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	businessID := input.LevelID
	if businessType == accountPaymentStoredValue {
		businessID = input.RuleID
	}
	if businessID <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "a membership level or stored-value rule is required")
		return
	}
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	session, ok := s.requirePublicCustomerSession(w, r, store.TenantID)
	if !ok {
		return
	}
	customerID := session.CustomerID
	openID := session.OpenID
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	fingerprint := requestFingerprint(map[string]any{"customerId": customerID, "businessType": businessType, "businessId": businessID})
	var existing publicAccountPaymentIntent
	err = scanPublicAccountPayment(tx.QueryRowContext(r.Context(), publicAccountPaymentSelect+` WHERE tenant_id=? AND idempotency_key=?`, store.TenantID, key), &existing)
	if err == nil {
		if existing.CustomerID != customerID || existing.RequestFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different account payment")
			return
		}
		if err = tx.Commit(); err != nil {
			handleSQLError(w, err)
			return
		}
		s.writePublicAccountPayment(w, http.StatusOK, existing)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}

	var amountCents, giftCents int64
	var snapshotJSON string
	switch businessType {
	case accountPaymentMembership:
		var enabled bool
		if err = tx.QueryRowContext(r.Context(), "SELECT enabled FROM membership_settings WHERE tenant_id=?", store.TenantID).Scan(&enabled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusConflict, "MEMBERSHIP_NOT_AVAILABLE", "membership is not enabled for this store")
				return
			}
			handleSQLError(w, err)
			return
		}
		if !enabled {
			writeError(w, http.StatusConflict, "MEMBERSHIP_NOT_AVAILABLE", "membership is not enabled for this store")
			return
		}
		var snapshot publicMembershipPaymentSnapshot
		if err = tx.QueryRowContext(r.Context(), `SELECT name,acquire_type,price_cents,valid_days,benefits_json
			FROM member_levels WHERE tenant_id=? AND id=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`,
			store.TenantID, businessID).Scan(&snapshot.Name, &snapshot.AcquireType, &amountCents, &snapshot.ValidDays, &snapshot.BenefitsJSON); err != nil {
			handleSQLError(w, err)
			return
		}
		if snapshot.AcquireType == "GROWTH" {
			writeError(w, http.StatusConflict, "MEMBERSHIP_LEVEL_NOT_PURCHASABLE", "growth membership levels cannot be purchased directly")
			return
		}
		if (snapshot.AcquireType == "PAID" && amountCents <= 0) || (snapshot.AcquireType == "FREE" && amountCents != 0) {
			writeError(w, http.StatusConflict, "MEMBERSHIP_LEVEL_INVALID", "membership level payment configuration is invalid")
			return
		}
		body, _ := json.Marshal(snapshot)
		snapshotJSON = string(body)
		if amountCents > 0 {
			maxBalance := int64(1000000)
			if settingsErr := tx.QueryRowContext(r.Context(), "SELECT max_balance_cents FROM stored_value_settings WHERE tenant_id=?",
				store.TenantID).Scan(&maxBalance); settingsErr != nil && !errors.Is(settingsErr, sql.ErrNoRows) {
				handleSQLError(w, settingsErr)
				return
			}
			var currentBalance, pendingBalance int64
			if balanceErr := tx.QueryRowContext(r.Context(), `SELECT COALESCE(principal_cents+bonus_cents,0)
				FROM balance_accounts WHERE tenant_id=? AND customer_id=?`, store.TenantID, customerID).Scan(&currentBalance); errors.Is(balanceErr, sql.ErrNoRows) {
				currentBalance = 0
			} else if balanceErr != nil {
				handleSQLError(w, balanceErr)
				return
			}
			if pendingErr := tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount_cents+gift_cents),0)
				FROM customer_account_payment_intents WHERE tenant_id=? AND customer_id=?
				AND (status IN ('CREATING','PENDING') OR (status='SUCCESS' AND fulfilled_at IS NULL))`,
				store.TenantID, customerID).Scan(&pendingBalance); pendingErr != nil {
				handleSQLError(w, pendingErr)
				return
			}
			if maxBalance < 0 || !nonNegativeSumWithin(maxBalance, currentBalance, pendingBalance, amountCents) {
				writeError(w, http.StatusConflict, "BALANCE_LIMIT_EXCEEDED", "the resulting customer balance would exceed the configured limit")
				return
			}
		}
	case accountPaymentStoredValue:
		var enabled, showInMiniapp bool
		var minRecharge, maxRecharge, maxBalance int64
		if err = tx.QueryRowContext(r.Context(), `SELECT enabled,show_in_miniapp,min_recharge_cents,max_recharge_cents,max_balance_cents
			FROM stored_value_settings WHERE tenant_id=? FOR UPDATE`, store.TenantID).
			Scan(&enabled, &showInMiniapp, &minRecharge, &maxRecharge, &maxBalance); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusConflict, "STORED_VALUE_NOT_AVAILABLE", "stored value is not enabled for this store")
				return
			}
			handleSQLError(w, err)
			return
		}
		if !enabled || !showInMiniapp {
			writeError(w, http.StatusConflict, "STORED_VALUE_NOT_AVAILABLE", "stored value is not enabled for this store")
			return
		}
		var name string
		var perCustomerLimit int
		if err = tx.QueryRowContext(r.Context(), `SELECT name,recharge_cents,gift_cents,per_customer_limit
			FROM stored_value_rules WHERE tenant_id=? AND id=? AND status='ACTIVE' AND deleted_at IS NULL
			AND (starts_at IS NULL OR starts_at<=NOW(3)) AND (ends_at IS NULL OR ends_at>=NOW(3)) FOR UPDATE`,
			store.TenantID, businessID).Scan(&name, &amountCents, &giftCents, &perCustomerLimit); err != nil {
			handleSQLError(w, err)
			return
		}
		if amountCents < minRecharge || amountCents > maxRecharge {
			writeError(w, http.StatusConflict, "RECHARGE_AMOUNT_OUT_OF_RANGE", "the stored-value rule is outside the configured recharge range")
			return
		}
		if perCustomerLimit > 0 {
			var used int
			if err = tx.QueryRowContext(r.Context(), `SELECT
				(SELECT COUNT(*) FROM stored_value_records WHERE tenant_id=? AND customer_id=? AND rule_id=? AND status='CONFIRMED')+
				(SELECT COUNT(*) FROM customer_account_payment_intents WHERE tenant_id=? AND customer_id=? AND business_type='STORED_VALUE'
				 AND business_id=? AND (status IN ('CREATING','PENDING') OR (status='SUCCESS' AND fulfilled_at IS NULL)))`,
				store.TenantID, customerID, businessID, store.TenantID, customerID, businessID).Scan(&used); err != nil {
				handleSQLError(w, err)
				return
			}
			if used >= perCustomerLimit {
				writeError(w, http.StatusConflict, "RULE_LIMIT_REACHED", "the customer has reached this stored-value rule limit")
				return
			}
		}
		var currentBalance, pendingBalance int64
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(principal_cents+bonus_cents,0)
			FROM balance_accounts WHERE tenant_id=? AND customer_id=?`, store.TenantID, customerID).Scan(&currentBalance); errors.Is(err, sql.ErrNoRows) {
			currentBalance = 0
		} else if err != nil {
			handleSQLError(w, err)
			return
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount_cents+gift_cents),0)
			FROM customer_account_payment_intents WHERE tenant_id=? AND customer_id=?
			AND (status IN ('CREATING','PENDING') OR (status='SUCCESS' AND fulfilled_at IS NULL))`,
			store.TenantID, customerID).Scan(&pendingBalance); err != nil {
			handleSQLError(w, err)
			return
		}
		if amountCents <= 0 || giftCents < 0 || maxBalance < 0 || !nonNegativeSumWithin(maxBalance, currentBalance, pendingBalance, amountCents, giftCents) {
			writeError(w, http.StatusConflict, "BALANCE_LIMIT_EXCEEDED", "the resulting customer balance would exceed the configured limit")
			return
		}
		body, _ := json.Marshal(map[string]any{"id": businessID, "name": name, "recharge_cents": amountCents, "gift_cents": giftCents})
		snapshotJSON = string(body)
	default:
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "unsupported account payment type")
		return
	}

	var paymentProvider, merchantNo, subAppID, onboardingStatus, productAuthorizationStatus string
	if err = tx.QueryRowContext(r.Context(), `SELECT payment_provider,payment_merchant_no,payment_sub_appid,
		payment_onboarding_status,payment_product_authorization_status FROM tenants WHERE id=?`,
		store.TenantID).Scan(&paymentProvider, &merchantNo, &subAppID, &onboardingStatus, &productAuthorizationStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if paymentProvider == "wechat_partner" {
		if session.MiniAppChannelKey == publicMiniAppChannelKey {
			subAppID = ""
		} else if session.MiniAppID != "" {
			if subAppID != "" && subAppID != session.MiniAppID {
				writeError(w, http.StatusConflict, "PAYMENT_APPID_MISMATCH", "当前小程序与商户支付 AppID 不一致")
				return
			}
			subAppID = session.MiniAppID
		}
	}
	if amountCents > 0 {
		enabled, enabledErr := s.paymentAcceptanceEnabled(r.Context())
		if enabledErr != nil {
			handleSQLError(w, enabledErr)
			return
		}
		if !enabled {
			writeError(w, http.StatusServiceUnavailable, "PAYMENTS_DISABLED", "payment acceptance is disabled by the platform")
			return
		}
		if paymentProvider != s.Payment.Name() {
			writeError(w, http.StatusServiceUnavailable, "PAYMENT_PROVIDER_UNAVAILABLE", "merchant payment provider is not active on the platform")
			return
		}
		if paymentProvider == "wechat_partner" && (merchantNo == "" || onboardingStatus != "ACTIVE" || productAuthorizationStatus != "AUTHORIZED") {
			writeError(w, http.StatusConflict, "WECHAT_PAY_MERCHANT_NOT_READY", "WeChat Pay sub-merchant onboarding or product authorization is incomplete")
			return
		}
	} else {
		paymentProvider = "free"
	}
	requestNo := newBusinessNo("AP")
	localProviderNo := localPaymentReference(requestNo)
	status := paymentStatusCreating
	if amountCents == 0 {
		status = string(provider.PaymentSuccess)
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO customer_account_payment_intents(
		tenant_id,store_id,customer_id,business_type,business_id,business_snapshot_json,amount_cents,gift_cents,
		provider,merchant_no,sub_appid,customer_openid,source_miniapp_channel_key,source_miniapp_appid,
		provider_request_no,provider_order_no,status,raw_response,idempotency_key,request_fingerprint,paid_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,IF(?='SUCCESS',NOW(3),NULL))`,
		store.TenantID, store.ID, customerID, businessType, businessID, snapshotJSON, amountCents, giftCents,
		paymentProvider, merchantNo, subAppID, openID, session.MiniAppChannelKey, session.MiniAppID,
		requestNo, localProviderNo, status, "{}", key, fingerprint, status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	intentID, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	if amountCents == 0 {
		if err = s.fulfillPublicAccountPayment(r.Context(), intentID, paymentProvider, localProviderNo, time.Now()); err != nil {
			handleSQLError(w, err)
			return
		}
		intent, loadErr := s.loadPublicAccountPayment(r.Context(), intentID)
		if loadErr != nil {
			handleSQLError(w, loadErr)
			return
		}
		s.writePublicAccountPayment(w, http.StatusCreated, intent)
		return
	}

	createResult, createErr := s.Payment.Create(r.Context(), provider.CreatePaymentRequest{
		MerchantNo: merchantNo,
		OrderNo:    requestNo,
		Amount:     amountCents,
		OpenID:     openID,
		SubAppID:   subAppID,
		NotifyURL:  s.paymentNotifyURL(),
	})
	if createErr != nil {
		raw, _ := json.Marshal(map[string]string{"phase": "creating", "last_error": truncateError(createErr)})
		nextStatus := paymentStatusCreating
		if errors.Is(createErr, provider.ErrNotConfigured) {
			nextStatus = string(provider.PaymentFailed)
		}
		_, _ = s.DB.ExecContext(r.Context(), `UPDATE customer_account_payment_intents SET status=?,raw_response=?
			WHERE id=? AND status='CREATING'`, nextStatus, string(raw), intentID)
		if errors.Is(createErr, provider.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "PAYMENT_PROVIDER_NOT_CONFIGURED", "the selected payment provider is not ready for transactions")
			return
		}
		writeError(w, http.StatusBadGateway, "PAYMENT_CREATE_FAILED", "payment creation is awaiting reconciliation")
		return
	}
	if strings.TrimSpace(createResult.ProviderOrderNo) == "" {
		writeError(w, http.StatusBadGateway, "PAYMENT_CREATE_FAILED", "payment provider returned an empty payment number")
		return
	}
	raw, _ := json.Marshal(createResult.PayParams)
	if _, err = s.DB.ExecContext(r.Context(), `UPDATE customer_account_payment_intents
		SET provider_order_no=?,status=?,raw_response=?,paid_at=IF(?='SUCCESS',NOW(3),NULL)
		WHERE id=? AND status='CREATING'`, createResult.ProviderOrderNo, string(createResult.Status), string(raw), string(createResult.Status), intentID); err != nil {
		handleSQLError(w, err)
		return
	}
	if createResult.Status == provider.PaymentSuccess {
		if err = s.fulfillPublicAccountPayment(r.Context(), intentID, paymentProvider, createResult.ProviderOrderNo, time.Now()); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	intent, err := s.loadPublicAccountPayment(r.Context(), intentID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.writePublicAccountPayment(w, http.StatusCreated, intent)
}

const publicAccountPaymentSelect = `SELECT id,tenant_id,store_id,customer_id,business_type,business_id,business_snapshot_json,
	amount_cents,gift_cents,provider,merchant_no,sub_appid,customer_openid,provider_request_no,provider_order_no,status,
	raw_response,idempotency_key,request_fingerprint,fulfilled_at,source_miniapp_channel_key,source_miniapp_appid
	FROM customer_account_payment_intents`

func scanPublicAccountPayment(row *sql.Row, intent *publicAccountPaymentIntent) error {
	return row.Scan(&intent.ID, &intent.TenantID, &intent.StoreID, &intent.CustomerID, &intent.BusinessType, &intent.BusinessID,
		&intent.SnapshotJSON, &intent.AmountCents, &intent.GiftCents, &intent.Provider, &intent.MerchantNo, &intent.SubAppID,
		&intent.OpenID, &intent.ProviderRequestNo, &intent.ProviderOrderNo, &intent.Status, &intent.RawResponse,
		&intent.IdempotencyKey, &intent.RequestFingerprint, &intent.FulfilledAt, &intent.SourceMiniAppChannelKey, &intent.SourceMiniAppID)
}

func (s *Server) loadPublicAccountPayment(ctx context.Context, id int64) (publicAccountPaymentIntent, error) {
	var intent publicAccountPaymentIntent
	err := scanPublicAccountPayment(s.DB.QueryRowContext(ctx, publicAccountPaymentSelect+" WHERE id=?", id), &intent)
	return intent, err
}

func (s *Server) writePublicAccountPayment(w http.ResponseWriter, statusCode int, intent publicAccountPaymentIntent) {
	params := map[string]string{}
	_ = json.Unmarshal([]byte(intent.RawResponse), &params)
	writeData(w, statusCode, map[string]any{
		"id": intent.ID, "paymentId": intent.ID, "provider": intent.Provider, "status": intent.Status,
		"businessType": intent.BusinessType, "amountCents": intent.AmountCents, "giftCents": intent.GiftCents,
		"providerOrderNo": intent.ProviderOrderNo, "wxPayParams": params, "fulfilled": intent.FulfilledAt.Valid,
	})
}

func (s *Server) publicGetAccountPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "paymentID")
	if !ok {
		return
	}
	intent, err := s.loadPublicAccountPayment(r.Context(), id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if !s.publicAccountPaymentOwnedByRequest(r.Context(), r, intent) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "account payment was not found")
		return
	}
	s.writePublicAccountPayment(w, http.StatusOK, intent)
}

func (s *Server) publicAccountPaymentMockConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.AllowMockConfirmation || s.Payment.Name() != "mock" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "mock confirmation endpoint is disabled")
		return
	}
	id, ok := pathID(w, r, "paymentID")
	if !ok {
		return
	}
	intent, err := s.loadPublicAccountPayment(r.Context(), id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if !s.publicAccountPaymentOwnedByRequest(r.Context(), r, intent) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "account payment was not found")
		return
	}
	if intent.Provider != "mock" {
		writeError(w, http.StatusConflict, "PAYMENT_PROVIDER_MISMATCH", "account payment is not a mock payment")
		return
	}
	if intent.Status != string(provider.PaymentSuccess) && !s.MockPayment.Confirm(intent.ProviderOrderNo) {
		writeError(w, http.StatusConflict, "MOCK_PAYMENT_NOT_PENDING", "mock payment is missing, closed, or already confirmed")
		return
	}
	if err = s.fulfillPublicAccountPayment(r.Context(), id, intent.Provider, intent.ProviderOrderNo, time.Now()); err != nil {
		handleSQLError(w, err)
		return
	}
	intent, err = s.loadPublicAccountPayment(r.Context(), id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.writePublicAccountPayment(w, http.StatusOK, intent)
}

func (s *Server) publicAccountPaymentOwnedByRequest(ctx context.Context, r *http.Request, intent publicAccountPaymentIntent) bool {
	session, ok := s.optionalPublicCustomerSession(ctx, r, intent.TenantID)
	return ok && session.CustomerID == intent.CustomerID
}

func (s *Server) fulfillPublicAccountPayment(ctx context.Context, id int64, providerName, providerNo string, paidAt time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var intent publicAccountPaymentIntent
	err = scanPublicAccountPayment(tx.QueryRowContext(ctx, publicAccountPaymentSelect+" WHERE id=? FOR UPDATE", id), &intent)
	if err != nil {
		return err
	}
	if intent.Provider != providerName || intent.ProviderOrderNo != providerNo {
		return errors.New("account payment identity mismatch")
	}
	if intent.FulfilledAt.Valid {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE customer_account_payment_intents SET status='SUCCESS',paid_at=?,updated_at=NOW(3) WHERE id=?`,
		paidAt, id); err != nil {
		return err
	}
	switch intent.BusinessType {
	case accountPaymentMembership:
		var snapshot publicMembershipPaymentSnapshot
		if err = json.Unmarshal([]byte(intent.SnapshotJSON), &snapshot); err != nil {
			return err
		}
		var memberID sql.NullInt64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM members WHERE tenant_id=? AND customer_id=? FOR UPDATE",
			intent.TenantID, intent.CustomerID).Scan(&memberID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var expires any
		if snapshot.ValidDays > 0 {
			expires = paidAt.AddDate(0, 0, snapshot.ValidDays)
		}
		if memberID.Valid {
			if _, err = tx.ExecContext(ctx, `UPDATE members SET current_level_id=?,status='ACTIVE',expires_at=?
				WHERE id=? AND tenant_id=?`, intent.BusinessID, expires, memberID.Int64, intent.TenantID); err != nil {
				return err
			}
		} else {
			result, insertErr := tx.ExecContext(ctx, `INSERT INTO members(tenant_id,customer_id,member_no,current_level_id,status,expires_at)
				VALUES(?,?,?,?,'ACTIVE',?)`, intent.TenantID, intent.CustomerID, newBusinessNo("MB"), intent.BusinessID, expires)
			if insertErr != nil {
				return insertErr
			}
			memberID.Int64, _ = result.LastInsertId()
			memberID.Valid = true
		}
		orderNo := newBusinessNo("ML")
		levelOrderKey := "PUBLICML:" + int64String(intent.ID)
		levelOrderFingerprint := requestFingerprint(map[string]any{"accountPaymentId": intent.ID, "businessType": intent.BusinessType})
		if _, err = tx.ExecContext(ctx, `INSERT INTO member_level_orders(tenant_id,order_no,customer_id,member_id,level_id,
			level_snapshot_json,amount_cents,payment_method,payment_status,status,remark,idempotency_key,request_fingerprint,created_by,completed_at)
			VALUES(?,?,?,?,?,?,?,?,?,'COMPLETED','小程序会员开通',?,?,0,?)`,
			intent.TenantID, orderNo, intent.CustomerID, memberID.Int64, intent.BusinessID, intent.SnapshotJSON,
			intent.AmountCents, intent.Provider, "SUCCESS", levelOrderKey, levelOrderFingerprint, paidAt); err != nil {
			return err
		}
		if intent.AmountCents > 0 {
			if err = applyPaidBalanceCreditTx(ctx, tx, intent.TenantID, intent.CustomerID, "PRINCIPAL", intent.AmountCents,
				"MEMBER_LEVEL_RECHARGE", orderNo, levelOrderKey+":principal", "会员等级充值："+snapshot.Name); err != nil {
				return err
			}
		}
	case accountPaymentStoredValue:
		recordNo := newBusinessNo("SV")
		recordKey := "PUBLICSV:" + int64String(intent.ID)
		recordFingerprint := requestFingerprint(map[string]any{"accountPaymentId": intent.ID, "businessType": intent.BusinessType})
		if _, err = tx.ExecContext(ctx, `INSERT INTO stored_value_records(tenant_id,record_no,customer_id,rule_id,
			rule_snapshot_json,principal_cents,gift_cents,payment_method,status,idempotency_key,request_fingerprint,created_by,remark)
			VALUES(?,?,?,?,?,?,?,?, 'CONFIRMED',?,?,0,'小程序储值充值')`,
			intent.TenantID, recordNo, intent.CustomerID, intent.BusinessID, intent.SnapshotJSON, intent.AmountCents,
			intent.GiftCents, intent.Provider, recordKey, recordFingerprint); err != nil {
			return err
		}
		if err = applyPaidBalanceCreditTx(ctx, tx, intent.TenantID, intent.CustomerID, "PRINCIPAL", intent.AmountCents,
			"STORED_VALUE", recordNo, recordKey+":principal", "小程序储值充值"); err != nil {
			return err
		}
		if intent.GiftCents > 0 {
			if err = applyPaidBalanceCreditTx(ctx, tx, intent.TenantID, intent.CustomerID, "BONUS", intent.GiftCents,
				"STORED_VALUE", recordNo, recordKey+":bonus", "小程序储值赠送"); err != nil {
				return err
			}
		}
		if err = s.enqueueRechargeSuccessNotificationTx(ctx, tx, intent, paidAt); err != nil {
			return err
		}
	default:
		return errors.New("unsupported account payment business type")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE customer_account_payment_intents SET fulfilled_at=?,updated_at=NOW(3) WHERE id=?", paidAt, id); err != nil {
		return err
	}
	return tx.Commit()
}

func applyPaidBalanceCreditTx(ctx context.Context, tx *sql.Tx, tenantID, customerID int64, bucket string, amount int64,
	businessType, businessNo, idempotencyKey, remark string) error {
	if amount <= 0 || !validStatus(bucket, "PRINCIPAL", "BONUS") {
		return errors.New("invalid paid balance credit")
	}
	if existing, found, err := loadBalanceLedgerByKey(ctx, tx, tenantID, idempotencyKey); err != nil {
		return err
	} else if found {
		_, _, compareErr := compareBalanceReplay(existing, customerID, bucket, amount, "RECHARGE", businessType, remark)
		return compareErr
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO balance_accounts(tenant_id,customer_id) VALUES(?,?)
		ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)`, tenantID, customerID); err != nil {
		return err
	}
	var principal, bonus int64
	if err := tx.QueryRowContext(ctx, `SELECT principal_cents,bonus_cents FROM balance_accounts
		WHERE tenant_id=? AND customer_id=? FOR UPDATE`, tenantID, customerID).Scan(&principal, &bonus); err != nil {
		return err
	}
	before := principal
	column := "principal_cents"
	if bucket == "BONUS" {
		before = bonus
		column = "bonus_cents"
	}
	if amount > maxBusinessAmountCents-before {
		return errBalanceLimitExceeded
	}
	after := before + amount
	if _, err := tx.ExecContext(ctx, `INSERT INTO balance_ledger(tenant_id,customer_id,account_bucket,delta_cents,
		balance_before_cents,balance_after_cents,entry_type,business_type,business_no,idempotency_key,operator_user_id,remark)
		VALUES(?,?,?,?,?,?,?,?,?,?,0,?)`, tenantID, customerID, bucket, amount, before, after, "RECHARGE", businessType,
		businessNo, idempotencyKey, remark); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE balance_accounts SET "+column+"=?,version=version+1 WHERE tenant_id=? AND customer_id=?",
		after, tenantID, customerID)
	return err
}
