package app

import "testing"

func TestPickupDisplayColumnKeepsOnlyActiveTakeoutStages(t *testing.T) {
	tests := map[string]string{
		"PAID":            "PREPARING",
		"accepted":        "PREPARING",
		" PREPARING ":     "PREPARING",
		"READY":           "READY",
		"PENDING_PAYMENT": "",
		"COMPLETED":       "",
		"CLOSED":          "",
		"REFUNDED":        "",
	}
	for status, expected := range tests {
		if actual := pickupDisplayColumn(status); actual != expected {
			t.Fatalf("pickupDisplayColumn(%q)=%q want %q", status, actual, expected)
		}
	}
}
