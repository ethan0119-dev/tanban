package app

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFormatBeijingDateTime(t *testing.T) {
	t.Parallel()
	value := time.Date(2026, time.July, 21, 6, 0, 0, 0, time.UTC)
	if got := formatBeijingDateTime(value); got != "2026-07-21 14:00:00" {
		t.Fatalf("formatBeijingDateTime=%q", got)
	}
}

func TestParseBeijingDateTimeSupportsWallClockAndRFC3339(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"2026-07-21 14:00:00", "2026-07-21T14:00:00+08:00"} {
		parsed, err := parseBeijingDateTime(value)
		if err != nil {
			t.Fatalf("parse %q: %v", value, err)
		}
		if got := formatBeijingDateTime(parsed); got != "2026-07-21 14:00:00" {
			t.Fatalf("parse %q formatted as %q", value, got)
		}
	}
}

func TestRequestDateTimeJSONSupportsWallClockAndRFC3339(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"2026-07-23 00:00:00", "2026-07-23T00:00:00+08:00"} {
		var decoded struct {
			StartsAt requestDateTime  `json:"starts_at"`
			EndsAt   *requestDateTime `json:"ends_at"`
		}
		body := []byte(`{"starts_at":"` + value + `","ends_at":"` + value + `"}`)
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("unmarshal %q: %v", value, err)
		}
		if got := formatBeijingDateTime(decoded.StartsAt.Time); got != "2026-07-23 00:00:00" {
			t.Fatalf("starts_at %q formatted as %q", value, got)
		}
		if decoded.EndsAt == nil || formatBeijingDateTime(decoded.EndsAt.Time) != "2026-07-23 00:00:00" {
			t.Fatalf("ends_at %q was not decoded", value)
		}
	}
}

func TestRequestDateTimeJSONRejectsInvalidFormatAndAllowsNullPointer(t *testing.T) {
	t.Parallel()
	var decoded struct {
		StartsAt *requestDateTime `json:"starts_at"`
	}
	if err := json.Unmarshal([]byte(`{"starts_at":null}`), &decoded); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if decoded.StartsAt != nil {
		t.Fatal("starts_at should remain nil")
	}
	if err := json.Unmarshal([]byte(`{"starts_at":"2026/07/23 00:00:00"}`), &decoded); err == nil {
		t.Fatal("invalid date-time format should be rejected")
	}
}
