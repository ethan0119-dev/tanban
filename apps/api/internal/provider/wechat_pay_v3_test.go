package provider

import (
	"strings"
	"testing"
)

func TestDecodeWechatApplymentResultAcceptsNumericID(t *testing.T) {
	result, err := decodeWechatApplymentResult(strings.NewReader(`{"applyment_id":2000002124775691}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ApplymentID != "2000002124775691" {
		t.Fatalf("unexpected applyment ID %q", result.ApplymentID)
	}
}

func TestDecodeWechatApplymentStatusAcceptsNumericID(t *testing.T) {
	status, err := decodeWechatApplymentStatus(strings.NewReader(`{
		"applyment_id":2000002124775691,
		"applyment_state":"APPLYMENT_STATE_TO_BE_SIGNED",
		"applyment_state_msg":"待签约",
		"sub_mchid":"1900000109",
		"sign_url":"https://example.test/sign"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if status.ApplymentID != "2000002124775691" || status.SubMchID != "1900000109" || status.SignURL == "" {
		t.Fatalf("unexpected applyment status %#v", status)
	}
}

func TestDecodeWechatApplymentResultRejectsMissingID(t *testing.T) {
	if _, err := decodeWechatApplymentResult(strings.NewReader(`{}`)); err == nil {
		t.Fatal("expected missing applyment_id to be rejected")
	}
}
