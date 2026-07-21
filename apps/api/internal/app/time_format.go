package app

import (
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
