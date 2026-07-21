package provider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXPrinterStatusMapsProviderStates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		data int
		want string
	}{{"offline", 0, "OFFLINE"}, {"online", 1, "ONLINE"}, {"abnormal", 2, "PAPER_OUT"}} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/queryPrinterStatus" {
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				timestamp := payload["timestamp"].(string)
				hash := sha1.Sum([]byte("developer" + "secret" + timestamp))
				if payload["sign"] != hex.EncodeToString(hash[:]) || payload["sn"] != "SN-1" {
					t.Fatalf("unexpected signed payload: %#v", payload)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": test.data})
			}))
			defer server.Close()

			printer := NewXPrinter(XPrinterConfig{BaseURL: server.URL, User: "developer", UserKey: "secret", Client: server.Client()})
			result := printer.Status(context.Background(), PrinterStatusRequest{DeviceSN: "SN-1"})
			if result.Status != test.want || result.CheckedAt.IsZero() {
				t.Fatalf("unexpected status result: %+v", result)
			}
		})
	}
}

func TestXPrinterStatusDistinguishesConfigurationAndRegistrationErrors(t *testing.T) {
	t.Parallel()
	unconfigured := NewXPrinter(XPrinterConfig{}).Status(context.Background(), PrinterStatusRequest{DeviceSN: "SN-1"})
	if unconfigured.Status != "UNREACHABLE" || !strings.Contains(unconfigured.Message, "UserKEY") {
		t.Fatalf("unexpected unconfigured result: %+v", unconfigured)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1002, "msg": "PRINTER_NOT_REGISTER", "data": 0})
	}))
	defer server.Close()
	printer := NewXPrinter(XPrinterConfig{BaseURL: server.URL, User: "developer", UserKey: "secret", Client: server.Client()})
	result := printer.Status(context.Background(), PrinterStatusRequest{DeviceSN: "SN-1"})
	if result.Status != "UNREACHABLE" || !strings.Contains(result.Message, "尚未绑定") {
		t.Fatalf("unexpected registration result: %+v", result)
	}
}

func TestPrinterRouterUsesDeviceProviderAndPrintEndpoint(t *testing.T) {
	t.Parallel()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": "ORDER-1"})
	}))
	defer server.Close()
	router := NewPrinterRouter("mock", nil, NewXPrinter(XPrinterConfig{BaseURL: server.URL, User: "developer", UserKey: "secret", Client: server.Client()}))

	result, err := router.Print(context.Background(), PrintRequest{Provider: "芯烨", DeviceSN: "SN-1", OutputType: "RECEIPT", Content: "test"})
	if err != nil || result.ProviderJobNo != "ORDER-1" || path != "/print" {
		t.Fatalf("unexpected receipt print result=%+v path=%s err=%v", result, path, err)
	}
	result, err = router.Print(context.Background(), PrintRequest{Provider: "xpyun", DeviceSN: "SN-2", OutputType: "LABEL", Content: "<SIZE>50,30</SIZE><TEXT>test</TEXT>"})
	if err != nil || result.ProviderJobNo != "ORDER-1" || path != "/printLabel" {
		t.Fatalf("unexpected label print result=%+v path=%s err=%v", result, path, err)
	}
}
