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
	if input.BusinessType != orderTypeDineIn || input.TemplateType != "LABEL" || input.TriggerEvent != "ORDER_CREATED" || input.Copies != 3 || input.Status != "DISABLED" {
		t.Fatalf("unexpected normalized template: %+v", input)
	}
	input.Copies = 6
	if err := normalizePrintTemplateInput(&input); err == nil {
		t.Fatal("more than five copies must be rejected")
	}
}

func TestEnsureDefaultPrintTemplatesCreatesReceiptAndLabelPerBusinessType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := regexp.QuoteMeta("INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)")
	for _, businessType := range []string{orderTypeDineIn, orderTypeTakeout, orderTypeDelivery} {
		for _, templateType := range []string{"RECEIPT", "LABEL"} {
			name := map[string]string{orderTypeDineIn: "店内", orderTypeTakeout: "自提", orderTypeDelivery: "外卖"}[businessType]
			content := defaultPrintTemplateContent[businessType]
			if templateType == "LABEL" {
				name += "标签"
				content = defaultLabelTemplateContent[businessType]
			} else {
				name += "小票"
			}
			mock.ExpectExec(query).WithArgs(int64(2), businessType, templateType, name, content, int64(5), int64(2)).
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
