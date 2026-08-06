package app

import (
	"net/http/httptest"
	"testing"
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
		SettlementID: "716", QualificationType: "餐饮",
	}
}

func TestValidateWechatOnboardingSensitiveSupportsMicroWithoutLicense(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.BusinessLicenseCopy = ""
	input.MerchantName = ""
	input.LegalPerson = ""
	input.MiniProgramPics = nil
	if err := validateWechatOnboardingSensitive(input, "MICRO"); err != nil {
		t.Fatalf("expected valid micro sensitive input, got %v", err)
	}
	input.AccountType = "BANK_ACCOUNT_TYPE_CORPORATE"
	if err := validateWechatOnboardingSensitive(input, "MICRO"); err == nil {
		t.Fatal("expected corporate account to be rejected for micro merchant")
	}
}

func TestValidateWechatOnboardingSensitiveStillRequiresRegisteredBusinessLicense(t *testing.T) {
	input := completeWechatOnboardingSensitiveInput()
	input.BusinessLicenseCopy = ""
	if err := validateWechatOnboardingSensitive(input, "INDIVIDUAL"); err == nil {
		t.Fatal("expected registered merchant license image to remain required")
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
