package app

import (
	"database/sql"
	"testing"
)

func TestDiscountedMemberPriceRoundsToNearestCent(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		percent int
		want    int64
	}{
		{name: "88 percent", amount: 1290, percent: 88, want: 1135},
		{name: "invalid zero keeps original", amount: 1290, percent: 0, want: 1290},
		{name: "full price keeps original", amount: 1290, percent: 100, want: 1290},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := discountedMemberPrice(test.amount, test.percent); got != test.want {
				t.Fatalf("discountedMemberPrice(%d, %d)=%d, want %d", test.amount, test.percent, got, test.want)
			}
		})
	}
}

func TestDecodeMemberDiscount(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{raw: `{"discount":88}`, want: 88},
		{raw: `{"discount":"95"}`, want: 95},
		{raw: `{"discount":0}`, want: 100},
		{raw: `not-json`, want: 100},
	}
	for _, test := range tests {
		if got := decodeMemberDiscount(test.raw); got != test.want {
			t.Fatalf("decodeMemberDiscount(%q)=%d, want %d", test.raw, got, test.want)
		}
	}
}

func TestMemberLevelUpgradeAllowedOnlyMovesUpward(t *testing.T) {
	tests := []struct {
		name        string
		currentRank sql.NullInt64
		targetRank  int
		want        bool
	}{
		{name: "no current level", currentRank: sql.NullInt64{}, targetRank: 1, want: true},
		{name: "higher rank", currentRank: sql.NullInt64{Int64: 2, Valid: true}, targetRank: 3, want: true},
		{name: "same rank", currentRank: sql.NullInt64{Int64: 2, Valid: true}, targetRank: 2, want: false},
		{name: "lower rank", currentRank: sql.NullInt64{Int64: 2, Valid: true}, targetRank: 1, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := memberLevelUpgradeAllowed(test.currentRank, test.targetRank); got != test.want {
				t.Fatalf("memberLevelUpgradeAllowed(%v, %d)=%t, want %t", test.currentRank, test.targetRank, got, test.want)
			}
		})
	}
}

func TestStoredValueRechargeAmountAllowedHonorsConfiguredRange(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   bool
	}{
		{name: "minimum", amount: 1000, want: true},
		{name: "within range", amount: 20000, want: true},
		{name: "maximum", amount: 50000, want: true},
		{name: "below minimum", amount: 999, want: false},
		{name: "above maximum", amount: 50001, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := storedValueRechargeAmountAllowed(test.amount, 1000, 50000); got != test.want {
				t.Fatalf("storedValueRechargeAmountAllowed(%d)=%t, want %t", test.amount, got, test.want)
			}
		})
	}
}
