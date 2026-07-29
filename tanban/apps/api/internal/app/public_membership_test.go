package app

import "testing"

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
