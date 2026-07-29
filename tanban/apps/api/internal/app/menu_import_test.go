package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xuri/excelize/v2"
)

func TestParseMenuImportWorkbookSupportsNoSpecMultiSpecAndDefinitions(t *testing.T) {
	data := buildMenuImportTestWorkbook(t, func(file *excelize.File) {
		setMenuImportTestRow(t, file, "商品", 2, []any{
			"P001", "招牌拿铁", "咖啡", "双份浓缩", "https://cdn.example.com/latte.jpg", "", "", "",
			"多规格", "", "", "是", "是", "是", "是", "温度", "加浓", "10",
		})
		setMenuImportTestRow(t, file, "商品", 3, []any{
			"P002", "黄油可颂", "烘焙", "每日现烤", "", "", "", "",
			"无规格", "16.50", "80", "是", "否", "是", "是", "", "", "20",
		})
		setMenuImportTestRow(t, file, "规格", 2, []any{"P001", "中杯热", "26", "50", "是"})
		setMenuImportTestRow(t, file, "规格", 3, []any{"P001", "大杯冰", "30", "40", "是"})
		setMenuImportTestRow(t, file, "规格属性", 2, []any{"P001", "中杯热", "杯型", "中杯"})
		setMenuImportTestRow(t, file, "规格属性", 3, []any{"P001", "中杯热", "温度", "热"})
		setMenuImportTestRow(t, file, "规格属性", 4, []any{"P001", "大杯冰", "杯型", "大杯"})
		setMenuImportTestRow(t, file, "规格属性", 5, []any{"P001", "大杯冰", "温度", "冰"})
		setMenuImportTestRow(t, file, "属性组", 2, []any{"温度", "单选", "1", "1", "热", "0", "是"})
		setMenuImportTestRow(t, file, "属性组", 3, []any{"温度", "单选", "1", "1", "冰", "0", "否"})
		setMenuImportTestRow(t, file, "加料组", 2, []any{"加浓", "0", "2", "浓缩咖啡", "4", "否"})
	})

	book, err := parseMenuImportWorkbook(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseMenuImportWorkbook returned error: %v", err)
	}
	if menuImportHasErrors(book.Issues) {
		t.Fatalf("expected valid workbook, got issues: %#v", book.Issues)
	}
	if len(book.Products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(book.Products))
	}
	if len(book.Products[0].SKUs) != 2 {
		t.Fatalf("expected 2 multi-spec skus, got %d", len(book.Products[0].SKUs))
	}
	if got := book.Products[0].SKUs[1].Attributes["杯型"]; got != "大杯" {
		t.Fatalf("expected normalized specification attribute sheet to parse, got %q", got)
	}
	if len(book.Products[1].SKUs) != 1 || book.Products[1].SKUs[0].Name != "默认规格" {
		t.Fatalf("expected implicit default sku, got %#v", book.Products[1].SKUs)
	}
	if book.Products[1].SKUs[0].PriceCents != 1650 || book.Products[1].SKUs[0].Stock != 80 {
		t.Fatalf("unexpected default sku price/stock: %#v", book.Products[1].SKUs[0])
	}
	if len(book.AttributeGroups) != 1 || len(book.ModifierGroups) != 1 {
		t.Fatalf("expected attribute and modifier definitions, got attributes=%d modifiers=%d", len(book.AttributeGroups), len(book.ModifierGroups))
	}
}

func TestParseMenuImportWorkbookReportsActionableRowErrors(t *testing.T) {
	data := buildMenuImportTestWorkbook(t, func(file *excelize.File) {
		setMenuImportTestRow(t, file, "商品", 2, []any{
			"P001", "错误商品", "测试分类", "", "javascript:alert(1)", "", "", "",
			"多规格", "", "", "也许", "否", "是", "是", "", "", "",
		})
	})
	book, err := parseMenuImportWorkbook(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseMenuImportWorkbook returned error: %v", err)
	}
	codes := map[string]bool{}
	for _, issue := range book.Issues {
		codes[issue.Code] = true
		if issue.Code == "INVALID_IMAGE_URL" && issue.Row != 2 {
			t.Fatalf("expected invalid image at product row 2, got %#v", issue)
		}
	}
	for _, code := range []string{"INVALID_IMAGE_URL", "INVALID_BOOLEAN", "MISSING_SKUS"} {
		if !codes[code] {
			t.Fatalf("expected issue %s, got %#v", code, book.Issues)
		}
	}
}

func TestDownloadMenuImportTemplateServesValidWorkbook(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.downloadMenuImportTemplate(response, httptest.NewRequest(http.MethodGet, "/products/import-template", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	file, err := excelize.OpenReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("downloaded template is not a valid workbook: %v", err)
	}
	defer file.Close()
	for _, expected := range []string{"导入说明", "商品", "规格", "规格属性", "属性组", "加料组", "填写示例"} {
		found := false
		for _, sheet := range file.GetSheetList() {
			found = found || sheet == expected
		}
		if !found {
			t.Fatalf("downloaded template missing sheet %q", expected)
		}
	}
}

func TestAnalyzeMenuImportMatchesExistingNamesAndPlansMissingLibraries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,name,status FROM categories").WithArgs(int64(7), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status"}).AddRow(11, "咖啡", "ACTIVE"))
	mock.ExpectQuery("SELECT id,name,status FROM attribute_groups").WithArgs(int64(7), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status"}).AddRow(21, "温度", "ACTIVE"))
	mock.ExpectQuery("SELECT id,name,status FROM modifier_groups").WithArgs(int64(7), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status"}))
	mock.ExpectQuery("SELECT name FROM products").WithArgs(int64(7), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	book := &menuImportWorkbook{
		Products: []*menuImportProduct{{
			Row: 2, Code: "P001", Name: "拿铁", CategoryName: "咖啡",
			AttributeGroupNames: []string{"温度"}, ModifierGroupNames: []string{"加浓"},
			SKUs: []menuImportSKU{{Name: "默认规格", PriceCents: 2600, Stock: 30}},
		}, {
			Row: 3, Code: "P002", Name: "可颂", CategoryName: "烘焙",
			SKUs: []menuImportSKU{{Name: "默认规格", PriceCents: 1600, Stock: 20}},
		}},
		AttributeGroups: map[string]*menuImportAttributeGroup{},
		ModifierGroups: map[string]*menuImportModifierGroup{
			menuImportKey("加浓"): {Name: "加浓", MinSelect: 0, MaxSelect: 1, Items: []menuImportModifierItem{{Name: "浓缩", PriceCents: 400}}},
		},
		Issues: []menuImportIssue{},
	}
	resolution, err := analyzeMenuImport(context.Background(), db, 7, 9, book)
	if err != nil {
		t.Fatalf("analyzeMenuImport returned error: %v", err)
	}
	preview := buildMenuImportPreview(book, resolution)
	if !preview.Valid {
		t.Fatalf("expected valid preview, got %#v", preview.Issues)
	}
	if preview.ExistingCategoryCount != 1 || len(preview.NewCategories) != 1 || preview.NewCategories[0] != "烘焙" {
		t.Fatalf("unexpected category resolution: %#v", preview)
	}
	if len(preview.ExistingAttributeGroups) != 1 || preview.ExistingAttributeGroups[0] != "温度" {
		t.Fatalf("unexpected attribute resolution: %#v", preview.ExistingAttributeGroups)
	}
	if len(preview.NewModifierGroups) != 1 || preview.NewModifierGroups[0] != "加浓" {
		t.Fatalf("unexpected modifier resolution: %#v", preview.NewModifierGroups)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func buildMenuImportTestWorkbook(t *testing.T, fill func(*excelize.File)) []byte {
	t.Helper()
	file := excelize.NewFile()
	for _, sheet := range []string{"商品", "规格", "规格属性", "属性组", "加料组"} {
		if _, err := file.NewSheet(sheet); err != nil {
			t.Fatalf("create sheet %s: %v", sheet, err)
		}
	}
	if err := file.DeleteSheet("Sheet1"); err != nil {
		t.Fatalf("delete default sheet: %v", err)
	}
	setMenuImportTestRow(t, file, "商品", 1, []any{
		"商品编号*", "商品名称*", "分类名称*", "商品描述", "主图URL", "辅图URL1", "辅图URL2", "辅图URL3",
		"规格类型*", "无规格售价", "无规格库存", "是否上架", "是否推荐", "参与会员折扣", "店内销售", "点单属性组", "加料组", "排序",
	})
	setMenuImportTestRow(t, file, "规格", 1, []any{"商品编号*", "规格名称*", "售价*", "库存*", "是否上架"})
	setMenuImportTestRow(t, file, "规格属性", 1, []any{"商品编号*", "规格名称*", "属性名称*", "属性值*"})
	setMenuImportTestRow(t, file, "属性组", 1, []any{"属性组名称*", "选择方式*", "最少选*", "最多选*", "属性值*", "加价", "是否默认"})
	setMenuImportTestRow(t, file, "加料组", 1, []any{"加料组名称*", "最少选*", "最多选*", "加料名称*", "加价*", "是否默认"})
	fill(file)
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close workbook: %v", err)
	}
	return buffer.Bytes()
}

func setMenuImportTestRow(t *testing.T, file *excelize.File, sheet string, row int, values []any) {
	t.Helper()
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		t.Fatalf("build cell name: %v", err)
	}
	if err = file.SetSheetRow(sheet, cell, &values); err != nil {
		t.Fatalf("set %s row %d: %v", sheet, row, err)
	}
}
