package app

import (
	"net/http/httptest"
	"testing"
)

func completeWechatOnboardingApplication() wechatOnboardingApplication {
	return wechatOnboardingApplication{
		SubjectType:            "MICRO",
		BusinessScene:          "STORE",
		MerchantShortName:      "码农咖啡",
		ServicePhone:           "13800000000",
		BusinessAddress:        "天津市和平区测试路1号",
		OperatorName:           "张三",
		ContactPhone:           "13800000000",
		QualificationConfirmed: true,
		IdentityMaterialReady:  true,
		SettlementAccountReady: true,
		BusinessMaterialReady:  true,
	}
}

func TestValidateWechatOnboardingRequiresMicroQualification(t *testing.T) {
	input := completeWechatOnboardingApplication()
	input.QualificationConfirmed = false
	response := httptest.NewRecorder()

	if validateWechatOnboarding(response, input, true) {
		t.Fatal("expected micro qualification validation to fail")
	}
	if response.Code != 400 {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestValidateWechatOnboardingRequiresLicenseForRegisteredSubject(t *testing.T) {
	input := completeWechatOnboardingApplication()
	input.SubjectType = "INDIVIDUAL"
	input.QualificationConfirmed = false
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
	input.SubjectType = " micro "
	input.BusinessScene = " mobile "
	input.MerchantShortName = " 码农咖啡 "
	normalizeWechatOnboarding(&input)
	if input.SubjectType != "MICRO" || input.BusinessScene != "MOBILE" || input.MerchantShortName != "码农咖啡" {
		t.Fatalf("application was not normalized: %#v", input)
	}
}
