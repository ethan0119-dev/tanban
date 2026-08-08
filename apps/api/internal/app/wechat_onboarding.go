package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	wechatcore "github.com/wechatpay-apiv3/wechatpay-go/core"
)

type wechatOnboardingReviewInput struct {
	Action             string `json:"action"`
	Note               string `json:"note"`
	MaterialsConfirmed bool   `json:"materialsConfirmed"`
}

type wechatOnboardingReviewDetail struct {
	TenantID                    int64                           `json:"tenantId"`
	TenantName                  string                          `json:"tenantName"`
	TenantCode                  string                          `json:"tenantCode"`
	Application                 wechatOnboardingApplication     `json:"application"`
	Sensitive                   wechatOnboardingSensitiveInput  `json:"sensitive"`
	Media                       []onboardingReviewMedia         `json:"media"`
	FieldMappings               []onboardingFieldMapping        `json:"fieldMappings"`
	MediaRequirements           []onboardingMediaRequirement    `json:"mediaRequirements"`
	IndustryOptions             []onboardingIndustryRequirement `json:"industryOptions"`
	SelectedIndustryRequirement *onboardingIndustryRequirement  `json:"selectedIndustryRequirement,omitempty"`
	MissingItems                []string                        `json:"missingItems"`
	Warnings                    []string                        `json:"warnings"`
	SubmissionBlockers          []string                        `json:"submissionBlockers"`
	ReviewReady                 bool                            `json:"reviewReady"`
}

type wechatOnboardingAdminUpdateInput struct {
	Application wechatOnboardingApplication    `json:"application"`
	Sensitive   wechatOnboardingSensitiveInput `json:"sensitive"`
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
		COALESCE(sensitive_ciphertext,''),COALESCE(wechat_applyment_id,''),wechat_applyment_state,wechat_state_message,sign_url,
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
	if !input.QualificationConfirmed {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请确认所填资料和上传材料真实有效")
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
	if input.Action == "approve" {
		if !input.MaterialsConfirmed {
			writeError(w, http.StatusBadRequest, "MATERIAL_REVIEW_REQUIRED", "请先打开完整商户资料并确认已人工核对证件与照片")
			return
		}
		detail, detailErr := s.loadWechatOnboardingReviewDetail(r, tenantID)
		if detailErr != nil {
			handleSQLError(w, detailErr)
			return
		}
		if !detail.ReviewReady {
			writeError(w, http.StatusConflict, "ONBOARDING_MATERIAL_INCOMPLETE", "进件资料不完整，请驳回并要求商户补充后再审核")
			return
		}
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
			message := "提交微信支付进件失败，请核对资料或平台配置"
			var apiErr *wechatcore.APIError
			if errors.As(submitErr, &apiErr) && strings.TrimSpace(apiErr.Message) != "" {
				message = "微信支付进件失败：" + strings.TrimSpace(apiErr.Message)
				if strings.TrimSpace(apiErr.Code) != "" {
					message += "（" + strings.TrimSpace(apiErr.Code) + "）"
				}
			}
			writeError(w, http.StatusBadGateway, "WECHAT_APPLYMENT_SUBMIT_FAILED", message)
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

func (s *Server) getWechatOnboardingReviewDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	detail, err := s.loadWechatOnboardingReviewDetail(r, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "进件申请不存在")
			return
		}
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "platform.wechat_onboarding.material.view", "tenant", int64String(tenantID), map[string]any{"review_ready": detail.ReviewReady}, r)
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, detail)
}

func (s *Server) updateWechatOnboardingReviewDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	if s.SensitiveData == nil {
		writeError(w, http.StatusServiceUnavailable, "SENSITIVE_STORE_NOT_CONFIGURED", "平台尚未配置专用数据加密主密钥")
		return
	}
	var input wechatOnboardingAdminUpdateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWechatOnboarding(&input.Application)
	normalizeWechatOnboardingSensitive(&input.Sensitive)
	if err := validateWechatOnboardingAdminUpdate(input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var ciphertext, currentStatus string
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COALESCE(sensitive_ciphertext,''),application_status
		FROM wechat_pay_onboarding_applications WHERE tenant_id=?`, tenantID).Scan(&ciphertext, &currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "进件申请不存在")
			return
		}
		handleSQLError(w, err)
		return
	}
	if currentStatus != "PENDING_PLATFORM_REVIEW" {
		writeError(w, http.StatusConflict, "CONFLICT", "只有待平台审核的申请允许编辑")
		return
	}
	if ciphertext == "" {
		writeError(w, http.StatusConflict, "SENSITIVE_MATERIAL_REQUIRED", "申请尚未保存安全进件资料")
		return
	}
	currentSensitive, err := s.decryptWechatOnboardingSensitiveRaw(r.Context(), tenantID, ciphertext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DECRYPTION_FAILED", "安全进件资料解密失败")
		return
	}
	preserveWechatOnboardingMediaReferences(&input.Sensitive, currentSensitive)
	payload, err := json.Marshal(input.Sensitive)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ENCODING_FAILED", "安全进件资料编码失败")
		return
	}
	updatedCiphertext, err := s.SensitiveData.Encrypt(payload, tenantID, wechatOnboardingPurpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ENCRYPTION_FAILED", "安全进件资料加密失败")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE wechat_pay_onboarding_applications SET
		subject_type=?,business_scene=?,merchant_short_name=?,service_phone=?,business_address=?,operator_name=?,
		contact_phone=?,contact_email=?,license_number=?,qualification_confirmed=?,identity_material_ready=?,
		settlement_account_ready=?,business_material_ready=?,sensitive_ciphertext=?,sensitive_key_version='v1',platform_note=''
		WHERE tenant_id=? AND application_status='PENDING_PLATFORM_REVIEW'`,
		input.Application.SubjectType, input.Application.BusinessScene, input.Application.MerchantShortName,
		input.Application.ServicePhone, input.Application.BusinessAddress, input.Application.OperatorName,
		input.Application.ContactPhone, input.Application.ContactEmail, input.Application.LicenseNumber,
		input.Application.QualificationConfirmed, input.Application.IdentityMaterialReady,
		input.Application.SettlementAccountReady, input.Application.BusinessMaterialReady, updatedCiphertext, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "CONFLICT", "申请状态已变化，请刷新后重试")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "platform.wechat_onboarding.material.update", "tenant", int64String(tenantID), map[string]any{
		"subject_type": input.Application.SubjectType, "settlement_id": input.Sensitive.SettlementID,
		"qualification_type": input.Sensitive.QualificationType,
	}, r)
	s.getWechatOnboardingReviewDetail(w, r)
}

func validateWechatOnboardingAdminUpdate(input wechatOnboardingAdminUpdateInput) error {
	if !validStatus(input.Application.SubjectType, "MICRO", "INDIVIDUAL", "ENTERPRISE") {
		return errors.New("主体类型不正确")
	}
	if !validStatus(input.Application.BusinessScene, "STORE", "MOBILE") {
		return errors.New("经营场景不正确")
	}
	if input.Sensitive.AccountType != "" && !validStatus(input.Sensitive.AccountType, "BANK_ACCOUNT_TYPE_CORPORATE", "BANK_ACCOUNT_TYPE_PERSONAL") {
		return errors.New("结算账户类型不正确")
	}
	if input.Application.SubjectType == "MICRO" && input.Sensitive.AccountType != "" && input.Sensitive.AccountType != "BANK_ACCOUNT_TYPE_PERSONAL" {
		return errors.New("小微商户只能使用经营者个人银行卡")
	}
	if input.Application.SubjectType == "ENTERPRISE" && input.Sensitive.AccountType != "" && input.Sensitive.AccountType != "BANK_ACCOUNT_TYPE_CORPORATE" {
		return errors.New("企业主体只能使用对公银行账户")
	}
	if input.Sensitive.BankAddressCode != "" && (len(input.Sensitive.BankAddressCode) != 6 || !digitsOnly(input.Sensitive.BankAddressCode)) {
		return errors.New("开户银行省市编码必须是6位数字")
	}
	if input.Sensitive.BankBranchID != "" && (len(input.Sensitive.BankBranchID) != 12 || !digitsOnly(input.Sensitive.BankBranchID)) {
		return errors.New("开户银行联行号必须是12位数字")
	}
	expectedSettlementID := expectedWechatSettlementID(input.Application.SubjectType)
	if input.Sensitive.SettlementID != "" && input.Sensitive.SettlementID != expectedSettlementID {
		return fmt.Errorf("%s主体的结算规则 ID 必须为 %s", input.Application.SubjectType, expectedSettlementID)
	}
	if input.Sensitive.SettlementID != "" && input.Sensitive.QualificationType != "" && input.Application.SubjectType != "MICRO" {
		if _, ok := selectedOnboardingIndustryRule(input.Application.SubjectType, input.Sensitive.SettlementID, input.Sensitive.QualificationType); !ok {
			return errors.New("所属行业与主体类型、结算规则 ID 不匹配，请从微信官方行业列表中选择")
		}
	}
	return nil
}

func preserveWechatOnboardingMediaReferences(target *wechatOnboardingSensitiveInput, current wechatOnboardingSensitiveInput) {
	target.IDCardCopy = current.IDCardCopy
	target.IDCardNational = current.IDCardNational
	target.BusinessLicenseCopy = current.BusinessLicenseCopy
	target.StoreEntrancePic = current.StoreEntrancePic
	target.IndoorPic = current.IndoorPic
	target.CashierPic = current.CashierPic
	target.MiniProgramPics = append([]string(nil), current.MiniProgramPics...)
	target.QualificationPics = append([]string(nil), current.QualificationPics...)
}

func (s *Server) loadWechatOnboardingReviewDetail(r *http.Request, tenantID int64) (wechatOnboardingReviewDetail, error) {
	detail := wechatOnboardingReviewDetail{TenantID: tenantID, Media: []onboardingReviewMedia{}, MissingItems: []string{}, Warnings: []string{}, SubmissionBlockers: []string{}}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT name,COALESCE(code,'') FROM tenants WHERE id=? AND deleted_at IS NULL", tenantID).Scan(&detail.TenantName, &detail.TenantCode); err != nil {
		return detail, err
	}
	application, err := s.loadWechatOnboarding(r, tenantID)
	if err != nil {
		return detail, err
	}
	detail.Application = application
	var ciphertext string
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COALESCE(sensitive_ciphertext,'') FROM wechat_pay_onboarding_applications WHERE tenant_id=?", tenantID).Scan(&ciphertext); err != nil {
		return detail, err
	}
	if ciphertext != "" && s.SensitiveData != nil {
		detail.Sensitive, err = s.decryptWechatOnboardingSensitiveRaw(r.Context(), tenantID, ciphertext)
		if err != nil {
			return detail, err
		}
	}
	mediaByField := map[string]onboardingReviewMedia{}
	if s.SensitiveData != nil {
		mediaByField, err = s.loadOnboardingReviewMedia(r.Context(), tenantID)
		if err != nil {
			return detail, err
		}
	}
	for _, item := range mediaByField {
		detail.Media = append(detail.Media, item)
	}
	sort.Slice(detail.Media, func(i, j int) bool { return detail.Media[i].FieldName < detail.Media[j].FieldName })
	detail.MissingItems = missingWechatOnboardingReviewItems(application, detail.Sensitive, mediaByField, ciphertext != "")
	detail.FieldMappings = append([]onboardingFieldMapping(nil), commonWechatOnboardingFieldMappings...)
	detail.MediaRequirements = onboardingMediaRequirements(application, detail.Sensitive)
	detail.IndustryOptions = allOnboardingIndustryOptions()
	if selected, ok := selectedOnboardingIndustryRule(application.SubjectType, detail.Sensitive.SettlementID, detail.Sensitive.QualificationType); ok {
		detail.SelectedIndustryRequirement = &selected
		if selected.QualificationMode == "CONDITIONAL" {
			detail.Warnings = append(detail.Warnings, "当前行业的特殊资质要求取决于实际经营内容，请人工核对经营范围后决定是否提交资质。")
		}
	} else if application.SubjectType != "MICRO" && detail.Sensitive.SettlementID != "" && detail.Sensitive.QualificationType != "" {
		detail.MissingItems = append(detail.MissingItems, "所属行业与主体类型、结算规则 ID 不匹配")
	}
	if expected := expectedWechatSettlementID(application.SubjectType); detail.Sensitive.SettlementID != "" && detail.Sensitive.SettlementID != expected {
		detail.MissingItems = append(detail.MissingItems, "结算规则 ID 与主体类型不匹配（应为 "+expected+"）")
	}
	if application.SubjectType == "MICRO" {
		settings := paymentSettings{MicroOnboardingPermissionStatus: "NOT_APPLIED"}
		_ = s.loadSettingJSON(r, "payment", &settings)
		if settings.MicroOnboardingPermissionStatus != "ENABLED" {
			detail.SubmissionBlockers = append(detail.SubmissionBlockers, "服务商尚未取得微信支付小微进件权限；当前提交会返回 NO_AUTH，请先在合作伙伴平台完成权限申请并将平台状态改为已开通。")
		}
	}
	detail.ReviewReady = len(detail.MissingItems) == 0 && len(detail.SubmissionBlockers) == 0
	return detail, nil
}

func missingWechatOnboardingReviewItems(app wechatOnboardingApplication, sensitive wechatOnboardingSensitiveInput, media map[string]onboardingReviewMedia, sensitiveConfigured bool) []string {
	missing := []string{}
	addText := func(value, label string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, label)
		}
	}
	addText(app.MerchantShortName, "商户简称")
	addText(app.SubjectType, "主体类型")
	addText(app.BusinessScene, "经营场景")
	addText(app.ServicePhone, "客服电话")
	addText(app.BusinessAddress, "实际经营地址")
	addText(app.OperatorName, "经营者／法定代表人")
	addText(app.ContactPhone, "联系手机号")
	addText(app.ContactEmail, "联系邮箱")
	if app.SubjectType != "MICRO" {
		addText(app.LicenseNumber, "营业执照统一社会信用代码")
	}
	if !app.QualificationConfirmed {
		missing = append(missing, "资料真实有效确认")
	}
	if !app.IdentityMaterialReady {
		missing = append(missing, "身份证材料准备确认")
	}
	if !app.SettlementAccountReady {
		missing = append(missing, "结算账户材料准备确认")
	}
	if !app.BusinessMaterialReady {
		missing = append(missing, "经营场景材料准备确认")
	}
	if !sensitiveConfigured {
		return append(missing, "身份证、结算账户及经营场景安全资料")
	}
	for _, item := range []struct{ value, label string }{
		{sensitive.IDCardName, "身份证姓名"}, {sensitive.IDCardNumber, "身份证号码"}, {sensitive.IDCardAddress, "身份证住址"}, {sensitive.CardPeriodBegin, "身份证有效期开始"},
		{sensitive.CardPeriodEnd, "身份证有效期结束"}, {sensitive.AccountName, "结算账户名称"}, {sensitive.AccountNumber, "银行账号"},
		{sensitive.AccountType, "结算账户类型"}, {sensitive.AccountBank, "开户银行"}, {sensitive.StoreName, "门店名称"}, {sensitive.StoreAddressCode, "门店省市编码"},
		{sensitive.SettlementID, "结算规则 ID"}, {sensitive.QualificationType, "所属行业"},
	} {
		addText(item.value, item.label)
	}
	if app.SubjectType != "MICRO" {
		addText(sensitive.MerchantName, "营业执照主体全称")
		addText(sensitive.LegalPerson, "营业执照经营者／法定代表人")
	}
	if sensitive.AccountBank == "其他银行" && sensitive.BankBranchID == "" && sensitive.BankName == "" {
		missing = append(missing, "其他银行的支行全称或联行号")
	}
	requireMedia := func(field, label, expectedMediaID string) {
		item, ok := media[field]
		if !ok || item.DataURL == "" || !item.WechatSet || expectedMediaID == "" || item.WechatMediaID != expectedMediaID {
			missing = append(missing, label+"（需重新上传审核副本）")
		}
	}
	if app.SubjectType != "MICRO" {
		requireMedia("businessLicenseCopy", "营业执照照片", sensitive.BusinessLicenseCopy)
	}
	requireMedia("idCardCopy", "身份证人像面", sensitive.IDCardCopy)
	requireMedia("idCardNational", "身份证国徽面", sensitive.IDCardNational)
	requireMedia("storeEntrancePic", "门店门头照片", sensitive.StoreEntrancePic)
	requireMedia("indoorPic", "店内环境照片", sensitive.IndoorPic)
	if app.SubjectType != "MICRO" {
		miniProgramMediaID := ""
		if len(sensitive.MiniProgramPics) > 0 {
			miniProgramMediaID = sensitive.MiniProgramPics[0]
		}
		requireMedia("miniProgramPic", "小程序经营页面截图", miniProgramMediaID)
	}
	if rule, ok := selectedOnboardingIndustryRule(app.SubjectType, sensitive.SettlementID, sensitive.QualificationType); ok {
		switch rule.QualificationMode {
		case "REQUIRED":
			qualificationID := ""
			if len(sensitive.QualificationPics) > 0 {
				qualificationID = sensitive.QualificationPics[0]
			}
			requireMedia("qualificationPic", "行业特殊资质", qualificationID)
		case "ALTERNATIVE":
			if len(sensitive.QualificationPics) > 0 {
				requireMedia("qualificationPic", "行业特殊资质", sensitive.QualificationPics[0])
			} else {
				requireMedia("cashierPic", "收银台照片（无许可证时的替代材料）", sensitive.CashierPic)
			}
		}
	}
	return missing
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
