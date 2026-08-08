package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func completeWechatOnboardingApplication() wechatOnboardingApplication {
	return wechatOnboardingApplication{
		SubjectType:            "INDIVIDUAL",
		BusinessScene:          "STORE",
		MerchantShortName:      "码农咖啡",
		ServicePhone:           "13800000000",
		BusinessAddress:        "天津市和平区测试路1号",
		OperatorName:           "张三",
		ContactPhone:           "13800000000",
		ContactEmail:           "merchant@example.com",
		LicenseNumber:          "91120101MA00000000",
		QualificationConfirmed: true,
		IdentityMaterialReady:  true,
		SettlementAccountReady: true,
		BusinessMaterialReady:  true,
	}
}

func TestValidateWechatOnboardingAllowsMicroSubjectWithoutLicense(t *testing.T) {
	input := completeWechatOnboardingApplication()
	input.SubjectType = "MICRO"
	input.LicenseNumber = ""
	response := httptest.NewRecorder()

	if !validateWechatOnboarding(response, input, true) {
		t.Fatalf("expected micro subject without license to pass, body=%s", response.Body.String())
	}
}

func TestValidateWechatOnboardingRequiresLicenseForRegisteredSubject(t *testing.T) {
	input := completeWechatOnboardingApplication()
	input.LicenseNumber = ""
	response := httptest.NewRecorder()

	if validateWechatOnboarding(response, input, true) {
		t.Fatal("expected registered subject without license number to fail")
	}
	input.LicenseNumber = "91120101MA00000000"
	response = httptest.NewRecorder()
	if !validateWechatOnboarding(response, input, true) {
		t.Fatalf("expected complete individual application to pass, body=%s", response.Body.String())
	}
}

func TestNormalizeWechatOnboarding(t *testing.T) {
	input := completeWechatOnboardingApplication()
	input.SubjectType = " individual "
	input.BusinessScene = " mobile "
	input.MerchantShortName = " 码农咖啡 "
	normalizeWechatOnboarding(&input)
	if input.SubjectType != "INDIVIDUAL" || input.BusinessScene != "MOBILE" || input.MerchantShortName != "码农咖啡" {
		t.Fatalf("application was not normalized: %#v", input)
	}
}

func completeWechatOnboardingSensitiveInput() wechatOnboardingSensitiveInput {
	return wechatOnboardingSensitiveInput{
		IDCardName: "张三", IDCardNumber: "120101199001010000", IDCardAddress: "天津市和平区",
		CardPeriodBegin: "2020-01-01", CardPeriodEnd: "长期", IDCardCopy: "media-front", IDCardNational: "media-back",
		BusinessLicenseCopy: "media-license", MerchantName: "测试商户", LegalPerson: "张三",
		AccountType: "BANK_ACCOUNT_TYPE_PERSONAL", AccountName: "张三", AccountNumber: "6222000000000000",
		AccountBank: "工商银行", BankAddressCode: "120000", StoreName: "张三小吃摊", StoreAddressCode: "120100",
		StoreEntrancePic: "media-scene", IndoorPic: "media-goods", MiniProgramPics: []string{"media-miniapp"},
		CashierPic: "media-cashier", SettlementID: "719", QualificationType: "餐饮",
	}
}

func TestValidateWechatOnboardingSensitiveSupportsMicroWithoutLicense(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.BusinessLicenseCopy = ""
	input.MerchantName = ""
	input.LegalPerson = ""
	input.MiniProgramPics = nil
	input.SettlementID = "703"
	if err := validateWechatOnboardingSensitive(input, "MICRO"); err != nil {
		t.Fatalf("expected valid micro sensitive input, got %v", err)
	}
	input.AccountType = "BANK_ACCOUNT_TYPE_CORPORATE"
	if err := validateWechatOnboardingSensitive(input, "MICRO"); err == nil {
		t.Fatal("expected corporate account to be rejected for micro merchant")
	}
}

func TestWechatOnboardingIndustryRulesMatchSubjectAndQualificationRequirements(t *testing.T) {
	for subjectType, settlementID := range map[string]string{"MICRO": "703", "INDIVIDUAL": "719", "ENTERPRISE": "716"} {
		if got := expectedWechatSettlementID(subjectType); got != settlementID {
			t.Fatalf("expected %s settlement ID %s, got %s", subjectType, settlementID, got)
		}
	}
	restaurant, ok := selectedOnboardingIndustryRule("INDIVIDUAL", "719", "餐饮")
	if !ok || restaurant.QualificationMode != "ALTERNATIVE" {
		t.Fatalf("expected individual restaurant alternative qualification rule, got %#v", restaurant)
	}
	retail, ok := selectedOnboardingIndustryRule("INDIVIDUAL", "719", "零售批发/生活娱乐/其他")
	if !ok || retail.QualificationMode != "NONE" {
		t.Fatalf("expected retail to omit qualifications, got %#v", retail)
	}
	if _, ok = selectedOnboardingIndustryRule("INDIVIDUAL", "716", "餐饮"); ok {
		t.Fatal("enterprise settlement ID must not be accepted for an individual merchant")
	}
}

func TestValidateWechatOnboardingSensitiveRequiresIndustrySpecificQualification(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.QualificationType = "私立/民营医院/诊所"
	input.QualificationPics = nil
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err == nil || !strings.Contains(err.Error(), "必须上传") {
		t.Fatalf("expected medical qualification to be required, got %v", err)
	}
	input.QualificationPics = []string{"media-medical-license"}
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err != nil {
		t.Fatalf("expected medical qualification to pass after upload, got %v", err)
	}
}

func TestValidateWechatOnboardingAdminUpdateRejectsMismatchedSettlementRule(t *testing.T) {
	input := wechatOnboardingAdminUpdateInput{Application: completeWechatOnboardingApplication(), Sensitive: completeWechatOnboardingSensitiveInput()}
	input.Sensitive.SettlementID = "716"
	if err := validateWechatOnboardingAdminUpdate(input); err == nil || !strings.Contains(err.Error(), "719") {
		t.Fatalf("expected individual settlement mismatch, got %v", err)
	}
}

func TestBuildWechatOnboardingSettlementInfoOmitsUnneededQualifications(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.QualificationType = "零售批发/生活娱乐/其他"
	input.QualificationPics = []string{"media-that-must-not-be-sent"}
	settlement, err := buildWechatOnboardingSettlementInfo("INDIVIDUAL", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := settlement["qualifications"]; exists {
		t.Fatalf("qualifications must be omitted for an industry that does not require them: %#v", settlement)
	}
}

func TestBuildWechatOnboardingSettlementInfoUsesRestaurantScenePhotoAlternative(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.QualificationPics = nil
	settlement, err := buildWechatOnboardingSettlementInfo("INDIVIDUAL", input)
	if err != nil {
		t.Fatal(err)
	}
	qualifications, ok := settlement["qualifications"].([]string)
	if !ok || !slices.Equal(qualifications, []string{"media-scene", "media-goods", "media-cashier"}) {
		t.Fatalf("expected restaurant scene-photo alternative, got %#v", settlement["qualifications"])
	}
}

func TestValidateWechatOnboardingSensitiveStillRequiresRegisteredBusinessLicense(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.BusinessLicenseCopy = ""
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err == nil {
		t.Fatal("expected registered merchant license image to remain required")
	}
}

func TestValidateWechatOnboardingSensitiveRequiresCorporateAccountForEnterprise(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.SettlementID = "716"
	input.AccountType = "BANK_ACCOUNT_TYPE_PERSONAL"
	if err := validateWechatOnboardingSensitive(input, "ENTERPRISE"); err == nil || !strings.Contains(err.Error(), "对公") {
		t.Fatalf("expected enterprise personal account to be rejected, got %v", err)
	}
	input.AccountType = "BANK_ACCOUNT_TYPE_CORPORATE"
	if err := validateWechatOnboardingSensitive(input, "ENTERPRISE"); err != nil {
		t.Fatalf("expected enterprise corporate account to pass, got %v", err)
	}
}

func TestValidateWechatOnboardingSensitiveBankFields(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.BankAddressCode = ""
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err != nil {
		t.Fatalf("bank address code is optional, got %v", err)
	}

	input.BankAddressCode = "0302"
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err == nil {
		t.Fatal("expected invalid four-digit bank address code to be rejected")
	}

	input.BankAddressCode = "120118"
	input.AccountBank = "其他银行"
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err == nil {
		t.Fatal("expected other bank without branch details to be rejected")
	}
	input.BankName = "天津银行股份有限公司测试支行"
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err != nil {
		t.Fatalf("expected other bank with a branch name to be accepted, got %v", err)
	}
}

func completeWechatOnboardingReviewMedia() map[string]onboardingReviewMedia {
	mediaIDs := map[string]string{
		"businessLicenseCopy": "media-license", "idCardCopy": "media-front", "idCardNational": "media-back",
		"storeEntrancePic": "media-scene", "indoorPic": "media-goods", "cashierPic": "media-cashier", "miniProgramPic": "media-miniapp",
	}
	result := map[string]onboardingReviewMedia{}
	for field, mediaID := range mediaIDs {
		result[field] = onboardingReviewMedia{FieldName: field, DataURL: "data:image/png;base64,dGVzdA==", WechatSet: true, WechatMediaID: mediaID}
	}
	return result
}

func TestMissingWechatOnboardingReviewItemsAcceptsCompleteApplication(t *testing.T) {
	missing := missingWechatOnboardingReviewItems(
		completeWechatOnboardingApplication(),
		completeWechatOnboardingSensitiveInput(),
		completeWechatOnboardingReviewMedia(),
		true,
	)
	if len(missing) != 0 {
		t.Fatalf("expected complete review material, missing=%v", missing)
	}
}

func TestMissingWechatOnboardingReviewItemsRequiresReviewableImageCopy(t *testing.T) {
	media := completeWechatOnboardingReviewMedia()
	delete(media, "idCardCopy")
	missing := missingWechatOnboardingReviewItems(
		completeWechatOnboardingApplication(),
		completeWechatOnboardingSensitiveInput(),
		media,
		true,
	)
	if !slices.Contains(missing, "身份证人像面（需重新上传审核副本）") {
		t.Fatalf("expected missing review copy to block approval, missing=%v", missing)
	}
}

func TestMissingWechatOnboardingReviewItemsReportsEveryEmptySensitiveField(t *testing.T) {
	missing := missingWechatOnboardingReviewItems(
		completeWechatOnboardingApplication(),
		wechatOnboardingSensitiveInput{},
		map[string]onboardingReviewMedia{},
		true,
	)
	for _, expected := range []string{"身份证姓名", "身份证号码", "结算账户名称", "银行账号", "门店名称", "结算规则 ID"} {
		if !slices.Contains(missing, expected) {
			t.Fatalf("expected %q in missing items, got %v", expected, missing)
		}
	}
}

func TestApproveWechatOnboardingRequiresExplicitMaterialConfirmation(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/platform/tenants/5/wechat-onboarding/review", strings.NewReader(`{"action":"approve"}`))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("tenantID", "5")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	(&Server{}).reviewWechatOnboarding(recorder, request)

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "MATERIAL_REVIEW_REQUIRED") {
		t.Fatalf("expected review confirmation to be required before database or provider calls, status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
