package app

import (
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
