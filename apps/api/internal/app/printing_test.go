package app

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRenderTicketIncludesProductConfiguration(t *testing.T) {
	order := orderDTO{OrderNo: "TB1", OrderType: orderTypeDineIn, TotalCents: 1800, Table: &orderTableDTO{Name: "B02", AreaName: "大厅", TableCode: "B02"}, Items: []orderItemDTO{{
		ProductName: "拿铁", SKUName: "大杯", Quantity: 1, ItemRemark: "奶泡少一点",
		Configuration: map[string]any{
			"options":   []any{map[string]any{"groupName": "温度", "valueName": "冰"}},
			"modifiers": []any{map[string]any{"name": "浓缩", "quantity": float64(2)}},
		},
	}}}
	result := renderTicket("{{order_type}} {{table_area}} {{table_name}} {{table_code}}\n{{items}}", order, "", false)
	for _, expected := range []string{"DINE_IN 大厅 B02 B02", "拿铁 大杯 x1", "温度：冰", "加料：浓缩x2", "单品备注：奶泡少一点"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("ticket missing %q: %s", expected, result)
		}
	}
}

func TestPrintPlanRespectsTemplateTriggerCopiesAndEnabled(t *testing.T) {
	t.Parallel()
	device := printerDTO{PrintTrigger: "ORDER_CREATED", TemplateText: "legacy"}
	template := activePrintTemplate{Found: true, Enabled: true, Content: "dine-in", TriggerEvent: "PAYMENT_SUCCESS", Copies: 2}
	if _, _, ok := resolvePrintPlan(device, template, "ORDER_CREATED"); ok {
		t.Fatal("template trigger mismatch must not print")
	}
	content, copies, ok := resolvePrintPlan(device, template, "PAYMENT_SUCCESS")
	if !ok || content != "dine-in" || copies != 2 {
		t.Fatalf("unexpected print plan content=%q copies=%d ok=%v", content, copies, ok)
	}
	template.Enabled = false
	if _, _, ok = resolvePrintPlan(device, template, "PAYMENT_SUCCESS"); ok {
		t.Fatal("disabled template must not fall back to the legacy device template")
	}
	content, copies, ok = resolvePrintPlan(device, activePrintTemplate{}, "ORDER_CREATED")
	if !ok || content != "legacy" || copies != 1 {
		t.Fatal("a store without business templates must retain the legacy device behavior")
	}
}

func TestStorePrintPolicyGatesAutomaticJobsButNotManualReprints(t *testing.T) {
	t.Parallel()
	policy := storePrintPolicy{AutoReceipt: false, AutoLabel: true}
	if storePolicyAllowsPrint(policy, "RECEIPT", "PAYMENT_SUCCESS") {
		t.Fatal("disabled receipt switch must block automatic receipt jobs")
	}
	if !storePolicyAllowsPrint(policy, "LABEL", "PAYMENT_SUCCESS") {
		t.Fatal("enabled label switch must allow automatic label jobs")
	}
	if !storePolicyAllowsPrint(policy, "RECEIPT", "REPRINT") {
		t.Fatal("an explicit manual reprint must bypass the automatic print switch")
	}
}

func TestInactiveOrMissingStoreDisablesPrintingWithoutFailingMoneyFlow(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := regexp.QuoteMeta("SELECT auto_print_receipt,auto_print_label,status,deleted_at FROM stores") + `\s+` + regexp.QuoteMeta("WHERE id=? AND tenant_id=?")
	mock.ExpectQuery(query).WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"auto_print_receipt", "auto_print_label", "status", "deleted_at"}).AddRow(true, true, "DISABLED", nil))
	policy, err := loadStorePrintPolicy(context.Background(), db, 7, 9)
	if err != nil || policy.AutoReceipt || policy.AutoLabel {
		t.Fatalf("disabled store must yield a no-print policy without error: policy=%+v err=%v", policy, err)
	}
	mock.ExpectQuery(query).WithArgs(int64(10), int64(7)).WillReturnError(sql.ErrNoRows)
	policy, err = loadStorePrintPolicy(context.Background(), db, 7, 10)
	if err != nil || policy.AutoReceipt || policy.AutoLabel {
		t.Fatalf("missing store must not abort payment recognition: policy=%+v err=%v", policy, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLabelRenderingSplitsByItemQuantityAndReplacesItemVariables(t *testing.T) {
	t.Parallel()
	order := orderDTO{
		ID: 23, StoreName: "码农咖啡", OrderNo: "TB23", OrderType: orderTypeDineIn, PaidCents: 1890,
		Table: &orderTableDTO{Name: "B02", AreaName: "露台", TableCode: "B02"},
		Items: []orderItemDTO{{ProductName: "拿铁", SKUName: "大杯", Quantity: 2, ItemRemark: "少冰", Configuration: map[string]any{
			"options":   []any{map[string]any{"groupName": "温度", "valueName": "少冰"}},
			"modifiers": []any{map[string]any{"name": "燕麦奶", "quantity": float64(1)}},
		}}},
	}
	template := "{{store_name}} #{{pickup_no}} {{paid_amount}}\n{{product_name}} {{sku_name}} x{{quantity}}/{{ordered_quantity}}\n{{options}}\n{{modifiers}}\n{{item_remark}}\n{{items}}"
	contents := renderPrintContents("LABEL", template, order, "", false)
	if len(contents) != 2 {
		t.Fatalf("two ordered drinks must create two labels, got %d", len(contents))
	}
	for _, content := range contents {
		for _, expected := range []string{"码农咖啡 #0023 18.90", "拿铁 大杯 x1/2", "温度：少冰", "燕麦奶", "少冰"} {
			if !strings.Contains(content, expected) {
				t.Fatalf("label missing %q: %s", expected, content)
			}
		}
		if strings.Contains(content, "{{") {
			t.Fatalf("supported template variable remained unresolved: %s", content)
		}
	}
}
