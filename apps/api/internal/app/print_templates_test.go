package app

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizePrintTemplateInput(t *testing.T) {
	t.Parallel()
	disabled := false
	input := printTemplateInput{BusinessType: "dine_in", TemplateType: "label", Name: "杯贴", Content: "{{items}}", TriggerEvent: "order_created", Copies: 3, Enabled: &disabled}
	if err := normalizePrintTemplateInput(&input); err != nil {
		t.Fatal(err)
	}
	if input.BusinessType != orderTypeDineIn || input.TemplateType != "LABEL" || input.CopyRole != "ITEM" || input.PaperWidth != 58 || input.TriggerEvent != "ORDER_CREATED" || input.Copies != 3 || input.Status != "DISABLED" {
		t.Fatalf("unexpected normalized template: %+v", input)
	}
	itemLayout := defaultStructuredPrintLayout("ITEM")
	if itemLayout["headerStyle"] != "SIMPLE" || itemLayout["fontSize"] != "LARGE" || itemLayout["preset"] != "LARGE" || itemLayout["showStoreName"] != false || itemLayout["showOrderType"] != true || itemLayout["showOrderNo"] != false || itemLayout["showItemSequence"] != true || itemLayout["labelWidthMM"] != 40 || itemLayout["labelHeightMM"] != 30 {
		t.Fatalf("item labels must default to SIMPLE/LARGE: %+v", itemLayout)
	}
	customerLayout := defaultStructuredPrintLayout("CUSTOMER")
	if customerLayout["showCustomer"] != true || customerLayout["showAddress"] != true || customerLayout["showQrCode"] != true || customerLayout["customFooter"] == "" || customerLayout["copyTitle"] != "客" || customerLayout["showEndMarker"] != true || customerLayout["feedLines"] != 3 {
		t.Fatalf("customer copies must default to customer, address and pickup-code fields: %+v", customerLayout)
	}
	input.Copies = 6
	if err := normalizePrintTemplateInput(&input); err == nil {
		t.Fatal("more than five copies must be rejected")
	}
}

func TestNormalizeStructuredPrintLayoutAndCopyRoles(t *testing.T) {
	t.Parallel()
	input := printTemplateInput{
		BusinessType: orderTypeDineIn, TemplateType: "RECEIPT", CopyRole: "customer", Name: "顾客联",
		PaperWidth: 80, TriggerEvent: "PAYMENT_SUCCESS", Copies: 1,
		Layout: map[string]any{"schemaVersion": float64(1), "preset": "custom", "headerStyle": "simple", "fontSize": "normal", "copyTitle": "客", "showStoreName": false, "showItemOptions": false, "feedLines": float64(4), "labelWidthMM": float64(40), "labelHeightMM": float64(30)},
	}
	if err := normalizePrintTemplateInput(&input); err != nil {
		t.Fatal(err)
	}
	if input.CopyRole != "CUSTOMER" || input.PaperWidth != 80 || input.Layout["preset"] != "CUSTOM" || input.Layout["headerStyle"] != "SIMPLE" || input.Layout["showStoreName"] != false || input.Layout["showItemOptions"] != false || input.Layout["feedLines"] != 4 || input.LayoutJSON == "" {
		t.Fatalf("unexpected structured template normalization: %+v", input)
	}
	input.CopyRole = "ITEM"
	if err := normalizePrintTemplateInput(&input); err == nil {
		t.Fatal("ITEM is not a valid RECEIPT copy role")
	}
	input.CopyRole = "KITCHEN"
	input.Layout = map[string]any{"schemaVersion": 1, "headerStyle": "CENTER"}
	if err := normalizePrintTemplateInput(&input); err == nil {
		t.Fatal("legacy header styles must not bypass the SIMPLE/PROMINENT contract")
	}
	input.Layout = map[string]any{"schemaVersion": 1, "feedLines": 9}
	if err := normalizePrintTemplateInput(&input); err == nil {
		t.Fatal("receipt feed lines outside the supported range must be rejected")
	}
}

func TestEnsureDefaultPrintTemplatesCreatesReceiptAndLabelPerBusinessType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := regexp.QuoteMeta("INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,copy_role,name,content_text,trigger_event,copies,paper_width,layout_json,status)")
	for _, businessType := range []string{orderTypeDineIn, orderTypeTakeout, orderTypeDelivery} {
		prefix := map[string]string{orderTypeDineIn: "店内", orderTypeTakeout: "自提", orderTypeDelivery: "外卖"}[businessType]
		specs := []struct{ templateType, copyRole, name, content, status string }{
			{"RECEIPT", "MERCHANT", prefix + "商家联", defaultPrintTemplateContent[businessType], "ACTIVE"},
			{"RECEIPT", "CUSTOMER", prefix + "顾客联", defaultPrintTemplateContent[businessType], "DISABLED"},
			{"RECEIPT", "KITCHEN", prefix + "后厨联", defaultPrintTemplateContent[businessType], "DISABLED"},
			{"LABEL", "ITEM", prefix + "商品标签", defaultLabelTemplateContent[businessType], "ACTIVE"},
		}
		for _, spec := range specs {
			_, layoutJSON, layoutErr := normalizePrintLayout(defaultStructuredPrintLayout(spec.copyRole), spec.copyRole)
			if layoutErr != nil {
				t.Fatal(layoutErr)
			}
			mock.ExpectExec(query).WithArgs(int64(2), businessType, spec.templateType, spec.copyRole, spec.name, spec.content, layoutJSON, spec.status, int64(5), int64(2)).
				WillReturnResult(sqlmock.NewResult(1, 1))
		}
	}
	if err = ensureDefaultPrintTemplates(context.Background(), db, 2, 5); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
