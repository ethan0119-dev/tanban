package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResolveProductImagesEnforcesPrimaryAndGalleryLimit(t *testing.T) {
	t.Parallel()
	images := []productImageInput{
		{URL: "https://cdn.example.com/main.png", IsPrimary: true},
		{URL: "https://cdn.example.com/one.png"},
		{URL: "https://cdn.example.com/two.png"},
		{URL: "https://cdn.example.com/three.png"},
	}
	resolved, mainURL, err := resolveProductImages(context.Background(), nil, 1, 2, images, "")
	if err != nil {
		t.Fatalf("valid image set rejected: %v", err)
	}
	if len(resolved) != 4 || mainURL != images[0].URL {
		t.Fatalf("unexpected resolved images: %#v main=%q", resolved, mainURL)
	}
	if _, _, err = resolveProductImages(context.Background(), nil, 1, 2, append(images, productImageInput{URL: "https://cdn.example.com/four.png"}), ""); err == nil {
		t.Fatal("more than one primary plus three gallery images must be rejected")
	}
	if _, _, err = resolveProductImages(context.Background(), nil, 1, 2, []productImageInput{{URL: "https://cdn.example.com/no-primary.png"}}, ""); err == nil {
		t.Fatal("an image set without a primary image must be rejected")
	}
	if _, _, err = resolveProductImages(context.Background(), nil, 1, 2, []productImageInput{
		{URL: "https://cdn.example.com/duplicate.png", IsPrimary: true},
		{URL: "https://cdn.example.com/duplicate.png"},
	}, ""); err == nil {
		t.Fatal("duplicate images must be rejected")
	}
}

func TestResolveProductImagesValidatesLegacyURL(t *testing.T) {
	t.Parallel()
	if _, _, err := resolveProductImages(context.Background(), nil, 1, 2, nil, "javascript:alert(1)"); err == nil {
		t.Fatal("legacy image_url must use the same URL validation as the image collection")
	}
	tooLong := "https://cdn.example.com/" + strings.Repeat("a", 1024)
	if _, _, err := resolveProductImages(context.Background(), nil, 1, 2, nil, tooLong); err == nil {
		t.Fatal("legacy image_url longer than the database contract must be rejected")
	}
}

func TestResolveProductImagesRejectsCrossStoreMediaAsset(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT url FROM media_assets").WithArgs(int64(44), int64(5), int64(9)).WillReturnError(sql.ErrNoRows)
	_, _, err = resolveProductImages(context.Background(), db, 5, 9, []productImageInput{{MediaAssetID: 44, IsPrimary: true}}, "")
	if err == nil {
		t.Fatal("cross-store or missing media asset must be rejected")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCopyProductClonesOrderingConfigurationAndStartsDisabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO products").
		WithArgs("拿铁（副本）", int64(17), int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(81, 1))
	mock.ExpectQuery("SELECT s.name,s.attributes_json").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "attributes_json", "price_cents", "status", "stock", "auto_sold_out", "auto_refill", "refill_stock"}).
			AddRow("标准", `{}`, 1800, "ACTIVE", 8, true, false, 20))
	mock.ExpectExec("INSERT INTO skus").
		WithArgs(int64(5), int64(9), int64(81), "标准", `{}`, int64(1800), "ACTIVE").
		WillReturnResult(sqlmock.NewResult(91, 1))
	mock.ExpectExec("INSERT INTO inventory").
		WithArgs(int64(91), int64(5), int64(9), 8, true, false, 20).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO product_images").
		WithArgs(int64(81), int64(17), int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO product_resource_bindings").
		WithArgs(int64(81), int64(17), int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO product_modifier_groups").
		WithArgs(int64(81), int64(17), int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id,name,kind,selection_mode").
		WithArgs(int64(17), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "kind", "selection_mode", "min_select", "max_select", "sort_order", "status"}).
			AddRow(22, "温度", "ATTRIBUTE", "SINGLE", 1, 1, 0, "ACTIVE"))
	mock.ExpectQuery("SELECT name,price_delta_cents,is_default").
		WithArgs(int64(22), int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price_delta_cents", "is_default", "sort_order", "status"}).
			AddRow("热", 0, true, 0, "ACTIVE"))
	mock.ExpectExec("INSERT INTO product_option_groups").
		WithArgs(int64(5), int64(9), int64(81), "温度", "ATTRIBUTE", "SINGLE", 1, 1, 0, "ACTIVE").
		WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectExec("INSERT INTO product_option_values").
		WithArgs(int64(5), int64(9), int64(101), "热", int64(0), true, 0, "ACTIVE").
		WillReturnResult(sqlmock.NewResult(111, 1))
	mock.ExpectRollback()

	copyID, err := copyProduct(context.Background(), tx, 5, 9, 17, "拿铁")
	if err != nil {
		t.Fatalf("copy product: %v", err)
	}
	if copyID != 81 {
		t.Fatalf("unexpected copy id %d", copyID)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMediaGroupIDFailsClosedAcrossStores(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id FROM media_asset_groups").WithArgs(int64(4), int64(5), int64(9)).WillReturnError(sql.ErrNoRows)
	err = validateMediaGroupID(context.Background(), db, 5, 9, 4)
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected a domain validation error, got %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
