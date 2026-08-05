package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testWechatV2Key = "12345678901234567890123456789012"

func TestWeChatPayPartnerPayCodeSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wechatMicropayPath {
			t.Fatalf("path=%q", r.URL.Path)
		}
		request, err := parseWeChatXMLReader(r)
		if err != nil {
			t.Fatal(err)
		}
		if request["auth_code"] != "101234567890123456" || request["sub_mch_id"] != "sub-merchant" {
			t.Fatalf("unexpected request: %+v", request)
		}
		if request["sign"] != signWeChatV2(request, testWechatV2Key) {
			t.Fatal("request signature mismatch")
		}
		response := map[string]string{
			"return_code": "SUCCESS", "result_code": "SUCCESS", "trade_type": "MICROPAY",
			"transaction_id": "4200000000001", "out_trade_no": request["out_trade_no"],
			"total_fee": request["total_fee"], "time_end": "20260726150506",
		}
		response["sign"] = signWeChatV2(response, testWechatV2Key)
		_, _ = w.Write(marshalWeChatXML(response))
	}))
	defer server.Close()

	adapter := configuredWechatV2Adapter(server)
	result, err := adapter.PayCode(context.Background(), CodePaymentRequest{
		MerchantNo: "sub-merchant", OrderNo: "MP202607260001", Amount: 1688,
		AuthCode: "101234567890123456", Description: "摊伴测试订单",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != PaymentSuccess || result.ProviderOrderNo != "MP202607260001" ||
		result.ProviderTransactionNo != "4200000000001" || result.PaidAt == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestWeChatPayPartnerPayCodeUserPayingAndQuery(t *testing.T) {
	t.Parallel()
	queryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := parseWeChatXMLReader(r)
		if err != nil {
			t.Fatal(err)
		}
		response := map[string]string{"return_code": "SUCCESS", "result_code": "FAIL"}
		if r.URL.Path == wechatMicropayPath {
			response["err_code"] = "USERPAYING"
			response["err_code_des"] = "需要输入密码"
		} else {
			queryCount++
			response["result_code"] = "SUCCESS"
			response["trade_state"] = "SUCCESS"
			response["transaction_id"] = "4200000000002"
			response["out_trade_no"] = request["out_trade_no"]
			response["total_fee"] = "1688"
		}
		response["sign"] = signWeChatV2(response, testWechatV2Key)
		_, _ = w.Write(marshalWeChatXML(response))
	}))
	defer server.Close()

	adapter := configuredWechatV2Adapter(server)
	payment, err := adapter.PayCode(context.Background(), CodePaymentRequest{
		MerchantNo: "sub-merchant", OrderNo: "MP202607260002", Amount: 1688,
		AuthCode: "111234567890123456", Description: "摊伴测试订单",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payment.Status != PaymentPending || !payment.NeedCustomerAction {
		t.Fatalf("unexpected pending result: %+v", payment)
	}
	queried, err := adapter.QueryCode(context.Background(), QueryCodePaymentRequest{
		MerchantNo: "sub-merchant", OrderNo: "MP202607260002",
	})
	if err != nil {
		t.Fatal(err)
	}
	if queryCount != 1 || queried.Status != PaymentSuccess || queried.ProviderTransactionNo != "4200000000002" {
		t.Fatalf("unexpected query result: %+v count=%d", queried, queryCount)
	}
}

func TestWeChatPayPartnerRejectsInvalidAuthCodeBeforeTransport(t *testing.T) {
	t.Parallel()
	adapter := configuredWechatV2Adapter(nil)
	_, err := adapter.PayCode(context.Background(), CodePaymentRequest{
		MerchantNo: "sub-merchant", OrderNo: "MP202607260003", Amount: 1688,
		AuthCode: "not-a-payment-code",
	})
	if err == nil {
		t.Fatal("expected invalid auth code error")
	}
}

func configuredWechatV2Adapter(server *httptest.Server) WeChatPayPartner {
	baseURL := "http://127.0.0.1"
	var client *http.Client
	if server != nil {
		baseURL = server.URL
		client = server.Client()
	}
	return WeChatPayPartner{
		Config: WeChatPayPartnerConfig{
			BaseURL: baseURL, ServiceProviderMchID: "service-mch", ServiceProviderAppID: "wx-service",
			APIV2Key: testWechatV2Key, MerchantCertificate: "configured",
			MerchantPrivateKey: "configured", ServerIP: "203.0.113.10",
		},
		HTTPClient: client,
	}
}

func parseWeChatXMLReader(r *http.Request) (map[string]string, error) {
	defer r.Body.Close()
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return parseWeChatXML(payload)
}
