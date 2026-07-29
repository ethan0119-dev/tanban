package app

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	menuImportMaxFileBytes = 10 << 20
	menuImportMaxProducts  = 500
	menuImportMaxSKUs      = 2000
)

//go:embed assets/menu-import-template.xlsx
var menuImportTemplate []byte

type menuImportIssue struct {
	Level   string `json:"level"`
	Sheet   string `json:"sheet"`
	Row     int    `json:"row,omitempty"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type menuImportSKU struct {
	Row        int
	Name       string
	PriceCents int64
	Stock      int
	Enabled    bool
	Attributes map[string]string
}

type menuImportProduct struct {
	Row                   int
	Code                  string
	Name                  string
	CategoryName          string
	Description           string
	ImageURLs             []string
	MultiSKU              bool
	BasePriceCents        int64
	BaseStock             int
	Enabled               bool
	Recommended           bool
	MemberDiscountEnabled bool
	InStoreEnabled        bool
	AttributeGroupNames   []string
	ModifierGroupNames    []string
	SortOrder             int
	SKUs                  []menuImportSKU
}

type menuImportAttributeValue struct {
	Row             int
	Name            string
	PriceDeltaCents int64
	IsDefault       bool
}

type menuImportAttributeGroup struct {
	Name          string
	SelectionMode string
	MinSelect     int
	MaxSelect     int
	Values        []menuImportAttributeValue
}

type menuImportModifierItem struct {
	Row        int
	Name       string
	PriceCents int64
	IsDefault  bool
}

type menuImportModifierGroup struct {
	Name      string
	MinSelect int
	MaxSelect int
	Items     []menuImportModifierItem
}

type menuImportWorkbook struct {
	Products        []*menuImportProduct
	AttributeGroups map[string]*menuImportAttributeGroup
	ModifierGroups  map[string]*menuImportModifierGroup
	Issues          []menuImportIssue
}

type menuImportProductPreview struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	CategoryName    string   `json:"category_name"`
	SKUCount        int      `json:"sku_count"`
	ImageCount      int      `json:"image_count"`
	AttributeGroups []string `json:"attribute_groups"`
	ModifierGroups  []string `json:"modifier_groups"`
}

type menuImportPreview struct {
	Valid                   bool                       `json:"valid"`
	ProductCount            int                        `json:"product_count"`
	SKUCount                int                        `json:"sku_count"`
	ExistingCategoryCount   int                        `json:"existing_category_count"`
	NewCategories           []string                   `json:"new_categories"`
	ExistingAttributeGroups []string                   `json:"existing_attribute_groups"`
	NewAttributeGroups      []string                   `json:"new_attribute_groups"`
	ExistingModifierGroups  []string                   `json:"existing_modifier_groups"`
	NewModifierGroups       []string                   `json:"new_modifier_groups"`
	Products                []menuImportProductPreview `json:"products"`
	Issues                  []menuImportIssue          `json:"issues"`
}

type menuImportResult struct {
	ProductCount      int      `json:"product_count"`
	SKUCount          int      `json:"sku_count"`
	CreatedCategories []string `json:"created_categories"`
	CreatedAttributes []string `json:"created_attribute_groups"`
	CreatedModifiers  []string `json:"created_modifier_groups"`
	CreatedProductIDs []int64  `json:"created_product_ids"`
	Warnings          int      `json:"warnings"`
}

type menuImportNamedID struct {
	ID     int64
	Name   string
	Status string
}

type menuImportResolution struct {
	CategoryIDs            map[string]menuImportNamedID
	AttributeGroupIDs      map[string]menuImportNamedID
	ModifierGroupIDs       map[string]menuImportNamedID
	NewCategories          []string
	NewAttributeGroups     []string
	NewModifierGroups      []string
	ExistingCategories     []string
	ExistingAttributeNames []string
	ExistingModifierNames  []string
	Issues                 []menuImportIssue
}

type menuImportQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Server) downloadMenuImportTemplate(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=\"tanban-menu-import-template.xlsx\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(menuImportTemplate)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(menuImportTemplate)
}

func (s *Server) previewMenuImport(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	book, err := readMenuImportRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MENU_IMPORT_FILE", err.Error())
		return
	}
	resolution, err := analyzeMenuImport(r.Context(), s.DB, identity.TenantID, storeID, book)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, buildMenuImportPreview(book, resolution))
}

func (s *Server) importMenu(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	book, err := readMenuImportRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MENU_IMPORT_FILE", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	resolution, err := analyzeMenuImport(r.Context(), tx, identity.TenantID, storeID, book)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	preview := buildMenuImportPreview(book, resolution)
	if !preview.Valid {
		first := firstMenuImportError(preview.Issues)
		writeError(w, http.StatusUnprocessableEntity, "MENU_IMPORT_VALIDATION_FAILED", fmt.Sprintf("%s；请重新预检文件", first))
		return
	}

	result, err := executeMenuImport(r.Context(), s, tx, identity.TenantID, storeID, book, resolution)
	if err != nil {
		if errors.Is(err, errDecorationAssetUnavailable) {
			writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
			return
		}
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "menu.import", "store", int64String(storeID), map[string]any{
		"product_count": len(book.Products),
		"sku_count":     result.SKUCount,
		"categories":    result.CreatedCategories,
		"attributes":    result.CreatedAttributes,
		"modifiers":     result.CreatedModifiers,
	}, r)
	writeData(w, http.StatusCreated, result)
}

func readMenuImportRequest(w http.ResponseWriter, r *http.Request) (*menuImportWorkbook, error) {
	r.Body = http.MaxBytesReader(w, r.Body, menuImportMaxFileBytes)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		return nil, errors.New("文件不能超过 10MB，且必须使用 multipart/form-data 上传")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, errors.New("请选择要导入的 .xlsx 文件")
	}
	defer file.Close()
	if header.Size > menuImportMaxFileBytes {
		return nil, errors.New("文件不能超过 10MB")
	}
	if !strings.EqualFold(filepath.Ext(header.Filename), ".xlsx") {
		return nil, errors.New("仅支持 .xlsx 格式，请使用下载的模板")
	}
	return parseMenuImportWorkbook(file)
}

func parseMenuImportWorkbook(reader io.Reader) (*menuImportWorkbook, error) {
	file, err := excelize.OpenReader(reader, excelize.Options{
		RawCellValue:      false,
		UnzipSizeLimit:    64 << 20,
		UnzipXMLSizeLimit: 16 << 20,
	})
	if err != nil {
		return nil, errors.New("无法读取 Excel 文件，请确认文件未损坏且不是加密工作簿")
	}
	defer file.Close()
	book := &menuImportWorkbook{
		Products:        []*menuImportProduct{},
		AttributeGroups: map[string]*menuImportAttributeGroup{},
		ModifierGroups:  map[string]*menuImportModifierGroup{},
		Issues:          []menuImportIssue{},
	}
	productByCode := parseMenuImportProducts(file, book)
	parseMenuImportSKUs(file, book, productByCode)
	parseMenuImportSKUAttributes(file, book, productByCode)
	parseMenuImportAttributeGroups(file, book)
	parseMenuImportModifierGroups(file, book)
	validateMenuImportWorkbook(book)
	return book, nil
}

func parseMenuImportProducts(file *excelize.File, book *menuImportWorkbook) map[string]*menuImportProduct {
	const sheet = "商品"
	required := []string{"商品编号*", "商品名称*", "分类名称*", "规格类型*"}
	rows, headers, ok := menuImportRows(file, book, sheet, required)
	products := map[string]*menuImportProduct{}
	if !ok {
		return products
	}
	for index, row := range rows[1:] {
		rowNumber := index + 2
		if menuImportRowBlank(row) {
			continue
		}
		code := strings.TrimSpace(menuImportCell(row, headers, "商品编号*"))
		name := strings.TrimSpace(menuImportCell(row, headers, "商品名称*"))
		category := strings.TrimSpace(menuImportCell(row, headers, "分类名称*"))
		specType := strings.TrimSpace(menuImportCell(row, headers, "规格类型*"))
		product := &menuImportProduct{
			Row:                   rowNumber,
			Code:                  code,
			Name:                  name,
			CategoryName:          category,
			Description:           strings.TrimSpace(menuImportCell(row, headers, "商品描述")),
			Enabled:               menuImportBool(book, sheet, rowNumber, "是否上架", menuImportCell(row, headers, "是否上架"), true),
			Recommended:           menuImportBool(book, sheet, rowNumber, "是否推荐", menuImportCell(row, headers, "是否推荐"), false),
			MemberDiscountEnabled: menuImportBool(book, sheet, rowNumber, "参与会员折扣", menuImportCell(row, headers, "参与会员折扣"), true),
			InStoreEnabled:        menuImportBool(book, sheet, rowNumber, "店内销售", menuImportCell(row, headers, "店内销售"), true),
			AttributeGroupNames:   menuImportNames(menuImportCell(row, headers, "点单属性组")),
			ModifierGroupNames:    menuImportNames(menuImportCell(row, headers, "加料组")),
			SKUs:                  []menuImportSKU{},
		}
		for _, field := range []string{"主图URL", "辅图URL1", "辅图URL2", "辅图URL3"} {
			if raw := strings.TrimSpace(menuImportCell(row, headers, field)); raw != "" {
				product.ImageURLs = append(product.ImageURLs, raw)
			}
		}
		switch specType {
		case "无规格":
			product.MultiSKU = false
			product.BasePriceCents = menuImportMoney(book, sheet, rowNumber, "无规格售价", menuImportCell(row, headers, "无规格售价"), true)
			product.BaseStock = menuImportInteger(book, sheet, rowNumber, "无规格库存", menuImportCell(row, headers, "无规格库存"), true)
		case "多规格":
			product.MultiSKU = true
		default:
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "规格类型*", "INVALID_SPEC_TYPE", "规格类型必须填写“无规格”或“多规格”")
		}
		rawSort := strings.TrimSpace(menuImportCell(row, headers, "排序"))
		if rawSort == "" {
			product.SortOrder = len(book.Products) + 1
		} else {
			product.SortOrder = menuImportInteger(book, sheet, rowNumber, "排序", rawSort, true)
		}
		if code == "" {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "PRODUCT_CODE_REQUIRED", "商品编号不能为空")
		} else if !validRequiredText(code, 64) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "INVALID_PRODUCT_CODE", "商品编号不能超过 64 个字符")
		} else if _, exists := products[menuImportKey(code)]; exists {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "DUPLICATE_PRODUCT_CODE", "商品编号在文件中重复")
		} else {
			products[menuImportKey(code)] = product
		}
		if !validRequiredText(name, 120) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品名称*", "INVALID_PRODUCT_NAME", "商品名称必填且不能超过 120 个字符")
		}
		if !validRequiredText(category, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "分类名称*", "INVALID_CATEGORY_NAME", "分类名称必填且不能超过 100 个字符")
		}
		if !validText(product.Description, 1000) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品描述", "INVALID_DESCRIPTION", "商品描述不能超过 1000 个字符")
		}
		seenImages := map[string]bool{}
		for _, imageURL := range product.ImageURLs {
			if len(imageURL) > 1024 || !validDecorationURL(imageURL) {
				menuImportAddIssue(book, "ERROR", sheet, rowNumber, "图片URL", "INVALID_IMAGE_URL", "图片必须填写不超过 1024 个字符的完整 HTTPS URL")
			}
			if seenImages[imageURL] {
				menuImportAddIssue(book, "ERROR", sheet, rowNumber, "图片URL", "DUPLICATE_IMAGE_URL", "同一商品不能填写重复图片")
			}
			seenImages[imageURL] = true
		}
		if product.SortOrder < 0 {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "排序", "INVALID_SORT_ORDER", "排序不能小于 0")
		}
		book.Products = append(book.Products, product)
	}
	return products
}

func parseMenuImportSKUs(file *excelize.File, book *menuImportWorkbook, productByCode map[string]*menuImportProduct) {
	const sheet = "规格"
	required := []string{"商品编号*", "规格名称*", "售价*", "库存*"}
	rows, headers, ok := menuImportRows(file, book, sheet, required)
	if !ok {
		return
	}
	for index, row := range rows[1:] {
		rowNumber := index + 2
		if menuImportRowBlank(row) {
			continue
		}
		code := strings.TrimSpace(menuImportCell(row, headers, "商品编号*"))
		product := productByCode[menuImportKey(code)]
		if product == nil {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "UNKNOWN_PRODUCT_CODE", "规格引用的商品编号不存在")
			continue
		}
		sku := menuImportSKU{
			Row:        rowNumber,
			Name:       strings.TrimSpace(menuImportCell(row, headers, "规格名称*")),
			PriceCents: menuImportMoney(book, sheet, rowNumber, "售价*", menuImportCell(row, headers, "售价*"), true),
			Stock:      menuImportInteger(book, sheet, rowNumber, "库存*", menuImportCell(row, headers, "库存*"), true),
			Enabled:    menuImportBool(book, sheet, rowNumber, "是否上架", menuImportCell(row, headers, "是否上架"), true),
			Attributes: map[string]string{},
		}
		if !validRequiredText(sku.Name, 120) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "规格名称*", "INVALID_SKU_NAME", "规格名称必填且不能超过 120 个字符")
		}
		product.SKUs = append(product.SKUs, sku)
	}
}

func parseMenuImportSKUAttributes(file *excelize.File, book *menuImportWorkbook, productByCode map[string]*menuImportProduct) {
	const sheet = "规格属性"
	required := []string{"商品编号*", "规格名称*", "属性名称*", "属性值*"}
	rows, headers, ok := menuImportRows(file, book, sheet, required)
	if !ok {
		return
	}
	for index, row := range rows[1:] {
		rowNumber := index + 2
		if menuImportRowBlank(row) {
			continue
		}
		code := strings.TrimSpace(menuImportCell(row, headers, "商品编号*"))
		product := productByCode[menuImportKey(code)]
		if product == nil {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "UNKNOWN_PRODUCT_CODE", "规格属性引用的商品编号不存在")
			continue
		}
		if !product.MultiSKU {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "商品编号*", "NO_SPEC_PRODUCT_ATTRIBUTE", "无规格商品不能填写规格属性")
			continue
		}
		skuName := strings.TrimSpace(menuImportCell(row, headers, "规格名称*"))
		var target *menuImportSKU
		for skuIndex := range product.SKUs {
			if menuImportKey(product.SKUs[skuIndex].Name) == menuImportKey(skuName) {
				target = &product.SKUs[skuIndex]
				break
			}
		}
		if target == nil {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "规格名称*", "UNKNOWN_SKU_NAME", "规格属性引用的规格名称不存在")
			continue
		}
		name := strings.TrimSpace(menuImportCell(row, headers, "属性名称*"))
		value := strings.TrimSpace(menuImportCell(row, headers, "属性值*"))
		if !validRequiredText(name, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性名称*", "INVALID_SKU_ATTRIBUTE_NAME", "属性名称必填且不能超过 100 个字符")
			continue
		}
		if !validRequiredText(value, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性值*", "INVALID_SKU_ATTRIBUTE_VALUE", "属性值必填且不能超过 100 个字符")
			continue
		}
		for existing := range target.Attributes {
			if menuImportKey(existing) == menuImportKey(name) {
				menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性名称*", "DUPLICATE_SKU_ATTRIBUTE", "同一规格不能重复填写相同属性名称")
				name = ""
				break
			}
		}
		if name != "" {
			target.Attributes[name] = value
		}
	}
}

func parseMenuImportAttributeGroups(file *excelize.File, book *menuImportWorkbook) {
	const sheet = "属性组"
	required := []string{"属性组名称*", "选择方式*", "最少选*", "最多选*", "属性值*"}
	rows, headers, ok := menuImportRows(file, book, sheet, required)
	if !ok {
		return
	}
	for index, row := range rows[1:] {
		rowNumber := index + 2
		if menuImportRowBlank(row) {
			continue
		}
		name := strings.TrimSpace(menuImportCell(row, headers, "属性组名称*"))
		key := menuImportKey(name)
		modeText := strings.TrimSpace(menuImportCell(row, headers, "选择方式*"))
		mode := map[string]string{"单选": "SINGLE", "多选": "MULTIPLE"}[modeText]
		minSelect := menuImportInteger(book, sheet, rowNumber, "最少选*", menuImportCell(row, headers, "最少选*"), true)
		maxSelect := menuImportInteger(book, sheet, rowNumber, "最多选*", menuImportCell(row, headers, "最多选*"), true)
		value := menuImportAttributeValue{
			Row:             rowNumber,
			Name:            strings.TrimSpace(menuImportCell(row, headers, "属性值*")),
			PriceDeltaCents: menuImportMoney(book, sheet, rowNumber, "加价", menuImportCell(row, headers, "加价"), false),
			IsDefault:       menuImportBool(book, sheet, rowNumber, "是否默认", menuImportCell(row, headers, "是否默认"), false),
		}
		if !validRequiredText(name, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性组名称*", "INVALID_ATTRIBUTE_GROUP_NAME", "属性组名称必填且不能超过 100 个字符")
		}
		if mode == "" {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "选择方式*", "INVALID_SELECTION_MODE", "选择方式必须填写“单选”或“多选”")
		}
		if !validRequiredText(value.Name, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性值*", "INVALID_ATTRIBUTE_VALUE", "属性值必填且不能超过 100 个字符")
		}
		group := book.AttributeGroups[key]
		if group == nil {
			group = &menuImportAttributeGroup{Name: name, SelectionMode: mode, MinSelect: minSelect, MaxSelect: maxSelect, Values: []menuImportAttributeValue{}}
			book.AttributeGroups[key] = group
		} else if group.SelectionMode != mode || group.MinSelect != minSelect || group.MaxSelect != maxSelect {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "属性组名称*", "INCONSISTENT_ATTRIBUTE_GROUP", "同一属性组的选择方式、最少选和最多选必须保持一致")
		}
		group.Values = append(group.Values, value)
	}
}

func parseMenuImportModifierGroups(file *excelize.File, book *menuImportWorkbook) {
	const sheet = "加料组"
	required := []string{"加料组名称*", "最少选*", "最多选*", "加料名称*", "加价*"}
	rows, headers, ok := menuImportRows(file, book, sheet, required)
	if !ok {
		return
	}
	for index, row := range rows[1:] {
		rowNumber := index + 2
		if menuImportRowBlank(row) {
			continue
		}
		name := strings.TrimSpace(menuImportCell(row, headers, "加料组名称*"))
		key := menuImportKey(name)
		minSelect := menuImportInteger(book, sheet, rowNumber, "最少选*", menuImportCell(row, headers, "最少选*"), true)
		maxSelect := menuImportInteger(book, sheet, rowNumber, "最多选*", menuImportCell(row, headers, "最多选*"), true)
		item := menuImportModifierItem{
			Row:        rowNumber,
			Name:       strings.TrimSpace(menuImportCell(row, headers, "加料名称*")),
			PriceCents: menuImportMoney(book, sheet, rowNumber, "加价*", menuImportCell(row, headers, "加价*"), true),
			IsDefault:  menuImportBool(book, sheet, rowNumber, "是否默认", menuImportCell(row, headers, "是否默认"), false),
		}
		if !validRequiredText(name, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "加料组名称*", "INVALID_MODIFIER_GROUP_NAME", "加料组名称必填且不能超过 100 个字符")
		}
		if !validRequiredText(item.Name, 100) {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "加料名称*", "INVALID_MODIFIER_ITEM", "加料名称必填且不能超过 100 个字符")
		}
		group := book.ModifierGroups[key]
		if group == nil {
			group = &menuImportModifierGroup{Name: name, MinSelect: minSelect, MaxSelect: maxSelect, Items: []menuImportModifierItem{}}
			book.ModifierGroups[key] = group
		} else if group.MinSelect != minSelect || group.MaxSelect != maxSelect {
			menuImportAddIssue(book, "ERROR", sheet, rowNumber, "加料组名称*", "INCONSISTENT_MODIFIER_GROUP", "同一加料组的最少选和最多选必须保持一致")
		}
		group.Items = append(group.Items, item)
	}
}

func validateMenuImportWorkbook(book *menuImportWorkbook) {
	if len(book.Products) == 0 {
		menuImportAddIssue(book, "ERROR", "商品", 0, "", "NO_PRODUCTS", "商品表至少需要填写一个商品")
	}
	if len(book.Products) > menuImportMaxProducts {
		menuImportAddIssue(book, "ERROR", "商品", 0, "", "TOO_MANY_PRODUCTS", fmt.Sprintf("单次最多导入 %d 个商品", menuImportMaxProducts))
	}
	skuCount := 0
	for _, product := range book.Products {
		if product.MultiSKU {
			if len(product.SKUs) == 0 {
				menuImportAddIssue(book, "ERROR", "商品", product.Row, "规格类型*", "MISSING_SKUS", "多规格商品必须在规格表至少填写一个规格")
			}
		} else {
			if len(product.SKUs) > 0 {
				menuImportAddIssue(book, "ERROR", "规格", product.SKUs[0].Row, "商品编号*", "UNEXPECTED_SKUS", "无规格商品不能再在规格表填写规格")
			}
			product.SKUs = []menuImportSKU{{
				Row: product.Row, Name: "默认规格", PriceCents: product.BasePriceCents,
				Stock: product.BaseStock, Enabled: product.Enabled, Attributes: map[string]string{},
			}}
		}
		seenNames := map[string]bool{}
		for _, sku := range product.SKUs {
			key := menuImportKey(sku.Name)
			if seenNames[key] {
				menuImportAddIssue(book, "ERROR", "规格", sku.Row, "规格名称*", "DUPLICATE_SKU_NAME", "同一商品的规格名称不能重复")
			}
			seenNames[key] = true
		}
		skuCount += len(product.SKUs)
	}
	if skuCount > menuImportMaxSKUs {
		menuImportAddIssue(book, "ERROR", "规格", 0, "", "TOO_MANY_SKUS", fmt.Sprintf("单次最多导入 %d 个规格", menuImportMaxSKUs))
	}
	for _, group := range book.AttributeGroups {
		if group.MinSelect < 0 || group.MaxSelect < 1 || group.MinSelect > group.MaxSelect {
			menuImportAddIssue(book, "ERROR", "属性组", 0, group.Name, "INVALID_ATTRIBUTE_LIMITS", "属性组的最少选/最多选范围无效")
		}
		if group.SelectionMode == "SINGLE" && group.MaxSelect != 1 {
			menuImportAddIssue(book, "ERROR", "属性组", 0, group.Name, "INVALID_SINGLE_LIMIT", "单选属性组的最多选必须为 1")
		}
		seenValues := map[string]bool{}
		defaultCount := 0
		for _, value := range group.Values {
			key := menuImportKey(value.Name)
			if seenValues[key] {
				menuImportAddIssue(book, "ERROR", "属性组", value.Row, "属性值*", "DUPLICATE_ATTRIBUTE_VALUE", "同一属性组的属性值不能重复")
			}
			seenValues[key] = true
			if value.IsDefault {
				defaultCount++
			}
		}
		if len(group.Values) < group.MinSelect {
			menuImportAddIssue(book, "ERROR", "属性组", 0, group.Name, "ATTRIBUTE_VALUES_TOO_FEW", "属性值数量少于最少选择数量")
		}
		if defaultCount > group.MaxSelect {
			menuImportAddIssue(book, "ERROR", "属性组", 0, group.Name, "ATTRIBUTE_DEFAULTS_TOO_MANY", "默认属性值数量超过最多选择数量")
		}
	}
	for _, group := range book.ModifierGroups {
		if group.MinSelect < 0 || group.MaxSelect < 1 || group.MinSelect > group.MaxSelect {
			menuImportAddIssue(book, "ERROR", "加料组", 0, group.Name, "INVALID_MODIFIER_LIMITS", "加料组的最少选/最多选范围无效")
		}
		seenItems := map[string]bool{}
		defaultCount := 0
		for _, item := range group.Items {
			key := menuImportKey(item.Name)
			if seenItems[key] {
				menuImportAddIssue(book, "ERROR", "加料组", item.Row, "加料名称*", "DUPLICATE_MODIFIER_ITEM", "同一加料组的加料名称不能重复")
			}
			seenItems[key] = true
			if item.IsDefault {
				defaultCount++
			}
		}
		if len(group.Items) < group.MinSelect {
			menuImportAddIssue(book, "ERROR", "加料组", 0, group.Name, "MODIFIER_ITEMS_TOO_FEW", "加料数量少于最少选择数量")
		}
		if defaultCount > group.MaxSelect {
			menuImportAddIssue(book, "ERROR", "加料组", 0, group.Name, "MODIFIER_DEFAULTS_TOO_MANY", "默认加料数量超过最多选择数量")
		}
	}
}

func analyzeMenuImport(ctx context.Context, queryer menuImportQueryer, tenantID, storeID int64, book *menuImportWorkbook) (menuImportResolution, error) {
	resolution := menuImportResolution{
		CategoryIDs:       map[string]menuImportNamedID{},
		AttributeGroupIDs: map[string]menuImportNamedID{},
		ModifierGroupIDs:  map[string]menuImportNamedID{},
		Issues:            append([]menuImportIssue{}, book.Issues...),
	}
	if err := loadMenuImportNamedIDs(ctx, queryer, `SELECT id,name,status FROM categories WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, tenantID, storeID, resolution.CategoryIDs, &resolution.Issues, "分类"); err != nil {
		return resolution, err
	}
	if err := loadMenuImportNamedIDs(ctx, queryer, `SELECT id,name,status FROM attribute_groups WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, tenantID, storeID, resolution.AttributeGroupIDs, &resolution.Issues, "属性组"); err != nil {
		return resolution, err
	}
	if err := loadMenuImportNamedIDs(ctx, queryer, `SELECT id,name,status FROM modifier_groups WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, tenantID, storeID, resolution.ModifierGroupIDs, &resolution.Issues, "加料组"); err != nil {
		return resolution, err
	}

	categorySeen := map[string]bool{}
	attributeSeen := map[string]bool{}
	modifierSeen := map[string]bool{}
	for _, product := range book.Products {
		categoryKey := menuImportKey(product.CategoryName)
		if !categorySeen[categoryKey] {
			categorySeen[categoryKey] = true
			if existing, ok := resolution.CategoryIDs[categoryKey]; ok {
				resolution.ExistingCategories = append(resolution.ExistingCategories, existing.Name)
				if existing.Status != "ACTIVE" {
					resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "商品", Row: product.Row, Field: "分类名称*", Code: "CATEGORY_DISABLED", Message: fmt.Sprintf("分类“%s”当前已停用；导入后需启用分类才能在点单端展示", existing.Name)})
				}
			} else {
				resolution.NewCategories = append(resolution.NewCategories, product.CategoryName)
			}
		}
		for _, name := range product.AttributeGroupNames {
			key := menuImportKey(name)
			if attributeSeen[key] {
				continue
			}
			attributeSeen[key] = true
			if existing, ok := resolution.AttributeGroupIDs[key]; ok {
				resolution.ExistingAttributeNames = append(resolution.ExistingAttributeNames, existing.Name)
				if existing.Status != "ACTIVE" {
					resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "ERROR", Sheet: "商品", Row: product.Row, Field: "点单属性组", Code: "ATTRIBUTE_GROUP_DISABLED", Message: fmt.Sprintf("属性组“%s”已停用，请先在商品配置中心启用", existing.Name)})
					continue
				}
				if _, defined := book.AttributeGroups[key]; defined {
					resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "属性组", Field: name, Code: "ATTRIBUTE_DEFINITION_IGNORED", Message: "门店已有同名属性组，将复用已有配置并忽略模板中的定义"})
				}
			} else if definition := book.AttributeGroups[key]; definition == nil {
				resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "ERROR", Sheet: "商品", Row: product.Row, Field: "点单属性组", Code: "ATTRIBUTE_GROUP_NOT_FOUND", Message: fmt.Sprintf("属性组“%s”不存在，请在属性组表完整定义", name)})
			} else {
				resolution.NewAttributeGroups = append(resolution.NewAttributeGroups, definition.Name)
			}
		}
		for _, name := range product.ModifierGroupNames {
			key := menuImportKey(name)
			if modifierSeen[key] {
				continue
			}
			modifierSeen[key] = true
			if existing, ok := resolution.ModifierGroupIDs[key]; ok {
				resolution.ExistingModifierNames = append(resolution.ExistingModifierNames, existing.Name)
				if existing.Status != "ACTIVE" {
					resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "ERROR", Sheet: "商品", Row: product.Row, Field: "加料组", Code: "MODIFIER_GROUP_DISABLED", Message: fmt.Sprintf("加料组“%s”已停用，请先在商品配置中心启用", existing.Name)})
					continue
				}
				if _, defined := book.ModifierGroups[key]; defined {
					resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "加料组", Field: name, Code: "MODIFIER_DEFINITION_IGNORED", Message: "门店已有同名加料组，将复用已有配置并忽略模板中的定义"})
				}
			} else if definition := book.ModifierGroups[key]; definition == nil {
				resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "ERROR", Sheet: "商品", Row: product.Row, Field: "加料组", Code: "MODIFIER_GROUP_NOT_FOUND", Message: fmt.Sprintf("加料组“%s”不存在，请在加料组表完整定义", name)})
			} else {
				resolution.NewModifierGroups = append(resolution.NewModifierGroups, definition.Name)
			}
		}
	}
	for key, group := range book.AttributeGroups {
		if !attributeSeen[key] {
			resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "属性组", Field: group.Name, Code: "UNUSED_ATTRIBUTE_GROUP", Message: "该属性组未被任何商品引用，本次不会创建"})
		}
	}
	for key, group := range book.ModifierGroups {
		if !modifierSeen[key] {
			resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "加料组", Field: group.Name, Code: "UNUSED_MODIFIER_GROUP", Message: "该加料组未被任何商品引用，本次不会创建"})
		}
	}

	existingProductNames := map[string]bool{}
	rows, err := queryer.QueryContext(ctx, `SELECT name FROM products WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`, tenantID, storeID)
	if err != nil {
		return resolution, err
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return resolution, err
		}
		existingProductNames[menuImportKey(name)] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return resolution, err
	}
	rows.Close()
	for _, product := range book.Products {
		if existingProductNames[menuImportKey(product.Name)] {
			resolution.Issues = append(resolution.Issues, menuImportIssue{Level: "WARNING", Sheet: "商品", Row: product.Row, Field: "商品名称*", Code: "PRODUCT_NAME_EXISTS", Message: "门店已有同名商品；本次仍会新增，不会覆盖原商品"})
		}
	}
	return resolution, nil
}

func loadMenuImportNamedIDs(ctx context.Context, queryer menuImportQueryer, query string, tenantID, storeID int64, target map[string]menuImportNamedID, issues *[]menuImportIssue, sheet string) error {
	rows, err := queryer.QueryContext(ctx, query, tenantID, storeID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item menuImportNamedID
		if err = rows.Scan(&item.ID, &item.Name, &item.Status); err != nil {
			return err
		}
		key := menuImportKey(item.Name)
		if existing, duplicate := target[key]; duplicate {
			*issues = append(*issues, menuImportIssue{Level: "WARNING", Sheet: sheet, Field: item.Name, Code: "DUPLICATE_EXISTING_NAME", Message: fmt.Sprintf("门店已有多个同名配置，将复用较早创建的“%s”（ID %d）", existing.Name, existing.ID)})
			continue
		}
		target[key] = item
	}
	return rows.Err()
}

func buildMenuImportPreview(book *menuImportWorkbook, resolution menuImportResolution) menuImportPreview {
	skuCount := 0
	products := make([]menuImportProductPreview, 0, len(book.Products))
	for _, product := range book.Products {
		skuCount += len(product.SKUs)
		products = append(products, menuImportProductPreview{
			Code: product.Code, Name: product.Name, CategoryName: product.CategoryName,
			SKUCount: len(product.SKUs), ImageCount: len(product.ImageURLs),
			AttributeGroups: product.AttributeGroupNames, ModifierGroups: product.ModifierGroupNames,
		})
	}
	issues := append([]menuImportIssue{}, resolution.Issues...)
	sort.SliceStable(issues, func(left, right int) bool {
		leftError := issues[left].Level == "ERROR"
		rightError := issues[right].Level == "ERROR"
		if leftError != rightError {
			return leftError
		}
		if issues[left].Sheet != issues[right].Sheet {
			return issues[left].Sheet < issues[right].Sheet
		}
		if issues[left].Row != issues[right].Row {
			return issues[left].Row < issues[right].Row
		}
		if issues[left].Field != issues[right].Field {
			return issues[left].Field < issues[right].Field
		}
		return issues[left].Code < issues[right].Code
	})
	return menuImportPreview{
		Valid:                   !menuImportHasErrors(issues),
		ProductCount:            len(book.Products),
		SKUCount:                skuCount,
		ExistingCategoryCount:   len(resolution.ExistingCategories),
		NewCategories:           resolution.NewCategories,
		ExistingAttributeGroups: resolution.ExistingAttributeNames,
		NewAttributeGroups:      resolution.NewAttributeGroups,
		ExistingModifierGroups:  resolution.ExistingModifierNames,
		NewModifierGroups:       resolution.NewModifierGroups,
		Products:                products,
		Issues:                  issues,
	}
}

func executeMenuImport(ctx context.Context, server *Server, tx *sql.Tx, tenantID, storeID int64, book *menuImportWorkbook, resolution menuImportResolution) (menuImportResult, error) {
	result := menuImportResult{
		CreatedCategories: resolution.NewCategories,
		CreatedAttributes: resolution.NewAttributeGroups,
		CreatedModifiers:  resolution.NewModifierGroups,
		CreatedProductIDs: []int64{},
		ProductCount:      len(book.Products),
	}
	for _, issue := range resolution.Issues {
		if issue.Level == "WARNING" {
			result.Warnings++
		}
	}
	var maxCategorySort int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order),0) FROM categories WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`, tenantID, storeID).Scan(&maxCategorySort); err != nil {
		return result, err
	}
	for index, name := range resolution.NewCategories {
		insert, err := tx.ExecContext(ctx, `INSERT INTO categories(tenant_id,store_id,name,sort_order,in_store_enabled,delivery_enabled,status) VALUES(?,?,?,?,1,0,'ACTIVE')`, tenantID, storeID, name, maxCategorySort+index+1)
		if err != nil {
			return result, err
		}
		id, _ := insert.LastInsertId()
		resolution.CategoryIDs[menuImportKey(name)] = menuImportNamedID{ID: id, Name: name, Status: "ACTIVE"}
	}
	for _, name := range resolution.NewAttributeGroups {
		definition := book.AttributeGroups[menuImportKey(name)]
		insert, err := tx.ExecContext(ctx, `INSERT INTO attribute_groups(tenant_id,store_id,name,selection_mode,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?,?, 'ACTIVE')`, tenantID, storeID, definition.Name, definition.SelectionMode, definition.MinSelect, definition.MaxSelect, len(resolution.AttributeGroupIDs)+1)
		if err != nil {
			return result, err
		}
		groupID, _ := insert.LastInsertId()
		for index, value := range definition.Values {
			if _, err = tx.ExecContext(ctx, `INSERT INTO attribute_values(tenant_id,store_id,group_id,name,price_delta_cents,is_default,sort_order,status) VALUES(?,?,?,?,?,?,?, 'ACTIVE')`, tenantID, storeID, groupID, value.Name, value.PriceDeltaCents, value.IsDefault, index); err != nil {
				return result, err
			}
		}
		resolution.AttributeGroupIDs[menuImportKey(name)] = menuImportNamedID{ID: groupID, Name: definition.Name, Status: "ACTIVE"}
	}
	modifierItems, err := loadExistingModifierItems(ctx, tx, tenantID, storeID)
	if err != nil {
		return result, err
	}
	for _, name := range resolution.NewModifierGroups {
		definition := book.ModifierGroups[menuImportKey(name)]
		insert, insertErr := tx.ExecContext(ctx, `INSERT INTO modifier_groups(tenant_id,store_id,name,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?, 'ACTIVE')`, tenantID, storeID, definition.Name, definition.MinSelect, definition.MaxSelect, len(resolution.ModifierGroupIDs)+1)
		if insertErr != nil {
			return result, insertErr
		}
		groupID, _ := insert.LastInsertId()
		for index, item := range definition.Items {
			itemID := modifierItems[menuImportKey(item.Name)].ID
			if itemID == 0 {
				itemInsert, itemErr := tx.ExecContext(ctx, `INSERT INTO modifier_items(tenant_id,store_id,name,price_cents,image_url,sort_order,status) VALUES(?,?,?,?,'',?, 'ACTIVE')`, tenantID, storeID, item.Name, item.PriceCents, len(modifierItems)+1)
				if itemErr != nil {
					return result, itemErr
				}
				itemID, _ = itemInsert.LastInsertId()
				modifierItems[menuImportKey(item.Name)] = menuImportNamedID{ID: itemID, Name: item.Name, Status: "ACTIVE"}
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO modifier_group_items(tenant_id,store_id,group_id,modifier_item_id,price_override_cents,is_default,sort_order) VALUES(?,?,?,?,?,?,?)`, tenantID, storeID, groupID, itemID, item.PriceCents, item.IsDefault, index); err != nil {
				return result, err
			}
		}
		resolution.ModifierGroupIDs[menuImportKey(name)] = menuImportNamedID{ID: groupID, Name: definition.Name, Status: "ACTIVE"}
	}

	for _, product := range book.Products {
		categoryID := resolution.CategoryIDs[menuImportKey(product.CategoryName)].ID
		mainImage := ""
		if len(product.ImageURLs) > 0 {
			mainImage = product.ImageURLs[0]
		}
		for _, imageURL := range product.ImageURLs {
			if err = server.validateManagedMediaURL(ctx, tx, tenantID, storeID, imageURL); err != nil {
				return result, err
			}
		}
		productInsert, insertErr := tx.ExecContext(ctx, `INSERT INTO products(tenant_id,store_id,category_id,name,description,image_url,recommended,member_discount_enabled,in_store_enabled,delivery_enabled,sort_order,status) VALUES(?,?,?,?,?,?,?,?,?,0,?,?)`, tenantID, storeID, categoryID, product.Name, product.Description, mainImage, product.Recommended, product.MemberDiscountEnabled, product.InStoreEnabled, product.SortOrder, menuImportStatus(product.Enabled))
		if insertErr != nil {
			return result, insertErr
		}
		productID, _ := productInsert.LastInsertId()
		result.CreatedProductIDs = append(result.CreatedProductIDs, productID)
		for index, imageURL := range product.ImageURLs {
			if _, err = tx.ExecContext(ctx, `INSERT INTO product_images(tenant_id,store_id,product_id,url,is_primary,sort_order) VALUES(?,?,?,?,?,?)`, tenantID, storeID, productID, imageURL, index == 0, index); err != nil {
				return result, err
			}
		}
		for _, sku := range product.SKUs {
			attributes := make(map[string]any, len(sku.Attributes))
			for key, value := range sku.Attributes {
				attributes[key] = value
			}
			autoSoldOut := true
			if err = insertSKU(ctx, tx, tenantID, storeID, productID, skuInput{
				Name: sku.Name, Attributes: attributes, PriceCents: sku.PriceCents,
				Status: menuImportStatus(sku.Enabled), Stock: sku.Stock, AutoSoldOut: &autoSoldOut,
			}); err != nil {
				return result, err
			}
			result.SKUCount++
		}
		for index, groupName := range product.AttributeGroupNames {
			attributeGroupID := resolution.AttributeGroupIDs[menuImportKey(groupName)].ID
			groupInsert, groupErr := tx.ExecContext(ctx, `INSERT INTO product_option_groups(tenant_id,store_id,product_id,attribute_group_id,name,kind,selection_mode,min_select,max_select,sort_order,status)
				SELECT ?,?,?,id,name,'ATTRIBUTE',selection_mode,min_select,max_select,?,status FROM attribute_groups
				WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, tenantID, storeID, productID, index, attributeGroupID, tenantID, storeID)
			if groupErr != nil {
				return result, groupErr
			}
			productGroupID, _ := groupInsert.LastInsertId()
			if _, err = tx.ExecContext(ctx, `INSERT INTO product_option_values(tenant_id,store_id,group_id,attribute_value_id,name,price_delta_cents,is_default,sort_order,status)
				SELECT ?,?,?,id,name,price_delta_cents,is_default,sort_order,status FROM attribute_values
				WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY sort_order,id`, tenantID, storeID, productGroupID, attributeGroupID, tenantID, storeID); err != nil {
				return result, err
			}
		}
		for index, groupName := range product.ModifierGroupNames {
			modifierGroupID := resolution.ModifierGroupIDs[menuImportKey(groupName)].ID
			if _, err = tx.ExecContext(ctx, `INSERT INTO product_modifier_groups(tenant_id,store_id,product_id,modifier_group_id,sort_order) VALUES(?,?,?,?,?)`, tenantID, storeID, productID, modifierGroupID, index); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func loadExistingModifierItems(ctx context.Context, queryer menuImportQueryer, tenantID, storeID int64) (map[string]menuImportNamedID, error) {
	items := map[string]menuImportNamedID{}
	rows, err := queryer.QueryContext(ctx, `SELECT id,name FROM modifier_items WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, tenantID, storeID)
	if err != nil {
		return items, err
	}
	defer rows.Close()
	for rows.Next() {
		var item menuImportNamedID
		if err = rows.Scan(&item.ID, &item.Name); err != nil {
			return items, err
		}
		if _, exists := items[menuImportKey(item.Name)]; !exists {
			items[menuImportKey(item.Name)] = item
		}
	}
	return items, rows.Err()
}

func menuImportRows(file *excelize.File, book *menuImportWorkbook, sheet string, required []string) ([][]string, map[string]int, bool) {
	found := false
	for _, name := range file.GetSheetList() {
		if name == sheet {
			found = true
			break
		}
	}
	if !found {
		menuImportAddIssue(book, "ERROR", sheet, 0, "", "MISSING_SHEET", fmt.Sprintf("缺少“%s”工作表，请使用最新模板", sheet))
		return nil, nil, false
	}
	rows, err := file.GetRows(sheet)
	if err != nil {
		menuImportAddIssue(book, "ERROR", sheet, 0, "", "SHEET_READ_FAILED", "无法读取工作表")
		return nil, nil, false
	}
	if len(rows) == 0 {
		menuImportAddIssue(book, "ERROR", sheet, 0, "", "MISSING_HEADER", "工作表缺少表头")
		return nil, nil, false
	}
	headers := map[string]int{}
	for index, value := range rows[0] {
		headers[strings.TrimSpace(value)] = index
	}
	ok := true
	for _, header := range required {
		if _, exists := headers[header]; !exists {
			menuImportAddIssue(book, "ERROR", sheet, 1, header, "MISSING_COLUMN", fmt.Sprintf("缺少必需列“%s”，请勿修改模板表头", header))
			ok = false
		}
	}
	return rows, headers, ok
}

func menuImportCell(row []string, headers map[string]int, header string) string {
	index, ok := headers[header]
	if !ok || index >= len(row) {
		return ""
	}
	return row[index]
}

func menuImportRowBlank(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func menuImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func menuImportNames(value string) []string {
	seen := map[string]bool{}
	items := []string{}
	for _, part := range strings.FieldsFunc(value, func(char rune) bool {
		return char == '|' || char == '｜' || char == ';' || char == '；'
	}) {
		name := strings.TrimSpace(part)
		key := menuImportKey(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, name)
	}
	return items
}

func menuImportBool(book *menuImportWorkbook, sheet string, row int, field, raw string, defaultValue bool) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return defaultValue
	}
	switch raw {
	case "是", "yes", "y", "true", "1":
		return true
	case "否", "no", "n", "false", "0":
		return false
	default:
		menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_BOOLEAN", "请填写“是”或“否”")
		return defaultValue
	}
}

func menuImportMoney(book *menuImportWorkbook, sheet string, row int, field, raw string, required bool) int64 {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(raw), "¥"), "￥"))
	raw = strings.ReplaceAll(raw, ",", "")
	if raw == "" {
		if required {
			menuImportAddIssue(book, "ERROR", sheet, row, field, "MONEY_REQUIRED", "金额不能为空")
		}
		return 0
	}
	if strings.HasPrefix(raw, "-") {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_MONEY", "金额必须是非负数，最多两位小数")
		return 0
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && len(parts[1]) > 2 {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_MONEY", "金额必须是非负数，最多两位小数")
		return 0
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_MONEY", "金额必须是非负数，最多两位小数")
		return 0
	}
	if yuan > maxCatalogUnitPriceCents/100 {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "MONEY_TOO_LARGE", "单价不能超过 100,000 元")
		return 0
	}
	fraction := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		fractionText := parts[1]
		if len(fractionText) == 1 {
			fractionText += "0"
		}
		fraction, err = strconv.ParseInt(fractionText, 10, 64)
		if err != nil {
			menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_MONEY", "金额必须是非负数，最多两位小数")
			return 0
		}
	}
	cents := yuan*100 + fraction
	if cents > maxCatalogUnitPriceCents {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "MONEY_TOO_LARGE", "单价不能超过 100,000 元")
		return 0
	}
	return cents
}

func menuImportInteger(book *menuImportWorkbook, sheet string, row int, field, raw string, required bool) int {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		if required {
			menuImportAddIssue(book, "ERROR", sheet, row, field, "INTEGER_REQUIRED", "该字段不能为空")
		}
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 0 {
		menuImportAddIssue(book, "ERROR", sheet, row, field, "INVALID_INTEGER", "请填写非负整数")
		return 0
	}
	return int(value)
}

func menuImportAddIssue(book *menuImportWorkbook, level, sheet string, row int, field, code, message string) {
	book.Issues = append(book.Issues, menuImportIssue{Level: level, Sheet: sheet, Row: row, Field: field, Code: code, Message: message})
}

func menuImportHasErrors(issues []menuImportIssue) bool {
	for _, issue := range issues {
		if issue.Level == "ERROR" {
			return true
		}
	}
	return false
}

func firstMenuImportError(issues []menuImportIssue) string {
	for _, issue := range issues {
		if issue.Level == "ERROR" {
			location := issue.Sheet
			if issue.Row > 0 {
				location += fmt.Sprintf("第%d行", issue.Row)
			}
			return location + "：" + issue.Message
		}
	}
	return "导入文件校验失败"
}

func menuImportStatus(enabled bool) string {
	if enabled {
		return "ACTIVE"
	}
	return "DISABLED"
}

func init() {
	if len(menuImportTemplate) == 0 {
		panic("menu import template is unavailable")
	}
}
