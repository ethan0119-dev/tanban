package app

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeOrderTypeSupportsCanonicalAndLegacyNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		orderType, scene, fulfillment, expected string
	}{
		{"", "", "", orderTypeTakeout},
		{"", "", "PICKUP", orderTypeTakeout},
		{"", "DINE_IN", "", orderTypeDineIn},
		{"DELIVERY", "", "", orderTypeDelivery},
	}
	for _, test := range tests {
		actual, err := normalizeOrderType(test.orderType, test.scene, test.fulfillment)
		if err != nil || actual != test.expected {
			t.Fatalf("normalize (%q,%q,%q) = %q,%v; want %q", test.orderType, test.scene, test.fulfillment, actual, err, test.expected)
		}
	}
	if _, err := normalizeOrderType("CURBSIDE", "", ""); err == nil {
		t.Fatal("unknown order type must be rejected")
	}
}

func TestSettlementModeOnlyAppliesPayAfterToDineIn(t *testing.T) {
	if actual := settlementModeForOrder(orderTypeDineIn, "PAY_AFTER"); actual != "PAY_AFTER" {
		t.Fatalf("dine-in should inherit pay-after, got %s", actual)
	}
	for _, orderType := range []string{orderTypeTakeout, orderTypeDelivery} {
		if actual := settlementModeForOrder(orderType, "PAY_AFTER"); actual != "PAY_BEFORE" {
			t.Fatalf("%s must remain pay-before, got %s", orderType, actual)
		}
	}
}

func TestTablePublicIDFitsPrefixedWeChatScene(t *testing.T) {
	t.Parallel()
	publicID, err := newTablePublicID()
	if err != nil {
		t.Fatal(err)
	}
	if len(publicID) != 28 || len("tc="+publicID) > 32 {
		t.Fatalf("public id %q does not fit the WeChat scene limit", publicID)
	}
	if _, err = hex.DecodeString(publicID); err != nil {
		t.Fatalf("public id must be URL-safe hexadecimal: %v", err)
	}
}

func TestResolveOrderTableScopesPublicIDToTenantAndStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `SELECT c\.id,c\.public_scene,a\.name,c\.name,c\.table_code[\s\S]+WHERE c\.public_scene=\? AND c\.tenant_id=\? AND c\.store_id=\?`
	mock.ExpectQuery(query).WithArgs("0123456789abcdef0123456789ab", int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_scene", "area", "name", "table_code"}).
			AddRow(19, "0123456789abcdef0123456789ab", "一层", "A01", "A01"))
	item, err := resolveOrderTable(context.Background(), db, 7, 11, "0123456789abcdef0123456789ab")
	if err != nil || item.ID != 19 || item.AreaName != "一层" {
		t.Fatalf("resolve table: item=%+v err=%v", item, err)
	}
	mock.ExpectQuery(query).WithArgs("0123456789abcdef0123456789ab", int64(7), int64(12)).
		WillReturnError(sql.ErrNoRows)
	_, err = resolveOrderTable(context.Background(), db, 7, 12, "0123456789abcdef0123456789ab")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-store public id must not resolve, got %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublicOrderViewIncludesDineInSnapshot(t *testing.T) {
	t.Parallel()
	view := publicOrderView(orderDTO{
		ID: 1, OrderType: orderTypeDineIn, Fulfillment: orderTypeDineIn,
		Table: &orderTableDTO{ID: 3, PublicID: "0123456789abcdef0123456789ab", AreaName: "大厅", Name: "B02", TableCode: "B02"},
	})
	for key, expected := range map[string]any{"orderScene": orderTypeDineIn, "tablePublicId": "0123456789abcdef0123456789ab", "tableName": "B02", "tableAreaName": "大厅"} {
		if actual := view[key]; actual != expected {
			t.Fatalf("%s=%v want %v", key, actual, expected)
		}
	}
}

func TestPartialSettlementDisablesAdditionsAndExposesRemainingAmount(t *testing.T) {
	t.Parallel()
	view := publicOrderView(orderDTO{
		ID:             8,
		OrderType:      orderTypeDineIn,
		Fulfillment:    orderTypeDineIn,
		SettlementMode: "PAY_AFTER",
		Status:         "PREPARING",
		PaymentStatus:  "UNPAID",
		TotalCents:     3600,
		PaidCents:      1200,
		RemainingCents: 2400,
	})
	if view["canAddItems"] != false {
		t.Fatal("a partially settled order must not accept additional dishes")
	}
	if view["paidAmount"] != int64(1200) || view["remainingAmount"] != int64(2400) {
		t.Fatalf("unexpected partial settlement view: %#v", view)
	}
}

func TestDineInOperationsRequireUnpaidPayAfterMealOrder(t *testing.T) {
	t.Parallel()
	if !validDineInOperationOrder(orderTypeDineIn, "PAY_AFTER", "UNPAID", "READY", 0) {
		t.Fatal("valid pay-after-meal order should allow dine-in operations")
	}
	for _, test := range []struct {
		orderType, settlement, payment, status string
		paid                                   int64
	}{
		{orderTypeTakeout, "PAY_AFTER", "UNPAID", "READY", 0},
		{orderTypeDineIn, "PAY_BEFORE", "UNPAID", "READY", 0},
		{orderTypeDineIn, "PAY_AFTER", "PAID", "READY", 0},
		{orderTypeDineIn, "PAY_AFTER", "UNPAID", "READY", 1},
		{orderTypeDineIn, "PAY_AFTER", "UNPAID", "CLOSED", 0},
	} {
		if validDineInOperationOrder(test.orderType, test.settlement, test.payment, test.status, test.paid) {
			t.Fatalf("invalid operation state was accepted: %+v", test)
		}
	}
}

func TestDineInItemChangesCoverBothSettlementModesBeforeCollection(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"PAID", "ACCEPTED", "PREPARING", "READY"} {
		if !validDineInItemChangeOrder(orderTypeDineIn, "PAY_AFTER", "UNPAID", status, 0) {
			t.Fatalf("pay-after order in %s should allow item changes", status)
		}
	}
	if !validDineInItemChangeOrder(orderTypeDineIn, "PAY_BEFORE", "UNPAID", "PENDING_PAYMENT", 0) {
		t.Fatal("unpaid pay-before order should allow item changes before payment starts")
	}
	for _, test := range []struct {
		orderType, settlement, payment, status string
		paid                                   int64
	}{
		{orderTypeTakeout, "PAY_BEFORE", "UNPAID", "PENDING_PAYMENT", 0},
		{orderTypeDineIn, "PAY_BEFORE", "PAID", "PAID", 0},
		{orderTypeDineIn, "PAY_AFTER", "UNPAID", "READY", 1},
		{orderTypeDineIn, "PAY_AFTER", "UNPAID", "COMPLETED", 0},
	} {
		if validDineInItemChangeOrder(test.orderType, test.settlement, test.payment, test.status, test.paid) {
			t.Fatalf("invalid item-change state was accepted: %+v", test)
		}
	}
}

func TestCanChangeDineInItemsRejectsActivePaymentTransaction(t *testing.T) {
	t.Parallel()
	base := orderDTO{
		OrderType: orderTypeDineIn, SettlementMode: "PAY_AFTER", PaymentStatus: "UNPAID",
		Status: "PREPARING",
	}
	if !canChangeDineInItems(base) {
		t.Fatal("unpaid pay-after order should be editable before payment starts")
	}
	for _, status := range []string{"CREATING", "PENDING", "SUCCESS"} {
		item := base
		item.Payment = map[string]any{"status": status}
		if canChangeDineInItems(item) {
			t.Fatalf("payment transaction %s must lock item changes", status)
		}
	}
}

func TestMergedDineInOrderReturnsToTheEarlierProductionStage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		left, right, expected string
	}{
		{"READY", "PREPARING", "PREPARING"},
		{"PAID", "READY", "PAID"},
		{"ACCEPTED", "ACCEPTED", "ACCEPTED"},
	}
	for _, test := range tests {
		if actual := earlierDineInStatus(test.left, test.right); actual != test.expected {
			t.Fatalf("earlierDineInStatus(%s,%s)=%s want %s", test.left, test.right, actual, test.expected)
		}
	}
}
