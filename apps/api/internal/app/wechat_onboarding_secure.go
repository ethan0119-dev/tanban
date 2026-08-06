package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

const wechatOnboardingPurpose = "wechat-pay-onboarding-sensitive"
const wechatOnboardingMediaPurpose = "wechat-pay-onboarding-review-media"

type wechatOnboardingSensitiveInput struct {
	IDCardName          string   `json:"idCardName"`
	IDCardNumber        string   `json:"idCardNumber"`
	IDCardAddress       string   `json:"idCardAddress"`
	CardPeriodBegin     string   `json:"cardPeriodBegin"`
	CardPeriodEnd       string   `json:"cardPeriodEnd"`
	IDCardCopy          string   `json:"idCardCopy"`
	IDCardNational      string   `json:"idCardNational"`
	BusinessLicenseCopy string   `json:"businessLicenseCopy"`
	MerchantName        string   `json:"merchantName"`
	LegalPerson         string   `json:"legalPerson"`
	AccountType         string   `json:"accountType"`
	AccountName         string   `json:"accountName"`
	AccountNumber       string   `json:"accountNumber"`
	AccountBank         string   `json:"accountBank"`
	BankAddressCode     string   `json:"bankAddressCode"`
	BankBranchID        string   `json:"bankBranchId"`
	BankName            string   `json:"bankName"`
	StoreName           string   `json:"storeName"`
	StoreAddressCode    string   `json:"storeAddressCode"`
	StoreEntrancePic    string   `json:"storeEntrancePic"`
	IndoorPic           string   `json:"indoorPic"`
	CashierPic          string   `json:"cashierPic"`
	MiniProgramPics     []string `json:"miniProgramPics"`
	QualificationPics   []string `json:"qualificationPics"`
	SettlementID        string   `json:"settlementId"`
	QualificationType   string   `json:"qualificationType"`
}

func normalizeWechatOnboardingSensitive(input *wechatOnboardingSensitiveInput) {
	input.IDCardName = strings.TrimSpace(input.IDCardName)
	input.IDCardNumber = strings.ToUpper(strings.TrimSpace(input.IDCardNumber))
	input.IDCardAddress = strings.TrimSpace(input.IDCardAddress)
	input.CardPeriodBegin = strings.TrimSpace(input.CardPeriodBegin)
	input.CardPeriodEnd = strings.TrimSpace(input.CardPeriodEnd)
	input.IDCardCopy = strings.TrimSpace(input.IDCardCopy)
	input.IDCardNational = strings.TrimSpace(input.IDCardNational)
	input.BusinessLicenseCopy = strings.TrimSpace(input.BusinessLicenseCopy)
	input.MerchantName = strings.TrimSpace(input.MerchantName)
	input.LegalPerson = strings.TrimSpace(input.LegalPerson)
	input.AccountType = strings.ToUpper(strings.TrimSpace(input.AccountType))
	input.AccountName = strings.TrimSpace(input.AccountName)
	input.AccountNumber = strings.TrimSpace(input.AccountNumber)
	input.AccountBank = strings.TrimSpace(input.AccountBank)
	input.BankAddressCode = strings.TrimSpace(input.BankAddressCode)
	input.BankBranchID = strings.TrimSpace(input.BankBranchID)
	input.BankName = strings.TrimSpace(input.BankName)
	input.StoreName = strings.TrimSpace(input.StoreName)
	input.StoreAddressCode = strings.TrimSpace(input.StoreAddressCode)
	input.StoreEntrancePic = strings.TrimSpace(input.StoreEntrancePic)
	input.IndoorPic = strings.TrimSpace(input.IndoorPic)
	input.CashierPic = strings.TrimSpace(input.CashierPic)
	input.SettlementID = strings.TrimSpace(input.SettlementID)
	input.QualificationType = strings.TrimSpace(input.QualificationType)
	for i := range input.MiniProgramPics {
		input.MiniProgramPics[i] = strings.TrimSpace(input.MiniProgramPics[i])
	}
	for i := range input.QualificationPics {
		input.QualificationPics[i] = strings.TrimSpace(input.QualificationPics[i])
	}
}

func validateWechatOnboardingSensitive(input wechatOnboardingSensitiveInput, subjectType string) error {
	required := map[string]string{
		input.IDCardName: "身份证姓名", input.IDCardNumber: "身份证号码", input.IDCardAddress: "身份证住址",
		input.CardPeriodBegin: "身份证有效期开始日期", input.CardPeriodEnd: "身份证有效期结束日期", input.IDCardCopy: "身份证人像面", input.IDCardNational: "身份证国徽面",
		input.AccountName: "结算账户名称", input.AccountNumber: "结算账户号", input.AccountBank: "开户银行",
		input.StoreName: "门店名称", input.StoreAddressCode: "门店省市编码",
		input.StoreEntrancePic: "门店门头照片", input.IndoorPic: "店内环境照片", input.SettlementID: "结算规则ID",
		input.QualificationType: "所属行业名称",
	}
	if subjectType != "MICRO" {
		required[input.BusinessLicenseCopy] = "营业执照照片"
		required[input.MerchantName] = "主体全称"
		required[input.LegalPerson] = "法定代表人/经营者"
	}
	for value, label := range required {
		if value == "" {
			return fmt.Errorf("请填写或上传%s", label)
		}
	}
	if !validStatus(input.AccountType, "BANK_ACCOUNT_TYPE_CORPORATE", "BANK_ACCOUNT_TYPE_PERSONAL") {
		return errors.New("结算账户类型不正确")
	}
	if subjectType == "MICRO" && input.AccountType != "BANK_ACCOUNT_TYPE_PERSONAL" {
		return errors.New("小微商户只能使用经营者个人银行卡")
	}
	if input.BankAddressCode != "" && (len(input.BankAddressCode) != 6 || !digitsOnly(input.BankAddressCode)) {
		return errors.New("开户银行省市编码必须是微信支付地区表中的6位数字")
	}
	if input.BankBranchID != "" && (len(input.BankBranchID) != 12 || !digitsOnly(input.BankBranchID)) {
		return errors.New("开户银行联行号必须是12位数字")
	}
	if input.AccountBank == "其他银行" && input.BankBranchID == "" && input.BankName == "" {
		return errors.New("开户银行为其他银行时，开户支行全称和联行号至少填写一项")
	}
	if subjectType != "MICRO" && len(input.MiniProgramPics) == 0 {
		return errors.New("请至少上传一张小程序经营页面截图")
	}
	if input.QualificationType == "餐饮" && len(input.QualificationPics) == 0 && input.CashierPic == "" {
		return errors.New("餐饮商户请上传食品经营资质，或补充收银台照片")
	}
	return nil
}

func (s *Server) saveMerchantWechatOnboardingSensitive(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	if s.SensitiveData == nil {
		writeError(w, http.StatusServiceUnavailable, "SENSITIVE_STORE_NOT_CONFIGURED", "平台尚未配置专用数据加密主密钥")
		return
	}
	var input wechatOnboardingSensitiveInput
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWechatOnboardingSensitive(&input)
	var subjectType string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT subject_type FROM wechat_pay_onboarding_applications WHERE tenant_id=?", actor.TenantID).Scan(&subjectType); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusConflict, "ONBOARDING_DRAFT_REQUIRED", "请先保存主体基础资料")
			return
		}
		handleSQLError(w, err)
		return
	}
	if err := validateWechatOnboardingSensitive(input, subjectType); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	payload, _ := json.Marshal(input)
	ciphertext, err := s.SensitiveData.Encrypt(payload, actor.TenantID, wechatOnboardingPurpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ENCRYPTION_FAILED", "敏感资料加密失败")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE wechat_pay_onboarding_applications
		SET sensitive_ciphertext=?,sensitive_key_version='v1',application_status=IF(application_status IN ('DRAFT','NEEDS_INFO'),'DRAFT',application_status)
		WHERE tenant_id=?`, ciphertext, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "ONBOARDING_DRAFT_REQUIRED", "请先保存主体基础资料")
		return
	}
	s.audit(r.Context(), actor, "merchant.wechat_onboarding.sensitive.save", "tenant", int64String(actor.TenantID), map[string]any{"key_version": "v1"}, r)
	s.getMerchantWechatOnboarding(w, r)
}

func (s *Server) uploadWechatOnboardingMedia(w http.ResponseWriter, r *http.Request) {
	wechat := s.WeChatPay
	if wechat == nil {
		writeError(w, http.StatusServiceUnavailable, "WECHAT_PAY_NOT_ACTIVE", "微信支付服务商尚未启用")
		return
	}
	if s.SensitiveData == nil {
		writeError(w, http.StatusServiceUnavailable, "SENSITIVE_STORE_NOT_CONFIGURED", "平台尚未配置专用数据加密主密钥")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	if err := r.ParseMultipartForm(3 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_IMAGE", "图片不得超过 2MB")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_IMAGE", "请选择进件图片")
		return
	}
	defer file.Close()
	fieldName := strings.TrimSpace(r.FormValue("field"))
	if _, ok := onboardingReviewMediaLabels[fieldName]; !ok {
		writeError(w, http.StatusBadRequest, "INVALID_MEDIA_FIELD", "不支持的进件图片类型")
		return
	}
	contentType := detectUploadContentType(file, header)
	contents, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil || len(contents) == 0 || len(contents) > 2<<20 {
		writeError(w, http.StatusBadRequest, "INVALID_IMAGE", "图片不得超过 2MB")
		return
	}
	_, _ = file.Seek(0, io.SeekStart)
	mediaID, err := wechat.UploadApplymentImage(r.Context(), file, filepath.Base(header.Filename), contentType)
	if err != nil {
		s.Logger.Warn("upload WeChat onboarding image", "error", err)
		writeError(w, http.StatusBadGateway, "WECHAT_MEDIA_UPLOAD_FAILED", "图片上传微信支付失败")
		return
	}
	actor := currentIdentity(r.Context())
	ciphertext, err := s.SensitiveData.Encrypt(contents, actor.TenantID, wechatOnboardingMediaPurpose+":"+fieldName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ENCRYPTION_FAILED", "审核图片加密失败")
		return
	}
	if _, err = s.DB.ExecContext(r.Context(), `INSERT INTO wechat_pay_onboarding_review_media(
		tenant_id,field_name,ordinal_no,content_type,original_filename,ciphertext,key_version,wechat_media_id
	) VALUES(?,?,0,?,?,?,'v1',?)
	ON DUPLICATE KEY UPDATE content_type=VALUES(content_type),original_filename=VALUES(original_filename),
		ciphertext=VALUES(ciphertext),key_version='v1',wechat_media_id=VALUES(wechat_media_id)`,
		actor.TenantID, fieldName, contentType, filepath.Base(header.Filename), ciphertext, mediaID); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "merchant.wechat_onboarding.media.upload", "tenant", int64String(actor.TenantID), map[string]any{"content_type": contentType, "field": fieldName}, r)
	writeData(w, http.StatusCreated, map[string]string{"mediaId": mediaID})
}

var onboardingReviewMediaLabels = map[string]string{
	"businessLicenseCopy": "营业执照",
	"idCardCopy":          "身份证人像面",
	"idCardNational":      "身份证国徽面",
	"storeEntrancePic":    "门店门头",
	"indoorPic":           "店内环境",
	"cashierPic":          "收银台",
	"miniProgramPic":      "小程序经营页面",
	"qualificationPic":    "行业特殊资质",
}

type onboardingReviewMedia struct {
	FieldName     string `json:"fieldName"`
	Label         string `json:"label"`
	ContentType   string `json:"contentType"`
	DataURL       string `json:"dataUrl"`
	WechatSet     bool   `json:"wechatMediaIdConfigured"`
	WechatMediaID string `json:"-"`
}

func (s *Server) loadOnboardingReviewMedia(ctx context.Context, tenantID int64) (map[string]onboardingReviewMedia, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT field_name,content_type,ciphertext,wechat_media_id
		FROM wechat_pay_onboarding_review_media WHERE tenant_id=? ORDER BY field_name,ordinal_no`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]onboardingReviewMedia{}
	for rows.Next() {
		var fieldName, contentType, ciphertext, mediaID string
		if err = rows.Scan(&fieldName, &contentType, &ciphertext, &mediaID); err != nil {
			return nil, err
		}
		plaintext, decryptErr := s.SensitiveData.Decrypt(ciphertext, tenantID, wechatOnboardingMediaPurpose+":"+fieldName)
		if decryptErr != nil {
			return nil, decryptErr
		}
		result[fieldName] = onboardingReviewMedia{
			FieldName: fieldName, Label: onboardingReviewMediaLabels[fieldName], ContentType: contentType,
			DataURL:   "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(plaintext),
			WechatSet: mediaID != "", WechatMediaID: mediaID,
		}
	}
	return result, rows.Err()
}

func detectUploadContentType(file multipart.File, header *multipart.FileHeader) string {
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	_, _ = file.Seek(0, io.SeekStart)
	detected := http.DetectContentType(buffer[:n])
	if detected == "application/octet-stream" {
		detected = header.Header.Get("Content-Type")
	}
	return detected
}

func (s *Server) decryptWechatOnboardingSensitive(ctx context.Context, tenantID int64, ciphertext, subjectType string) (wechatOnboardingSensitiveInput, error) {
	input, err := s.decryptWechatOnboardingSensitiveRaw(ctx, tenantID, ciphertext)
	if err != nil {
		return input, err
	}
	return input, validateWechatOnboardingSensitive(input, subjectType)
}

func (s *Server) decryptWechatOnboardingSensitiveRaw(ctx context.Context, tenantID int64, ciphertext string) (wechatOnboardingSensitiveInput, error) {
	if s.SensitiveData == nil {
		return wechatOnboardingSensitiveInput{}, errors.New("sensitive store is not configured")
	}
	plaintext, err := s.SensitiveData.Decrypt(ciphertext, tenantID, wechatOnboardingPurpose)
	if err != nil {
		return wechatOnboardingSensitiveInput{}, err
	}
	var input wechatOnboardingSensitiveInput
	if err = json.Unmarshal(plaintext, &input); err != nil {
		return input, err
	}
	return input, nil
}

func (s *Server) submitWechatApplyment(ctx context.Context, tenantID int64) (provider.WeChatApplymentResult, error) {
	wechat := s.WeChatPay
	if wechat == nil {
		return provider.WeChatApplymentResult{}, errors.New("WeChat Pay partner provider is not active")
	}
	var app wechatOnboardingApplication
	var ciphertext, businessCode string
	err := s.DB.QueryRowContext(ctx, `SELECT subject_type,business_scene,merchant_short_name,service_phone,business_address,
		operator_name,contact_phone,contact_email,license_number,COALESCE(sensitive_ciphertext,''),COALESCE(business_code,'')
		FROM wechat_pay_onboarding_applications WHERE tenant_id=?`, tenantID).Scan(&app.SubjectType, &app.BusinessScene,
		&app.MerchantShortName, &app.ServicePhone, &app.BusinessAddress, &app.OperatorName, &app.ContactPhone,
		&app.ContactEmail, &app.LicenseNumber, &ciphertext, &businessCode)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	if businessCode == "" {
		businessCode = fmt.Sprintf("TB%08d%d", tenantID, time.Now().Unix())
	}
	sensitive, err := s.decryptWechatOnboardingSensitive(ctx, tenantID, ciphertext, app.SubjectType)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	encrypt := func(value string) (string, error) { return wechat.EncryptApplymentValue(ctx, value) }
	contactName, err := encrypt(app.OperatorName)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	contactPhone, err := encrypt(app.ContactPhone)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	contactEmail, err := encrypt(app.ContactEmail)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	idName, err := encrypt(sensitive.IDCardName)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	idNumber, err := encrypt(sensitive.IDCardNumber)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	idAddress, err := encrypt(sensitive.IDCardAddress)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	accountName, err := encrypt(sensitive.AccountName)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	accountNumber, err := encrypt(sensitive.AccountNumber)
	if err != nil {
		return provider.WeChatApplymentResult{}, err
	}
	subjectType := map[string]string{"MICRO": "SUBJECT_TYPE_MICRO", "INDIVIDUAL": "SUBJECT_TYPE_INDIVIDUAL", "ENTERPRISE": "SUBJECT_TYPE_ENTERPRISE"}[app.SubjectType]
	if subjectType == "" {
		return provider.WeChatApplymentResult{}, errors.New("unsupported WeChat onboarding subject type")
	}
	contactInfo := map[string]any{"contact_type": "LEGAL", "contact_name": contactName, "mobile_phone": contactPhone, "contact_email": contactEmail}
	idCardInfo := map[string]any{
		"id_card_copy": sensitive.IDCardCopy, "id_card_national": sensitive.IDCardNational, "id_card_name": idName, "id_card_number": idNumber,
		"id_card_address": idAddress, "card_period_begin": sensitive.CardPeriodBegin, "card_period_end": sensitive.CardPeriodEnd}
	identityInfo := map[string]any{"id_holder_type": "LEGAL", "id_doc_type": "IDENTIFICATION_TYPE_IDCARD", "id_card_info": idCardInfo}
	subjectInfo := map[string]any{"subject_type": subjectType, "identity_info": identityInfo}
	businessInfo := map[string]any{"merchant_shortname": app.MerchantShortName, "service_phone": app.ServicePhone}
	if app.SubjectType == "MICRO" {
		delete(contactInfo, "contact_type")
		contactInfo["contact_id_number"] = idNumber
		delete(identityInfo, "id_holder_type")
		delete(idCardInfo, "id_card_address")
		microBizInfo := map[string]any{}
		if app.BusinessScene == "MOBILE" {
			microBizInfo["micro_biz_type"] = "MICRO_TYPE_MOBILE"
			microBizInfo["micro_mobile_info"] = map[string]any{
				"micro_mobile_name": sensitive.StoreName, "micro_mobile_city": sensitive.StoreAddressCode,
				"micro_mobile_address": "无", "micro_mobile_pics": []string{sensitive.StoreEntrancePic, sensitive.IndoorPic},
			}
		} else {
			microBizInfo["micro_biz_type"] = "MICRO_TYPE_STORE"
			microBizInfo["micro_store_info"] = map[string]any{
				"micro_name": sensitive.StoreName, "micro_address_code": sensitive.StoreAddressCode, "micro_address": app.BusinessAddress,
				"store_entrance_pic": sensitive.StoreEntrancePic, "micro_indoor_copy": sensitive.IndoorPic,
			}
		}
		subjectInfo["micro_biz_info"] = microBizInfo
	} else {
		subjectInfo["business_license_info"] = map[string]any{"license_copy": sensitive.BusinessLicenseCopy, "license_number": app.LicenseNumber, "merchant_name": sensitive.MerchantName, "legal_person": sensitive.LegalPerson}
		indoorPics := []string{sensitive.IndoorPic}
		if sensitive.CashierPic != "" {
			indoorPics = append(indoorPics, sensitive.CashierPic)
		}
		businessInfo["sales_info"] = map[string]any{
			"sales_scenes_type": []string{"SALES_SCENES_STORE", "SALES_SCENES_MINI_PROGRAM"},
			"biz_store_info":    map[string]any{"biz_store_name": sensitive.StoreName, "biz_address_code": sensitive.StoreAddressCode, "biz_store_address": app.BusinessAddress, "store_entrance_pic": []string{sensitive.StoreEntrancePic}, "indoor_pic": indoorPics},
			"mini_program_info": map[string]any{"mini_program_appid": s.Config.WeChatPayPartner.ServiceProviderAppID, "mini_program_pics": sensitive.MiniProgramPics},
		}
	}
	bankAccountInfo := map[string]any{
		"bank_account_type": sensitive.AccountType,
		"account_name":      accountName,
		"account_bank":      sensitive.AccountBank,
		"account_number":    accountNumber,
	}
	if sensitive.BankAddressCode != "" {
		bankAccountInfo["bank_address_code"] = sensitive.BankAddressCode
	}
	if sensitive.BankBranchID != "" {
		bankAccountInfo["bank_branch_id"] = sensitive.BankBranchID
	}
	if sensitive.BankName != "" {
		bankAccountInfo["bank_name"] = sensitive.BankName
	}
	settlementInfo := map[string]any{"settlement_id": sensitive.SettlementID, "qualification_type": sensitive.QualificationType}
	if len(sensitive.QualificationPics) > 0 {
		settlementInfo["qualifications"] = sensitive.QualificationPics
	}
	payload := map[string]any{
		"business_code":     businessCode,
		"contact_info":      contactInfo,
		"subject_info":      subjectInfo,
		"business_info":     businessInfo,
		"settlement_info":   settlementInfo,
		"bank_account_info": bankAccountInfo,
	}
	result, err := wechat.SubmitApplyment(ctx, payload)
	if err != nil {
		return result, err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE wechat_pay_onboarding_applications SET business_code=?,wechat_applyment_id=?,
		wechat_applyment_state='APPLYMENT_STATE_AUDITING',application_status='SUBMITTED_TO_WECHAT',provider_submitted_at=NOW(3)
		WHERE tenant_id=?`, businessCode, result.ApplymentID, tenantID)
	return result, err
}

func (s *Server) syncWechatApplyment(ctx context.Context, tenantID int64) (provider.WeChatApplymentStatus, error) {
	wechat := s.WeChatPay
	if wechat == nil {
		return provider.WeChatApplymentStatus{}, errors.New("WeChat Pay partner provider is not active")
	}
	var applymentID string
	if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(wechat_applyment_id,'') FROM wechat_pay_onboarding_applications WHERE tenant_id=?", tenantID).Scan(&applymentID); err != nil {
		return provider.WeChatApplymentStatus{}, err
	}
	status, err := wechat.QueryApplyment(ctx, applymentID)
	if err != nil {
		return status, err
	}
	appStatus, tenantStatus := "SUBMITTED_TO_WECHAT", "REVIEWING"
	switch status.State {
	case "APPLYMENT_STATE_REJECTED":
		appStatus, tenantStatus = "NEEDS_INFO", "REJECTED"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED", "APPLYMENT_STATE_TO_BE_SIGNED", "APPLYMENT_STATE_SIGNING":
		tenantStatus = "PENDING_SIGNING"
	case "APPLYMENT_STATE_FINISHED":
		appStatus, tenantStatus = "FINISHED", "ACTIVE"
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return status, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE wechat_pay_onboarding_applications SET wechat_applyment_state=?,wechat_state_message=?,sign_url=?,application_status=? WHERE tenant_id=?`, status.State, status.StateMsg, status.SignURL, appStatus, tenantID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE tenants SET payment_onboarding_status=?,payment_merchant_no=IF(?<>'',?,payment_merchant_no),
			payment_product_authorization_status=IF(?='APPLYMENT_STATE_FINISHED','AUTHORIZED',payment_product_authorization_status)
			WHERE id=?`, tenantStatus, status.SubMchID, status.SubMchID, status.State, tenantID)
	}
	if err != nil {
		return status, err
	}
	return status, tx.Commit()
}

func (s *Server) syncMerchantWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	if _, err := s.syncWechatApplyment(r.Context(), actor.TenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "进件申请不存在")
			return
		}
		writeError(w, http.StatusBadGateway, "WECHAT_APPLYMENT_QUERY_FAILED", "查询微信支付进件状态失败")
		return
	}
	s.getMerchantWechatOnboarding(w, r)
}

func (s *Server) syncPlatformWechatOnboarding(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	status, err := s.syncWechatApplyment(r.Context(), tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "进件申请不存在")
			return
		}
		writeError(w, http.StatusBadGateway, "WECHAT_APPLYMENT_QUERY_FAILED", "查询微信支付进件状态失败")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "platform.wechat_onboarding.sync", "tenant", int64String(tenantID), map[string]any{"state": status.State, "sub_mchid": status.SubMchID != ""}, r)
	writeData(w, http.StatusOK, status)
}
