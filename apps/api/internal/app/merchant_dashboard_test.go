package app

import (
	"fmt"
	"testing"
	"time"
)

func TestMonthlyTreandGapFill(t *testing.T) {
	// Simulate the Go gap-fill logic from the dashboard handler
	testFill := func(dbRows map[string]float64, year int, month time.Month, today int) []string {
		monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		todayEnd := time.Date(year, month, today, 0, 0, 0, 0, time.UTC)
		var labels []string
		for d := monthStart; !d.After(todayEnd); d = d.AddDate(0, 0, 1) {
			key := d.Format("01-02")
			labels = append(labels, key)
		}
		return labels
	}

	// Today: 2026-07-29, should produce labels from 07-01 to 07-29
	labels := testFill(map[string]float64{}, 2026, 7, 29)
	if len(labels) != 29 {
		t.Fatalf("expected 29 days, got %d", len(labels))
	}
	if labels[0] != "07-01" {
		t.Fatalf("first label should be 07-01, got %s", labels[0])
	}
	if labels[28] != "07-29" {
		t.Fatalf("last label should be 07-29, got %s", labels[28])
	}
}

func TestTodayHourlyFill(t *testing.T) {
	// Simulate the 24-hour fill logic
	hourMap := map[int]int{9: 5, 14: 3}

	result := make([]map[string]any, 24)
	for i := 0; i < 24; i++ {
		result[i] = map[string]any{"hour": formatHour(i), "count": hourMap[i]}
	}

	// Verify length
	if len(result) != 24 {
		t.Fatalf("expected 24 hours, got %d", len(result))
	}

	// Verify format
	if result[0]["hour"] != "00:00" {
		t.Fatalf("first hour should be 00:00, got %v", result[0]["hour"])
	}
	if result[23]["hour"] != "23:00" {
		t.Fatalf("last hour should be 23:00, got %v", result[23]["hour"])
	}

	// Verify populated values
	if result[9]["count"] != 5 {
		t.Fatalf("hour 9 should have count 5, got %v", result[9]["count"])
	}
	if result[14]["count"] != 3 {
		t.Fatalf("hour 14 should have count 3, got %v", result[14]["count"])
	}

	// Verify zero-filled gap
	if result[10]["count"] != 0 {
		t.Fatalf("hour 10 should have count 0, got %v", result[10]["count"])
	}
}

func formatHour(h int) string {
	return fmt.Sprintf("%02d:00", h)
}
