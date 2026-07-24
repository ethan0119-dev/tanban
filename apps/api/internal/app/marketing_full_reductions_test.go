package app

import (
	"reflect"
	"testing"
)

func TestNormalizeFullReductionInput(t *testing.T) {
	t.Parallel()
	input := fullReductionInput{
		Name: " 满 50 减 8 ", ThresholdCents: 5000, DiscountCents: 800,
		OrderTypes: []string{"takeout", "dine_in", "takeout"},
	}
	raw, err := normalizeFullReductionInput(&input)
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "满 50 减 8" {
		t.Fatalf("name was not trimmed: %q", input.Name)
	}
	if got := decodeMarketingOrderTypes(raw); !reflect.DeepEqual(got, []string{"DINE_IN", "TAKEOUT"}) {
		t.Fatalf("order types = %#v", got)
	}
}

func TestBestStoreFullReductionUsesHighestEligibleDiscount(t *testing.T) {
	t.Parallel()
	rows := []fullReductionRow{
		{ID: 2, ThresholdCents: 8000, DiscountCents: 1500, OrderTypesJSON: `["DINE_IN","TAKEOUT"]`},
		{ID: 1, ThresholdCents: 5000, DiscountCents: 800, OrderTypesJSON: `["DINE_IN","TAKEOUT"]`},
	}
	if got := bestStoreFullReduction(rows, 6000, "TAKEOUT"); got.ID != 1 {
		t.Fatalf("campaign = %d, want 1", got.ID)
	}
	if got := bestStoreFullReduction(rows, 9000, "TAKEOUT"); got.ID != 2 {
		t.Fatalf("campaign = %d, want 2", got.ID)
	}
	if got := bestStoreFullReduction(rows, 9000, "DELIVERY"); got.ID != 0 {
		t.Fatalf("unexpected campaign = %d", got.ID)
	}
}

func TestNormalizeFullReductionRejectsDiscountAboveThreshold(t *testing.T) {
	t.Parallel()
	input := fullReductionInput{Name: "错误活动", ThresholdCents: 500, DiscountCents: 501}
	if _, err := normalizeFullReductionInput(&input); err == nil {
		t.Fatal("expected validation error")
	}
}
