package timeparse_test

import (
	"testing"
	"time"

	"github.com/mikanfactory/yakumo/internal/timeparse"
)

func TestParseCreatedAt(t *testing.T) {
	now := time.Date(2025, 6, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		// Unix milliseconds (backward compatibility)
		{
			name:  "unix millis",
			value: "1719000000000",
			want:  1719000000000,
		},
		{
			name:  "unix millis zero",
			value: "0",
			want:  0,
		},

		// Relative durations
		{
			name:  "30 seconds ago",
			value: "30s",
			want:  now.Add(-30 * time.Second).UnixMilli(),
		},
		{
			name:  "5 minutes ago with m",
			value: "5m",
			want:  now.Add(-5 * time.Minute).UnixMilli(),
		},
		{
			name:  "10 minutes ago with m",
			value: "10m",
			want:  now.Add(-10 * time.Minute).UnixMilli(),
		},
		{
			name:  "5 minutes ago with min",
			value: "5min",
			want:  now.Add(-5 * time.Minute).UnixMilli(),
		},
		{
			name:  "10 minutes ago with min",
			value: "10min",
			want:  now.Add(-10 * time.Minute).UnixMilli(),
		},
		{
			name:  "1 hour ago",
			value: "1h",
			want:  now.Add(-1 * time.Hour).UnixMilli(),
		},
		{
			name:  "1 hour 30 minutes ago",
			value: "1h30m",
			want:  now.Add(-1*time.Hour - 30*time.Minute).UnixMilli(),
		},

		// Whitespace handling
		{
			name:  "leading and trailing spaces",
			value: "  10m  ",
			want:  now.Add(-10 * time.Minute).UnixMilli(),
		},

		// Error cases
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
		{
			name:    "invalid word",
			value:   "yesterday",
			wantErr: true,
		},
		{
			name:    "invalid unit",
			value:   "10x",
			wantErr: true,
		},
		{
			name:    "zero duration",
			value:   "0s",
			wantErr: true,
		},
		{
			name:    "negative duration",
			value:   "-5m",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeparse.ParseCreatedAt(tt.value, now)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCreatedAt(%q) expected error, got %d", tt.value, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseCreatedAt(%q) unexpected error: %v", tt.value, err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseCreatedAt(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
