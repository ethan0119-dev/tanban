package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestUpdateProductAcceptsUnchangedBaseFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT store_id,recommended,in_store_enabled,delivery_enabled FROM products").
		WithArgs(int64(17), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"store_id", "recommended", "in_store_enabled", "delivery_enabled"}).AddRow(9, false, true, false))
	mock.ExpectQuery("SELECT id FROM stores").
		WithArgs(int64(9), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM categories").
		WithArgs(int64(3), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	// This is the regression condition: MySQL matched the product but changed
	// no base columns. SKU and inventory processing must still continue.
	mock.ExpectExec("UPDATE products SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT id FROM skus").
		WithArgs(int64(17), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectExec("UPDATE skus SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT stock FROM inventory").
		WithArgs(int64(21), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"stock"}).AddRow(10))
	mock.ExpectExec("UPDATE inventory SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE product_images SET deleted_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id,store_id,category_id,name,description,image_url,recommended,in_store_enabled,delivery_enabled,sort_order,status FROM products").
		WithArgs(int64(5), int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "store_id", "category_id", "name", "description", "image_url", "recommended", "in_store_enabled", "delivery_enabled", "sort_order", "status"}).
			AddRow(17, 9, 3, "拿铁", "", "", false, true, false, 0, "ACTIVE"))
	mock.ExpectQuery("SELECT id,media_asset_id,url,is_primary,sort_order FROM product_images").
		WithArgs(int64(5), int64(9), int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "media_asset_id", "url", "is_primary", "sort_order"}))
	mock.ExpectQuery("SELECT s.id,s.name,s.attributes_json,s.price_cents,s.status,i.stock,i.auto_sold_out,i.auto_refill,i.refill_stock").
		WithArgs(int64(5), int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "attributes_json", "price_cents", "status", "stock", "auto_sold_out", "auto_refill", "refill_stock"}).
			AddRow(21, "默认规格", `{}`, 1800, "ACTIVE", 10, true, false, 0))
	mock.ExpectQuery("SELECT oi.product_id,COALESCE\\(SUM\\(oi.quantity\\),0\\)").
		WithArgs(int64(5), int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "quantity"}))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Put("/products/{productID}", server.updateProduct)
	request := httptest.NewRequest(http.MethodPut, "/products/17", bytes.NewBufferString(`{
		"category_id":3,"name":"拿铁","description":"","image_url":"","images":[],
		"recommended":false,"in_store_enabled":true,"delivery_enabled":false,"sort_order":0,"status":"ACTIVE",
		"skus":[{"id":21,"name":"默认规格","attributes":{},"price_cents":1800,"status":"ACTIVE","stock":10,
		"expected_stock":10,"auto_sold_out":true,"auto_refill":false,"refill_stock":0}]
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

func TestUpdateProductConfigurationUsesOwningStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT store_id FROM products").
		WithArgs(int64(17), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"store_id"}).AddRow(9))
	mock.ExpectExec("DELETE v FROM product_option_values").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM product_option_groups").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM product_modifier_groups").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM product_resource_bindings").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM products").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id,name,kind,selection_mode").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "kind", "selection_mode", "min_select", "max_select", "sort_order", "status"}))
	mock.ExpectQuery("SELECT mg.id,mg.name,mg.min_select").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "min_select", "max_select", "sort_order", "status"}))
	mock.ExpectQuery("SELECT resource_id FROM product_resource_bindings").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id"}))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Put("/products/{productID}/configuration", server.updateProductConfiguration)
	request := httptest.NewRequest(http.MethodPut, "/products/17/configuration", bytes.NewBufferString(`{"option_groups":[],"modifier_group_ids":[],"resource_ids":[]}`))
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
