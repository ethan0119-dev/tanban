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
