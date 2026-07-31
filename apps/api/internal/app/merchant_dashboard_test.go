package app

import (
	"context"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFillMonthlyRevenueTrendIncludesZeroRevenueDays(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, beijingLocation)
	got := fillMonthlyRevenueTrend(now, map[string]float64{
		"07-01": 123.45,
		"07-03": 9.8,
	})
	want := []dashboardRevenuePoint{
		{Label: "07-01", Value: 123.45},
		{Label: "07-02", Value: 0},
		{Label: "07-03", Value: 9.8},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fillMonthlyRevenueTrend()=%#v, want %#v", got, want)
	}
}

func TestBuildTodayOrderDistributionsAggregatesAllTypesAndHours(t *testing.T) {
	orderTypes, hourly := buildTodayOrderDistributions([]dashboardOrderBucket{
		{OrderType: "DINE_IN", Hour: 9, Count: 2},
		{OrderType: "dine_in", Hour: 10, Count: 3},
		{OrderType: "TAKEOUT", Hour: 9, Count: 4},
		{OrderType: "DELIVERY", Hour: 18, Count: 1},
		{OrderType: "CURBSIDE", Hour: 18, Count: 2},
		{OrderType: "", Hour: 10, Count: 8},
		{OrderType: "TAKEOUT", Hour: 24, Count: 9},
	})

	wantOrderTypes := []dashboardOrderTypePoint{
		{Type: "DINE_IN", Value: 5},
		{Type: "TAKEOUT", Value: 4},
		{Type: "DELIVERY", Value: 1},
		{Type: "CURBSIDE", Value: 2},
	}
	if !reflect.DeepEqual(orderTypes, wantOrderTypes) {
		t.Fatalf("order types=%#v, want %#v", orderTypes, wantOrderTypes)
	}
	if len(hourly) != 24 {
		t.Fatalf("hourly points=%d, want 24", len(hourly))
	}
	for _, expected := range []dashboardHourlyPoint{
		{Hour: "09:00", Count: 6},
		{Hour: "10:00", Count: 3},
		{Hour: "18:00", Count: 3},
	} {
		hour := int(expected.Hour[0]-'0')*10 + int(expected.Hour[1]-'0')
		if hourly[hour] != expected {
			t.Fatalf("%s point=%#v, want %#v", expected.Hour, hourly[hour], expected)
		}
	}
}

func TestLoadDashboardOrderItemsBatchesAndGroupsRecentOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT order_id,product_name,sku_name,quantity
		FROM order_items
		WHERE tenant_id=? AND order_id IN (?,?,?) AND quantity>0
		ORDER BY order_id,id`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(int64(7), int64(31), int64(32), int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "product_name", "sku_name", "quantity"}).
			AddRow(31, "经典美式", "小杯", 1).
			AddRow(31, "可颂", "默认", 2).
			AddRow(33, "生椰拿铁", "大杯", 1))

	got, err := loadDashboardOrderItems(context.Background(), db, 7, []int64{31, 32, 33})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64][]dashboardOrderItem{
		31: {
			{ProductName: "经典美式", SKUName: "小杯", Quantity: 1},
			{ProductName: "可颂", SKUName: "默认", Quantity: 2},
		},
		32: {},
		33: {
			{ProductName: "生椰拿铁", SKUName: "大杯", Quantity: 1},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadDashboardOrderItems()=%#v, want %#v", got, want)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDashboardOrderItemsSkipsQueryForEmptyOrderList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	got, err := loadDashboardOrderItems(context.Background(), db, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("loadDashboardOrderItems()=%#v, want empty map", got)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
