package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const beijingDateTimeLayout = "2006-01-02 15:04:05"

var beijingLocation = func() *time.Location {
	location, err := time.LoadLocation(defaultStoreTimezone)
	if err != nil {
		panic("load Asia/Shanghai timezone: " + err.Error())
	}
	return location
}()

// formatBeijingDateTime is the only timestamp shape exposed to Tanban's own
// clients. Storage and provider integrations may use absolute instants, while
// operators and customers always read a Beijing wall-clock time.
func formatBeijingDateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(beijingLocation).Format(beijingDateTimeLayout)
}

// parseBeijingDateTime accepts the new Beijing wall-clock contract and keeps
// RFC3339 compatibility for already released clients during rolling deploys.
func parseBeijingDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.ParseInLocation(beijingDateTimeLayout, value, beijingLocation); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.In(beijingLocation), nil
}

// requestDateTime keeps JSON request decoding aligned with Tanban's public
// date-time contract. Go's time.Time decoder only accepts RFC3339, while our
// APIs also document and expose Beijing wall-clock strings.
type requestDateTime struct {
	time.Time
}

func (value *requestDateTime) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("date-time must be a string: %w", err)
	}
	parsed, err := parseBeijingDateTime(raw)
	if err != nil {
		return fmt.Errorf("date-time must use %s or RFC3339: %w", beijingDateTimeLayout, err)
	}
	value.Time = parsed
	return nil
}

func requestDateTimeArg(value *requestDateTime) any {
	if value == nil {
		return nil
	}
	return formatBeijingDateTime(value.Time)
}

func requestDateTimeWindowValid(from, to *requestDateTime) bool {
	if (from != nil && from.IsZero()) || (to != nil && to.IsZero()) {
		return false
	}
	return from == nil || to == nil || from.Time.Before(to.Time)
}
