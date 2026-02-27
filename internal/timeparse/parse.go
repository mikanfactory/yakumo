package timeparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseCreatedAt parses a created-at value as either a Unix millisecond
// timestamp or a relative duration (e.g., "10m", "5min", "1h30m").
// For relative durations, it subtracts the duration from now.
func ParseCreatedAt(value string, now time.Time) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty value")
	}

	// Try Unix milliseconds first
	if ms, err := strconv.ParseInt(value, 10, 64); err == nil {
		return ms, nil
	}

	// Normalize "min" suffix to "m" for time.ParseDuration compatibility
	if strings.HasSuffix(value, "min") {
		value = value[:len(value)-3] + "m"
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", value, err)
	}

	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %v", d)
	}

	return now.Add(-d).UnixMilli(), nil
}
