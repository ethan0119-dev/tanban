package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestValidateOperationSettingsRequiresCoordinatesForDistanceCheck(t *testing.T) {
	input := storeOperationSettings{
		SettlementMode:               "PAY_BEFORE",
		OrderingMode:                 "MULTI_PERSON",
		PayBeforeClearMode:           "AFTER_ORDER_COMPLETION",
		DistanceCheckEnabled:         true,
		DistanceLimitM:               1000,
		OrderReminderIntervalMinutes: 5,
	}
	if err := validateOperationSettings(input); err == nil {
		t.Fatal("distance validation must fail closed without store coordinates")
	}
	latitude, longitude := 39.9042, 116.4074
	input.StoreLatitude, input.StoreLongitude = &latitude, &longitude
	if err := validateOperationSettings(input); err != nil {
		t.Fatalf("valid distance configuration rejected: %v", err)
	}
}

func TestValidateOperationSettingsAcceptsPayAfterMeal(t *testing.T) {
	input := storeOperationSettings{
		SettlementMode:               "PAY_AFTER",
		OrderingMode:                 "MULTI_PERSON",
		PayBeforeClearMode:           "AFTER_ORDER_COMPLETION",
		DistanceLimitM:               1000,
		OrderReminderIntervalMinutes: 5,
	}
	if err := validateOperationSettings(input); err != nil {
		t.Fatalf("pay-after-meal workflow should be available: %v", err)
	}
}

func TestSettlementPrintTriggerFollowsSettlementMode(t *testing.T) {
	if got := settlementPrintTrigger("PAY_AFTER"); got != "ORDER_CREATED" {
		t.Fatalf("pay-after mode must print when the order is submitted, got %s", got)
	}
	if got := settlementPrintTrigger("PAY_BEFORE"); got != "PAYMENT_SUCCESS" {
		t.Fatalf("pay-before mode must print after payment, got %s", got)
	}
}

func TestTableBoardStateKeepsPaymentAndTableLifecycleDistinct(t *testing.T) {
	tests := []struct {
		orderStatus, paymentStatus, settlementMode, expected string
	}{
		{"", "", "", "UNOPENED"},
		{"PENDING_PAYMENT", "UNPAID", "PAY_BEFORE", "PENDING_PAYMENT"},
		{"PAID", "PAID", "PAY_BEFORE", "SETTLED"},
		{"PREPARING", "PAID", "PAY_BEFORE", "DINING"},
		{"READY", "PAID", "PAY_BEFORE", "READY"},
		{"PAID", "UNPAID", "PAY_AFTER", "UNSETTLED"},
		{"READY", "UNPAID", "PAY_AFTER", "UNSETTLED"},
	}
	for _, test := range tests {
		if actual := tableBoardState(test.orderStatus, test.paymentStatus, test.settlementMode); actual != test.expected {
			t.Fatalf("tableBoardState(%q,%q,%q)=%q want %q", test.orderStatus, test.paymentStatus, test.settlementMode, actual, test.expected)
		}
	}
}

func TestApplyOperationSettingsDefaultsRestoresDistanceWhenDisabled(t *testing.T) {
	input := storeOperationSettings{DistanceCheckEnabled: false}
	applyOperationSettingsDefaults(&input)
	if input.DistanceLimitM != 5000 {
		t.Fatalf("expected disabled distance check to retain a safe value, got %d", input.DistanceLimitM)
	}

	input.DistanceLimitM = -1
	applyOperationSettingsDefaults(&input)
	if input.DistanceLimitM != -1 {
		t.Fatal("explicit invalid values must still reach validation")
	}
}

func TestDistanceMeters(t *testing.T) {
	distance := distanceMeters(39.9042, 116.4074, 39.9051, 116.4074)
	if distance < 95 || distance > 105 {
		t.Fatalf("expected roughly 100m, got %.2fm", distance)
	}
}

func TestMerchantTableBoardUsesStablePublicScene(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id FROM stores WHERE tenant_id=").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec("INSERT IGNORE INTO store_operation_settings").
		WithArgs(int64(11), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT store_id,settlement_mode,ordering_mode").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"store_id", "settlement_mode", "ordering_mode", "table_scan_return_home", "pay_before_clear_mode",
			"pay_after_online_payment_enabled", "distance_check_enabled", "distance_limit_m",
			"store_latitude", "store_longitude", "require_customer_phone", "allow_order_remark", "allow_item_remark",
			"order_reminder_enabled", "order_reminder_interval_minutes", "takeaway_verification_enabled",
			"reviews_enabled", "customer_service_phone", "customer_service_wechat", "customer_service_qr_url",
			"privacy_policy_text", "user_agreement_text", "official_account_notify_enabled",
			"official_account_events_json", "notification_recipient_label",
		}).AddRow(
			11, "PAY_AFTER", "MULTI_PERSON", 1, "AFTER_ORDER_COMPLETION", 1, 0, 5000,
			nil, nil, 0, 1, 1, 1, 5, 1,
			1, "", "", "", "", "", 0, "[]", "",
		))
	mock.ExpectQuery(`SELECT t\.id,t\.public_scene,t\.area_id`).
		WithArgs("AFTER_ORDER_COMPLETION", int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_scene", "area_id", "area_name", "name", "table_code", "capacity",
			"order_id", "order_no", "order_status", "payment_status", "settlement_mode",
			"addition_count", "diner_count", "customer_name", "total_cents", "paid_cents", "payment_locked", "opened_at",
		}).AddRow(
			3, "0123456789abcdef0123456789ab", 2, "大厅", "A01", "A01", 4,
			0, "", "", "", "", 0, 0, "", 0, 0, 0, "",
		))

	server := &Server{DB: db, Logger: slog.Default()}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/merchant/table-board", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{TenantID: 7}))
	response := httptest.NewRecorder()

	server.getMerchantTableBoard(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"publicId":"0123456789abcdef0123456789ab"`) {
		t.Fatalf("table public scene missing from response: %s", response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
