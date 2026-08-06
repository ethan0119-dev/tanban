package app

import (
	"database/sql"
	"net/http"
	"strings"
)

type wechatOnboardingReviewInput struct {
	Action string `json:"action"`
	Note   string `json:"note"`
}

type pendingOnboardingItem struct {
	TenantID          int64  `json:"tenantId"`
	TenantName        string `json:"tenantName"`
	TenantCode        string `json:"tenantCode"`
	SubjectType       string `json:"subjectType"`
	MerchantShortName string `json:"merchantShortName"`
	OperatorName      string `json:"operatorName"`
	ContactPhone      string `json:"contactPhone"`
	ApplicationStatus string `json:"applicationStatus"`
	PlatformNote      string `json:"platformNote"`
	SubmittedAt       string `json:"submittedAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type wechatOnboardingApplication struct {
	SubjectType                string `json:"subjectType"`
	BusinessScene              string `json:"businessScene"`
	MerchantShortName          string `json:"merchantShortName"`
	ServicePhone               string `json:"servicePhone"`
	BusinessAddress            string `json:"businessAddress"`
	OperatorName               string `json:"operatorName"`
	ContactPhone               string `json:"contactPhone"`
	ContactEmail               string `json:"contactEmail"`
	LicenseNumber              string `json:"licenseNumber"`
	QualificationConfirmed     bool   `json:"qualificationConfirmed"`
	IdentityMaterialReady      bool   `json:"identityMaterialReady"`
	SettlementAccountReady     bool   `json:"settlementAccountReady"`
	BusinessMaterialReady      bool   `json:"businessMaterialReady"`
	ApplicationStatus          string `json:"applicationStatus"`
	PlatformNote               string `json:"platformNote"`
	SubmittedAt                string `json:"submittedAt"`
	UpdatedAt                  string `json:"updatedAt"`
	SensitiveCollectionEnabled bool   `json:"sensitiveCollectionEnabled"`
	ProviderSubmissionEnabled  bool   `json:"providerSubmissionEnabled"`
	SensitiveConfigured        bool   `json:"sensitiveConfigured"`
	WechatApplymentID          string `json:"wechatApplymentId"`
	WechatApplymentState       string `json:"wechatApplymentState"`
	WechatStateMessage         string `json:"wechatStateMessage"`
	SignURL                    string `json:"signUrl"`
}

func defaultWechatOnboardingApplication() wechatOnboardingApplication {
	return wechatOnboardingApplication{
		SubjectType:       "INDIVIDUAL",
		BusinessScene:     "STORE",
		ApplicationStatus: "DRAFT",
	}
}

func (s *Server) getMerchantWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	application, err := s.loadWechatOnboarding(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, application)
}

func (s *Server) loadWechatOnboarding(r *http.Request, tenantID int64) (wechatOnboardingApplication, error) {
	application := defaultWechatOnboardingApplication()
	var ciphertext string
	err := s.DB.QueryRowContext(r.Context(), `SELECT subject_type,business_scene,merchant_short_name,service_phone,business_address,
		operator_name,contact_phone,contact_email,license_number,qualification_confirmed,identity_material_ready,
		settlement_account_ready,business_material_ready,application_status,platform_note,
		COALESCE(sensitive_ciphertext,''),wechat_applyment_id,wechat_applyment_state,wechat_state_message,sign_url,
		COALESCE(DATE_FORMAT(submitted_at,'%Y-%m-%d %H:%i:%s'),''),DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
		FROM wechat_pay_onboarding_applications WHERE tenant_id=?`, tenantID).
		Scan(&application.SubjectType, &application.BusinessScene, &application.MerchantShortName, &application.ServicePhone,
			&application.BusinessAddress, &application.OperatorName, &application.ContactPhone, &application.ContactEmail,
			&application.LicenseNumber, &application.QualificationConfirmed, &application.IdentityMaterialReady,
			&application.SettlementAccountReady, &application.BusinessMaterialReady, &application.ApplicationStatus,
			&application.PlatformNote, &ciphertext, &application.WechatApplymentID, &application.WechatApplymentState,
			&application.WechatStateMessage, &application.SignURL, &application.SubmittedAt, &application.UpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		return application, err
	}
	application.SensitiveConfigured = ciphertext != ""
	application.SensitiveCollectionEnabled = s.SensitiveData != nil
	if s.WeChatPay != nil && s.SensitiveData != nil {
		ready, _ := s.WeChatPay.APIv3Ready(r.Context())
		application.ProviderSubmissionEnabled = ready
	}
	return application, nil
}

func normalizeWechatOnboarding(input *wechatOnboardingApplication) {
	input.SubjectType = strings.ToUpper(strings.TrimSpace(input.SubjectType))
	input.BusinessScene = strings.ToUpper(strings.TrimSpace(input.BusinessScene))
	input.MerchantShortName = strings.TrimSpace(input.MerchantShortName)
	input.ServicePhone = strings.TrimSpace(input.ServicePhone)
	input.BusinessAddress = strings.TrimSpace(input.BusinessAddress)
	input.OperatorName = strings.TrimSpace(input.OperatorName)
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	input.LicenseNumber = strings.TrimSpace(input.LicenseNumber)
}

func validateWechatOnboarding(w http.ResponseWriter, input wechatOnboardingApplication, submitting bool) bool {
	if !validStatus(input.SubjectType, "MICRO", "INDIVIDUAL", "ENTERPRISE") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "subjectType must be MICRO, INDIVIDUAL or ENTERPRISE")
		return false
	}
	if !validStatus(input.BusinessScene, "STORE", "MOBILE") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "businessScene must be STORE or MOBILE")
		return false
	}
	if input.SubjectType != "MICRO" && input.LicenseNumber == "" && submitting {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "个体工商户或企业进件必须填写营业执照统一社会信用代码")
		return false
	}
	if !submitting {
		return true
	}
	if input.MerchantShortName == "" || input.ServicePhone == "" || input.BusinessAddress == "" ||
		input.OperatorName == "" || input.ContactPhone == "" || input.ContactEmail == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请完整填写商户、经营地址、经营者和联系方式")
		return false
	}
	if !input.IdentityMaterialReady || !input.SettlementAccountReady || !input.BusinessMaterialReady {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请确认身份证、本人银行卡和经营证明材料均已准备")
		return false
	}
	return true
}

func (s *Server) saveMerchantWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input wechatOnboardingApplication
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWechatOnboarding(&input)
	if !validateWechatOnboarding(w, input, false) {
		return
	}
	if _, err := s.DB.ExecContext(r.Context(), `INSERT INTO wechat_pay_onboarding_applications(
		tenant_id,subject_type,business_scene,merchant_short_name,service_phone,business_address,operator_name,
		contact_phone,contact_email,license_number,qualification_confirmed,identity_material_ready,
		settlement_account_ready,business_material_ready,application_status
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,'DRAFT')
	ON DUPLICATE KEY UPDATE subject_type=VALUES(subject_type),business_scene=VALUES(business_scene),
		merchant_short_name=VALUES(merchant_short_name),service_phone=VALUES(service_phone),
		business_address=VALUES(business_address),operator_name=VALUES(operator_name),contact_phone=VALUES(contact_phone),
		contact_email=VALUES(contact_email),license_number=VALUES(license_number),
		qualification_confirmed=VALUES(qualification_confirmed),identity_material_ready=VALUES(identity_material_ready),
		settlement_account_ready=VALUES(settlement_account_ready),business_material_ready=VALUES(business_material_ready),
		application_status=IF(application_status IN ('DRAFT','NEEDS_INFO'),'DRAFT',application_status)`,
		actor.TenantID, input.SubjectType, input.BusinessScene, input.MerchantShortName, input.ServicePhone,
		input.BusinessAddress, input.OperatorName, input.ContactPhone, input.ContactEmail, input.LicenseNumber,
		input.QualificationConfirmed, input.IdentityMaterialReady, input.SettlementAccountReady, input.BusinessMaterialReady); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "merchant.wechat_onboarding.draft.save", "tenant", int64String(actor.TenantID),
		map[string]any{"subject_type": input.SubjectType, "business_scene": input.BusinessScene}, r)
	s.getMerchantWechatOnboarding(w, r)
}

func (s *Server) submitMerchantWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input wechatOnboardingApplication
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWechatOnboarding(&input)
	if !validateWechatOnboarding(w, input, true) {
		return
	}
	if s.SensitiveData == nil {
		writeError(w, http.StatusServiceUnavailable, "SENSITIVE_STORE_NOT_CONFIGURED", "平台尚未配置专用数据加密主密钥")
		return
	}
	var sensitiveConfigured bool
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COALESCE(sensitive_ciphertext,'')<>'' FROM wechat_pay_onboarding_applications WHERE tenant_id=?", actor.TenantID).Scan(&sensitiveConfigured); err != nil && err != sql.ErrNoRows {
		handleSQLError(w, err)
		return
	}
	if !sensitiveConfigured {
		writeError(w, http.StatusBadRequest, "SENSITIVE_MATERIAL_REQUIRED", "请先通过安全资料步骤填写身份证、结算账户并上传进件图片")
		return
	}
	if _, err := s.DB.ExecContext(r.Context(), `INSERT INTO wechat_pay_onboarding_applications(
		tenant_id,subject_type,business_scene,merchant_short_name,service_phone,business_address,operator_name,
		contact_phone,contact_email,license_number,qualification_confirmed,identity_material_ready,
		settlement_account_ready,business_material_ready,application_status,submitted_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,'PENDING_PLATFORM_REVIEW',NOW(3))
	ON DUPLICATE KEY UPDATE subject_type=VALUES(subject_type),business_scene=VALUES(business_scene),
		merchant_short_name=VALUES(merchant_short_name),service_phone=VALUES(service_phone),
		business_address=VALUES(business_address),operator_name=VALUES(operator_name),contact_phone=VALUES(contact_phone),
		contact_email=VALUES(contact_email),license_number=VALUES(license_number),
		qualification_confirmed=VALUES(qualification_confirmed),identity_material_ready=VALUES(identity_material_ready),
		settlement_account_ready=VALUES(settlement_account_ready),business_material_ready=VALUES(business_material_ready),
		application_status='PENDING_PLATFORM_REVIEW',platform_note='',submitted_at=NOW(3)`,
		actor.TenantID, input.SubjectType, input.BusinessScene, input.MerchantShortName, input.ServicePhone,
		input.BusinessAddress, input.OperatorName, input.ContactPhone, input.ContactEmail, input.LicenseNumber,
		input.QualificationConfirmed, input.IdentityMaterialReady, input.SettlementAccountReady, input.BusinessMaterialReady); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "merchant.wechat_onboarding.submit", "tenant", int64String(actor.TenantID),
		map[string]any{"subject_type": input.SubjectType, "business_scene": input.BusinessScene}, r)
	s.getMerchantWechatOnboarding(w, r)
}

func (s *Server) reviewWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input wechatOnboardingReviewInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	input.Note = strings.TrimSpace(input.Note)

	if input.Action != "approve" && input.Action != "reject" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "action must be approve or reject")
		return
	}
	if input.Action == "reject" && input.Note == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "驳回时必须填写驳回原因")
		return
	}

	var currentStatus string
	err := s.DB.QueryRowContext(r.Context(),
		"SELECT application_status FROM wechat_pay_onboarding_applications WHERE tenant_id=?", tenantID).
		Scan(&currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "进件申请不存在")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if currentStatus != "PENDING_PLATFORM_REVIEW" {
		writeError(w, http.StatusConflict, "CONFLICT", "该申请当前不在待审核状态")
		return
	}

	var newAppStatus, newTenantStatus string
	if input.Action == "approve" {
		if _, submitErr := s.submitWechatApplyment(r.Context(), tenantID); submitErr != nil {
			s.Logger.Warn("submit WeChat Pay applyment", "tenant_id", tenantID, "error", submitErr)
			writeError(w, http.StatusBadGateway, "WECHAT_APPLYMENT_SUBMIT_FAILED", "提交微信支付进件失败，请核对资料或平台配置")
			return
		}
		newAppStatus = "SUBMITTED_TO_WECHAT"
		newTenantStatus = "REVIEWING"
	} else {
		newAppStatus = "NEEDS_INFO"
		newTenantStatus = "REJECTED"
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(),
		"UPDATE wechat_pay_onboarding_applications SET application_status=?, platform_note=? WHERE tenant_id=?",
		newAppStatus, input.Note, tenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		"UPDATE tenants SET payment_onboarding_status=? WHERE id=? AND deleted_at IS NULL",
		newTenantStatus, tenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}

	auditAction := "platform.wechat_onboarding.approve"
	if input.Action == "reject" {
		auditAction = "platform.wechat_onboarding.reject"
	}
	s.audit(r.Context(), currentIdentity(r.Context()), auditAction, "tenant", int64String(tenantID),
		map[string]any{"note": input.Note}, r)
	s.getTenantPaymentSettings(w, r)
}

func (s *Server) listPendingWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	query := `SELECT w.tenant_id, t.name, COALESCE(t.code,''),
		w.subject_type, w.merchant_short_name, w.operator_name, w.contact_phone,
		w.application_status, COALESCE(w.platform_note,''),
		COALESCE(DATE_FORMAT(w.submitted_at,'%Y-%m-%d %H:%i:%s'),''),
		DATE_FORMAT(w.updated_at,'%Y-%m-%d %H:%i:%s')
		FROM wechat_pay_onboarding_applications w
		JOIN tenants t ON t.id = w.tenant_id AND t.deleted_at IS NULL`
	args := []any{}
	if status != "" && status != "ALL" {
		if !validStatus(status, "PENDING_PLATFORM_REVIEW", "SUBMITTED_TO_WECHAT", "NEEDS_INFO", "FINISHED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status filter")
			return
		}
		query += " WHERE w.application_status = ?"
		args = append(args, status)
	} else {
		query += " WHERE w.application_status != 'DRAFT'"
	}
	query += " ORDER BY w.updated_at DESC"
	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()

	items := []pendingOnboardingItem{}
	for rows.Next() {
		var item pendingOnboardingItem
		if err := rows.Scan(&item.TenantID, &item.TenantName, &item.TenantCode,
			&item.SubjectType, &item.MerchantShortName, &item.OperatorName, &item.ContactPhone,
			&item.ApplicationStatus, &item.PlatformNote, &item.SubmittedAt, &item.UpdatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}
