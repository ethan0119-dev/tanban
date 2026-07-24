package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

type fixedWidthPrintContent struct{ columns int }

var printMarkupPattern = regexp.MustCompile(`<[^>]+>`)

func plainPrintLine(value string) string {
	return strings.TrimSpace(printMarkupPattern.ReplaceAllString(value, ""))
}

func TestPrintablePaymentMethodSupportsAllPaymentAdapters(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"mock":           "模拟支付",
		"tianque":        "会生活 / 随行付",
		"wechat_partner": "微信支付",
	}
	for providerName, expected := range tests {
		if got := printablePaymentMethod(map[string]any{"provider": providerName}); got != expected {
			t.Fatalf("printablePaymentMethod(%q)=%q, want %q", providerName, got, expected)
		}
	}
}

func printContentLines(value string) []string {
	value = strings.ReplaceAll(value, "<BR>", "\n")
	return strings.Split(value, "\n")
}

func (matcher fixedWidthPrintContent) Match(value driver.Value) bool {
	content, ok := value.(string)
	if !ok {
		return false
	}
	foundSeparator := false
	for _, line := range printContentLines(content) {
		line = plainPrintLine(line)
		if line == "" {
			continue
		}
		if printDisplayWidth(line) > matcher.columns {
			return false
		}
		if line == strings.Repeat("-", matcher.columns) {
			foundSeparator = true
		}
	}
	return foundSeparator
}

func TestReprintOrderInputAcceptsLegacyClientFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/orders/8/reprint", bytes.NewBufferString(
		`{"type":"RECEIPT","business_type":"DINE_IN","markAsReprint":true}`,
	))
	response := httptest.NewRecorder()
	var input reprintOrderInput
	if !decodeJSON(response, request, &input) {
		t.Fatalf("legacy reprint payload must remain compatible: status=%d body=%s", response.Code, response.Body.String())
	}
	if input.Type != "RECEIPT" || input.BusinessType != "DINE_IN" || !input.MarkAsReprint {
		t.Fatalf("unexpected decoded reprint input: %+v", input)
	}
}

func TestClonePrintJobForReprintCreatesIndependentJob(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sourceContent := "<CB><BOLD>（商）取餐码：0028</BOLD><BR></CB>\n<L>美式 x1  ¥12.00<BR></L><CB>—— #0028 完 ——<BR></CB>"
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT j.order_id,j.printer_id,j.template_id,j.copy_role,j.paper_width,j.content_text,d.output_type
		FROM print_jobs j JOIN printer_devices d ON d.id=j.printer_id AND d.tenant_id=j.tenant_id AND d.store_id=j.store_id
		WHERE j.id=? AND j.tenant_id=? AND j.store_id=? AND j.status IN ('SUCCESS','FAILED','UNKNOWN')`)).
		WithArgs(int64(28), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "printer_id", "template_id", "copy_role", "paper_width", "content_text", "output_type"}).
			AddRow(100, 3, 7, "MERCHANT", 58, sourceContent, "RECEIPT"))

	expectedContent := "<CB><BOLD>【补打】</BOLD><BR></CB><CB><BOLD>(商)取餐码:0028</BOLD><BR></CB><L>美式 x1  12.00<BR></L><CB>--#0028完--<BR></CB>"
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO print_jobs(tenant_id,store_id,order_id,printer_id,template_id,copy_role,paper_width,content_text,status,attempts,is_reprint,reprint_of,error_message,created_by)
		VALUES(?,?,?,?,?,?,?,?,'PENDING',0,1,?,'',?)`)).
		WithArgs(int64(5), int64(9), int64(100), int64(3), int64(7), "MERCHANT", 58, expectedContent, int64(28), int64(12)).
		WillReturnResult(sqlmock.NewResult(29, 1))

	reprintID, err := clonePrintJobForReprint(context.Background(), db, 5, 9, 28, 12)
	if err != nil {
		t.Fatal(err)
	}
	if reprintID != 29 {
		t.Fatalf("expected independent reprint job 29, got %d", reprintID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenderTicketIncludesProductConfiguration(t *testing.T) {
	order := orderDTO{OrderNo: "TB1", OrderType: orderTypeDineIn, TotalCents: 1800, Table: &orderTableDTO{Name: "B02", AreaName: "大厅", TableCode: "B02"}, Items: []orderItemDTO{{
		ProductName: "拿铁", SKUName: "大杯", Quantity: 1, ItemRemark: "奶泡少一点",
		Configuration: map[string]any{
			"options":   []any{map[string]any{"groupName": "温度", "valueName": "冰"}},
			"modifiers": []any{map[string]any{"name": "浓缩", "quantity": float64(2)}},
		},
	}}}
	result := renderTicket("{{order_type}} {{table_area}} {{table_name}} {{table_code}}\n{{items}}", order, "", false)
	for _, expected := range []string{"DINE_IN 大厅 B02 B02", "拿铁 大杯 x1", "冰", "加料：浓缩x2", "单品备注：奶泡少一点"} {
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

func TestNormalizePrinterCopyRoleRoutingAndLegacyDefaults(t *testing.T) {
	t.Parallel()
	receipt := printerInput{OutputType: "receipt", PaperWidth: 58, CopyRoles: []string{"kitchen", "merchant", "kitchen"}}
	if err := normalizePrinterInput(&receipt); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(receipt.CopyRoles, ","); got != "MERCHANT,KITCHEN" || receipt.CopyRolesDatabase != got {
		t.Fatalf("unexpected normalized receipt roles: %+v", receipt.CopyRoles)
	}
	legacyReceipt := printerInput{OutputType: "RECEIPT"}
	if err := normalizePrinterInput(&legacyReceipt); err != nil || strings.Join(legacyReceipt.CopyRoles, ",") != "MERCHANT" {
		t.Fatalf("legacy receipt must conservatively default to MERCHANT: %+v err=%v", legacyReceipt, err)
	}
	legacyLabel := printerInput{OutputType: "LABEL", LabelWidthMM: 40, LabelHeightMM: 30}
	if err := normalizePrinterInput(&legacyLabel); err != nil || strings.Join(legacyLabel.CopyRoles, ",") != "ITEM" {
		t.Fatalf("legacy label must default to ITEM: %+v err=%v", legacyLabel, err)
	}
	missingLabelSize := printerInput{OutputType: "LABEL"}
	if err := normalizePrinterInput(&missingLabelSize); err == nil {
		t.Fatal("label printers must require physical label dimensions")
	}
	invalid := printerInput{OutputType: "RECEIPT", CopyRoles: []string{"ITEM"}}
	if err := normalizePrinterInput(&invalid); err == nil {
		t.Fatal("receipt printers must reject ITEM routing")
	}
	invalid = printerInput{OutputType: "LABEL", CopyRoles: []string{"MERCHANT"}}
	if err := normalizePrinterInput(&invalid); err == nil {
		t.Fatal("label printers must reject receipt copy roles")
	}
}

func TestUpdatePrinterAcceptsUnchangedValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE printer_devices SET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM printer_devices").
		WithArgs(int64(11), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices").
		WithArgs(int64(11), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "store_id", "name", "provider", "model", "sn", "paper_width", "label_width_mm", "label_height_mm", "print_trigger", "output_type", "copy_roles", "template_text", "status"}).
			AddRow(11, 9, "后厨打印机", "mock", "Mock Printer", "MOCK-11", 58, nil, nil, "PAYMENT_SUCCESS", "RECEIPT", "MERCHANT", "ticket", "ACTIVE"))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Put("/printers/{printerID}", server.updatePrinter)
	request := httptest.NewRequest(http.MethodPut, "/printers/11", bytes.NewBufferString(`{
		"name":"后厨打印机","provider":"mock","model":"Mock Printer","sn":"MOCK-11",
		"paper_width":58,"print_trigger":"PAYMENT_SUCCESS","output_type":"RECEIPT",
		"copyRoles":["MERCHANT"],"template_text":"ticket","status":"ACTIVE"
	}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 7, TenantID: 5, Role: RoleMerchantManager}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
	query := regexp.QuoteMeta("SELECT auto_print_receipt,auto_print_label,COALESCE(phone,''),status,deleted_at FROM stores") + `\s+` + regexp.QuoteMeta("WHERE id=? AND tenant_id=?")
	mock.ExpectQuery(query).WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"auto_print_receipt", "auto_print_label", "phone", "status", "deleted_at"}).AddRow(true, true, "18602296557", "DISABLED", nil))
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
	template := "{{store_name}} #{{pickup_no}} {{paid_amount}} {{item_sequence}}\n{{product_name}} {{sku_name}} x{{quantity}}/{{ordered_quantity}}\n{{options}}\n{{modifiers}}\n{{item_remark}}\n{{items}}"
	contents := renderPrintContents("LABEL", template, order, "", false)
	if len(contents) != 2 {
		t.Fatalf("two ordered drinks must create two labels, got %d", len(contents))
	}
	for index, content := range contents {
		for _, expected := range []string{fmt.Sprintf("码农咖啡 #0023 18.90 %d/2", index+1), "拿铁 大杯 x1/2", "燕麦奶", "少冰"} {
			if !strings.Contains(content, expected) {
				t.Fatalf("label missing %q: %s", expected, content)
			}
		}
		if strings.Contains(content, "{{") {
			t.Fatalf("supported template variable remained unresolved: %s", content)
		}
	}
}

func TestStructuredReceiptRendersCopyRoleAndKeepsCJKWithinPaperWidth(t *testing.T) {
	t.Parallel()
	order := orderDTO{
		ID: 23, StoreName: "码农咖啡", OrderNo: "TB202607200001", OrderType: orderTypeDineIn,
		CustomerName: "张三", CustomerPhone: "13800000000", Remark: "打包带走", TotalCents: 3780, PaidCents: 3780,
		CreatedAt: "2026-07-20T18:26:00Z", Payment: map[string]any{"provider": "tianque"},
		Table: &orderTableDTO{Name: "B02", AreaName: "露台", TableCode: "B02"},
		Items: []orderItemDTO{{
			ProductName: "超长名称燕麦奶生椰水拿铁", SKUName: "超大杯", Quantity: 2, UnitPriceCents: 1890, SubtotalCents: 3780, ItemRemark: "少冰不要吸管",
			Configuration: map[string]any{
				"options":   []any{map[string]any{"groupName": "温度", "valueName": "少冰"}},
				"modifiers": []any{map[string]any{"name": "燕麦奶", "quantity": float64(1)}},
			},
		}},
	}
	layout := defaultStructuredPrintLayout("MERCHANT")
	layout["showCustomer"] = true
	layout["customFooter"] = "谢谢惠顾 {{order_no}}"
	template := activePrintTemplate{CopyRole: "MERCHANT", PaperWidth: 58, StorePhone: "18602296557", Layout: layout}
	content := renderStructuredReceipt(template, order, "", false)
	for _, expected := range []string{"<CB>", "(商)取餐码:0023", "码农咖啡", "店内堂食", "露台 B02 B02", "2026-07-20 18:26:00", "商品", "数量", "单价", "金额", "超长名称燕", "麦奶生椰水", "拿铁 超大杯", "少冰", "加料：燕麦奶", "备注：少冰不要吸管", "<L><B>实付", "会生活 / 随行付", "张三 13800000000", "谢谢惠顾 TB202607200001", "客服电话：18602296557", "--#0023完--", "<BR>"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("structured receipt missing %q:\n%s", expected, content)
		}
	}
	if strings.ContainsAny(content, "\r\n") {
		t.Fatalf("structured receipt must use only explicit <BR> line feeds:\n%s", content)
	}
	if strings.ContainsAny(content, "¥￥") {
		t.Fatalf("structured receipt must omit unsupported currency glyphs:\n%s", content)
	}
	if strings.Contains(content, "温度：少冰") {
		t.Fatalf("structured receipt must omit option group names by default:\n%s", content)
	}
	if strings.Contains(content, "<CB><BOLD>(商)取餐码") {
		t.Fatalf("pickup headline must not stack large and bold printer commands:\n%s", content)
	}
	if !strings.Contains(content, "</B><BR></L><BR><L>支付：") {
		t.Fatalf("payment method must have a blank line after the emphasized paid row:\n%s", content)
	}
	footerSequence := "<L>" + strings.Repeat("-", 32) + "<BR></L><C>谢谢惠顾 TB202607200001<BR></C><C>客服电话：18602296557<BR></C><BR><CB>"
	if !strings.Contains(content, footerSequence) {
		t.Fatalf("footer must be separated, include the store phone and leave space before the end marker:\n%s", content)
	}
	if headline := "(商)取餐码:0023"; printDisplayWidth(headline) > printableColumns(58, "LARGE") {
		t.Fatalf("58mm pickup headline would wrap at large size: %q", headline)
	}
	for _, line := range printContentLines(content) {
		line = plainPrintLine(line)
		if line == "" {
			continue
		}
		if got := printDisplayWidth(line); got > 32 {
			t.Fatalf("58mm line exceeds 32 display columns (%d): %q", got, line)
		}
	}
	if !strings.Contains(content, strings.Repeat("-", 32)) {
		t.Fatalf("58mm receipt must use a 32-column separator:\n%s", content)
	}

	layout["endMarkerText"] = "这是一个很长但不应该被打印机自动换行的结束标识"
	customEnd := renderStructuredReceipt(template, order, "", false)
	if !strings.Contains(customEnd, "<C><BOLD>") {
		t.Fatalf("long custom end marker must fall back to centered normal size:\n%s", customEnd)
	}
	for _, line := range printContentLines(customEnd) {
		if got := printDisplayWidth(plainPrintLine(line)); got > 32 {
			t.Fatalf("custom 58mm line exceeds 32 display columns (%d): %q", got, plainPrintLine(line))
		}
	}

	layout["showItemHeader"] = false
	layout["showOptionGroupNames"] = true
	layout["emphasizePaid"] = false
	customized := renderStructuredReceipt(template, order, "", false)
	if strings.Contains(customized, "商品        数量") || !strings.Contains(customized, "温度：少冰") || strings.Contains(customized, "<L><B>实付") {
		t.Fatalf("structured receipt must honor item header, option label and paid emphasis switches:\n%s", customized)
	}

	template.CopyRole = "KITCHEN"
	template.PaperWidth = 80
	template.Layout = defaultStructuredPrintLayout("KITCHEN")
	kitchen := renderStructuredReceipt(template, order, "", false)
	if !strings.Contains(kitchen, "(厨)取餐码:0023") || strings.ContainsAny(kitchen, "¥￥") || strings.Contains(kitchen, "合计") || strings.Contains(kitchen, "实付") {
		t.Fatalf("kitchen copy must emphasize production data without prices:\n%s", kitchen)
	}
	if !strings.Contains(kitchen, "<BR><CB><BOLD>--#0023完--") {
		t.Fatalf("kitchen copy must leave the same blank line before the end marker:\n%s", kitchen)
	}
	for _, line := range printContentLines(kitchen) {
		line = plainPrintLine(line)
		if line == "" {
			continue
		}
		if got := printDisplayWidth(line); got > 24 {
			t.Fatalf("80mm LARGE line exceeds 24 display columns (%d): %q", got, line)
		}
	}
}

func TestStructuredReceiptPrintsProductAndOptionsBeforeBoldValues(t *testing.T) {
	t.Parallel()
	order := orderDTO{
		ID: 24, StoreName: "码农咖啡", OrderNo: "TB24", OrderType: orderTypeTakeout,
		TotalCents: 1200, PaidCents: 1200,
		Items: []orderItemDTO{{
			ProductName: "美式咖啡", SKUName: "标准杯", Quantity: 1,
			UnitPriceCents: 1200, SubtotalCents: 1200,
			Configuration: map[string]any{"options": []any{
				map[string]any{"groupName": "温度", "valueName": "冰"},
				map[string]any{"groupName": "甜度", "valueName": "无糖"},
			}},
		}},
	}
	template := activePrintTemplate{CopyRole: "MERCHANT", PaperWidth: 80, Layout: defaultStructuredPrintLayout("MERCHANT")}
	content := renderStructuredReceipt(template, order, "", false)
	if !strings.Contains(content, "<L>商品") || strings.Contains(content, "<BOLD>商品") {
		t.Fatalf("item table header must use normal weight:\n%s", content)
	}
	if !strings.Contains(content, "<L>美式咖啡 标准杯（冰，无糖）<BR></L>") || strings.Contains(content, "<BOLD>美式咖啡") {
		t.Fatalf("product, SKU and option values must share one normal-weight line:\n%s", content)
	}
	if !strings.Contains(content, "<BOLD>") || !strings.Contains(content, "x1") || !strings.Contains(content, "12.00") {
		t.Fatalf("quantity, unit price and amount must be emitted on a bold value row:\n%s", content)
	}
	valueLine := printReceiptItemValueLines(1, 1200, 1200, printableColumns(80, "NORMAL"), true)[0]
	if !strings.HasPrefix(valueLine, " ") || !strings.Contains(content, "<BOLD>"+valueLine+"</BOLD>") {
		t.Fatalf("printer markup must preserve the leading column padding on the value row:\n%s", content)
	}
	if strings.Contains(content, "<L>冰<BR></L>") || strings.Contains(content, "<L>无糖<BR></L>") {
		t.Fatalf("option values must not be emitted as standalone rows:\n%s", content)
	}
	if !strings.Contains(content, "<BR><CB><BOLD>--#0024完--") {
		t.Fatalf("merchant copy must leave a blank line before the end marker even without a footer:\n%s", content)
	}
}

func TestStructuredItemLabelSplitsQuantityAndHonorsLayoutSwitches(t *testing.T) {
	t.Parallel()
	order := orderDTO{
		ID: 8, StoreName: "码农咖啡", OrderNo: "TB8", OrderType: orderTypeDineIn, PaidCents: 2400, Remark: "整单加急",
		Table: &orderTableDTO{Name: "A01", AreaName: "大厅"},
		Items: []orderItemDTO{{ProductName: "美式", SKUName: "大杯", Quantity: 2, UnitPriceCents: 1200, ItemRemark: "不要糖", Configuration: map[string]any{
			"options": []any{map[string]any{"groupName": "温度", "valueName": "热"}},
		}}},
	}
	layout := defaultStructuredPrintLayout("ITEM")
	layout["showOrderNo"] = true
	layout["showPrices"] = true
	layout["showPayment"] = true
	layout["showQrCode"] = true
	template := activePrintTemplate{CopyRole: "ITEM", PaperWidth: 80, Layout: layout}
	contents := renderTemplateContents("LABEL", "ignored", template, order, "", false)
	if len(contents) != 2 {
		t.Fatalf("quantity two must create two structured item labels, got %d", len(contents))
	}
	for index, content := range contents {
		for _, expected := range []string{"<PAGE l=\"2\"><SIZE>40,30</SIZE>", fmt.Sprintf("数量：%d/2", index+1), "美式", "规格：大杯", "属性：热", "备注：不要糖", "订单：TB8", `h="2"`} {
			if !strings.Contains(content, expected) {
				t.Fatalf("structured label missing %q:\n%s", expected, content)
			}
		}
		if strings.Contains(content, "店内堂食") || strings.Contains(content, "码农咖啡") {
			t.Fatalf("default item label must keep the compact frontend contract:\n%s", content)
		}
		if !strings.HasSuffix(content, "</PAGE>") {
			t.Fatalf("structured label must close the physical label page:\n%s", content)
		}
	}

	layout["showRemark"] = false
	withoutRemarks := renderStructuredLabel(templateWithLayout(template, layout), order, order.Items[0], 1, 2, "", false)
	if strings.Contains(withoutRemarks, "不要糖") || strings.Contains(withoutRemarks, "整单加急") {
		t.Fatalf("showRemark=false must hide item and order remarks:\n%s", withoutRemarks)
	}
}

func templateWithLayout(template activePrintTemplate, layout map[string]any) activePrintTemplate {
	template.Layout = layout
	return template
}

func TestLoadPrintTemplatesReturnsEveryCopyRoleIncludingDisabledRows(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := regexp.QuoteMeta("SELECT id,template_type,COALESCE(copy_role,CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END),content_text,trigger_event,copies,paper_width,COALESCE(layout_json,'{}'),status")
	mock.ExpectQuery(query).WithArgs(int64(2), int64(5), orderTypeDineIn, "RECEIPT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "template_type", "copy_role", "content_text", "trigger_event", "copies", "paper_width", "layout_json", "status"}).
			AddRow(1, "RECEIPT", "MERCHANT", "legacy", "PAYMENT_SUCCESS", 1, 58, `{}`, "ACTIVE").
			AddRow(2, "RECEIPT", "CUSTOMER", "", "PAYMENT_SUCCESS", 2, 80, `{"schemaVersion":1,"headerStyle":"PROMINENT"}`, "DISABLED").
			AddRow(3, "RECEIPT", "KITCHEN", "", "ORDER_CREATED", 1, 80, `{"schemaVersion":1,"fontSize":"LARGE"}`, "ACTIVE"),
	)
	templates, err := loadPrintTemplates(context.Background(), db, 2, 5, orderTypeDineIn, "RECEIPT")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 3 || templates[0].CopyRole != "MERCHANT" || templates[1].CopyRole != "CUSTOMER" || templates[1].Enabled || templates[2].CopyRole != "KITCHEN" || !templates[2].Enabled || templates[2].PaperWidth != 80 {
		t.Fatalf("unexpected copy-role templates: %+v", templates)
	}
	if len(templates[0].Layout) != 0 || templates[1].Layout["headerStyle"] != "PROMINENT" {
		t.Fatalf("legacy and structured layouts were not preserved: %+v", templates)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnqueueOrderPrintsCreatesIndependentJobsForEnabledCopyRoles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,store_id,(SELECT name FROM stores WHERE stores.id=orders.store_id),order_no")).
		WithArgs(int64(2), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "store_id", "store_name", "order_no", "customer_name", "customer_phone", "remark", "source", "fulfillment_type", "order_type",
			"business_date", "pickup_sequence", "pickup_code", "fast_food_plate_id", "fast_food_plate_public_id_snapshot", "fast_food_plate_name_snapshot", "fast_food_plate_code_snapshot",
			"table_id", "table_public_id_snapshot", "table_area_name_snapshot", "table_name_snapshot", "table_code_snapshot", "status", "payment_status", "total_cents", "paid_cents", "refunded_cents", "paid_at", "created_at",
		}).AddRow(11, 2, 5, "码农咖啡", "TB11", "", "", "", "MINI_PROGRAM", "DINE_IN", orderTypeDineIn, "2026-07-20", nil, "", nil, "", "", "", nil, "", "", "", "", "PAID", "PAID", 1200, 1200, 0, nil, "2026-07-20T08:00:00Z"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,product_id,sku_id,product_name,sku_name,attributes_json,COALESCE(configuration_json,'{}'),item_remark,base_price_cents,modifier_price_cents,unit_price_cents,quantity,subtotal_cents FROM order_items")).
		WithArgs(int64(2), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "sku_id", "product_name", "sku_name", "attributes_json", "configuration_json", "item_remark", "base_price_cents", "modifier_price_cents", "unit_price_cents", "quantity", "subtotal_cents"}).
			AddRow(101, 201, 301, "美式", "大杯", `{}`, `{}`, "", 1200, 0, 1200, 1, 1200))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,amount_cents,status FROM payment_transactions")).
		WithArgs(int64(2), int64(11)).WillReturnError(sql.ErrNoRows)

	defaultInsert := regexp.QuoteMeta("INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,copy_role,name,content_text,trigger_event,copies,paper_width,layout_json,status)")
	for index := 0; index < 12; index++ {
		mock.ExpectExec(defaultInsert).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT auto_print_receipt,auto_print_label,COALESCE(phone,''),status,deleted_at FROM stores")).
		WithArgs(int64(5), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"auto_print_receipt", "auto_print_label", "phone", "status", "deleted_at"}).AddRow(true, false, "18602296557", "ACTIVE", nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices")).
		WithArgs(int64(2), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "store_id", "name", "provider", "model", "sn", "paper_width", "label_width_mm", "label_height_mm", "print_trigger", "output_type", "copy_roles", "template_text", "status"}).
			AddRow(31, 5, "收银台", "mock", "virtual", "SN31", 58, nil, nil, "PAYMENT_SUCCESS", "RECEIPT", "MERCHANT,CUSTOMER", "legacy", "ACTIVE"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,template_type,COALESCE(copy_role,CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END),content_text,trigger_event,copies,paper_width,COALESCE(layout_json,'{}'),status")).
		WithArgs(int64(2), int64(5), orderTypeDineIn, "RECEIPT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "template_type", "copy_role", "content_text", "trigger_event", "copies", "paper_width", "layout_json", "status"}).
			AddRow(41, "RECEIPT", "MERCHANT", "", "PAYMENT_SUCCESS", 1, 58, `{"schemaVersion":1,"headerStyle":"PROMINENT"}`, "ACTIVE").
			AddRow(42, "RECEIPT", "CUSTOMER", "", "PAYMENT_SUCCESS", 1, 80, `{"schemaVersion":1,"headerStyle":"SIMPLE"}`, "ACTIVE").
			AddRow(43, "RECEIPT", "KITCHEN", "", "PAYMENT_SUCCESS", 1, 58, `{"schemaVersion":1}`, "ACTIVE"))
	jobInsert := regexp.QuoteMeta("INSERT INTO print_jobs(tenant_id,store_id,order_id,printer_id,template_id,copy_role,paper_width,content_text,status,is_reprint,created_by) VALUES(?,?,?,?,?,?,?,?,'PENDING',?,?)")
	mock.ExpectExec(jobInsert).WithArgs(int64(2), int64(5), int64(11), int64(31), int64(41), "MERCHANT", 58, fixedWidthPrintContent{columns: 32}, false, int64(9)).WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec(jobInsert).WithArgs(int64(2), int64(5), int64(11), int64(31), int64(42), "CUSTOMER", 58, fixedWidthPrintContent{columns: 32}, false, int64(9)).WillReturnResult(sqlmock.NewResult(52, 1))

	server := &Server{}
	if err = server.enqueueOrderPrintsWithOutput(context.Background(), db, 2, 5, 11, "PAYMENT_SUCCESS", false, 9, "", "RECEIPT"); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
