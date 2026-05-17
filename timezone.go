package main

import (
	"fmt"
	"strings"
	"time"
)

// LoadTimezone loads the IANA timezone from config (e.g. Europe/Istanbul).
func LoadTimezone(cfg Config) (*time.Location, error) {
	name := strings.TrimSpace(cfg.UTC)
	if name == "" {
		name = "Europe/Istanbul"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid utc timezone %q: %w", name, err)
	}
	return loc, nil
}

// parseDateTimeInZone parses "2026-05-17T10:15" as wall time in the given zone.
func parseDateTimeInZone(loc *time.Location, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty datetime")
	}

	layouts := []string{"2006-01-02T15:04", "2006-01-02 15:04"}
	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.ParseInLocation(layout, s, loc)
		if err == nil {
			return t.Unix(), nil
		}
	}
	return 0, err
}

// formatDateTimeInZone formats unix time for HTML datetime-local in the given zone.
func formatDateTimeInZone(loc *time.Location, unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).In(loc).Format("2006-01-02T15:04")
}

// formatWallClock formats a time for display using the app timezone.
func formatWallClock(loc *time.Location, t time.Time) string {
	return t.In(loc).Format("2006-01-02 15:04")
}
