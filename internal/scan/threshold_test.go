package scan

import (
	"testing"
	"time"
)

func TestParseThreshold(t *testing.T) {
	cases := map[string]time.Duration{
		"90d": 90 * 24 * time.Hour,
		"6m":  6 * 30 * 24 * time.Hour,
		"1y":  365 * 24 * time.Hour,
		"2y":  2 * 365 * 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseThreshold(in)
		if err != nil || got != want {
			t.Errorf("ParseThreshold(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "12", "d", "1w", "-1y", "1.5y"} {
		if _, err := ParseThreshold(bad); err == nil {
			t.Errorf("ParseThreshold(%q) should fail", bad)
		}
	}
}
