package app

import (
	"database/sql"
	"net/http"
	"strings"
)

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
}

func defaultWechatOnboardingApplication() wechatOnboardingApplication {
	return wechatOnboardingApplication{
		SubjectType:       "MICRO",
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
	err := s.DB.QueryRowContext(r.Context(), `SELECT subject_type,business_scene,merchant_short_name,service_phone,business_address,
		operator_name,contact_phone,contact_email,license_number,qualification_confirmed,identity_material_ready,
		settlement_account_ready,business_material_ready,application_status,platform_note,
		COALESCE(DATE_FORMAT(submitted_at,'%Y-%m-%d %H:%i:%s'),''),DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
		FROM wechat_pay_onboarding_applications WHERE tenant_id=?`, tenantID).
		Scan(&application.SubjectType, &application.BusinessScene, &application.MerchantShortName, &application.ServicePhone,
			&application.BusinessAddress, &application.OperatorName, &application.ContactPhone, &application.ContactEmail,
			&application.LicenseNumber, &application.QualificationConfirmed, &application.IdentityMaterialReady,
			&application.SettlementAccountReady, &application.BusinessMaterialReady, &application.ApplicationStatus,
			&application.PlatformNote, &application.SubmittedAt, &application.UpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		return application, err
	}
	// Identity documents and bank-card numbers are deliberately not accepted until a
	// dedicated encrypted-at-rest store and the provider permission are configured.
	application.SensitiveCollectionEnabled = false
	application.ProviderSubmissionEnabled = false
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
		input.OperatorName == "" || input.ContactPhone == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请完整填写商户、经营地址、经营者和联系方式")
		return false
	}
	if input.SubjectType == "MICRO" && !input.QualificationConfirmed {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "小微商户必须确认属于依法免办理工商登记的实体经营者")
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
