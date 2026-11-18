package plugin

import (
	"net/http"
	"testing"
	"time"
)

func TestSnapInterval(t *testing.T) {
	for in, want := range map[time.Duration]time.Duration{
		0:                   time.Second,
		time.Second:         time.Second,
		2 * time.Second:     5 * time.Second,
		20 * time.Second:    time.Minute,
		90 * time.Second:    5 * time.Minute,
		2 * time.Hour:       6 * time.Hour,
		30 * 24 * time.Hour: 7 * 24 * time.Hour,
	} {
		if got := snapInterval(in); got != want {
			t.Errorf("snap(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestEffectiveInterval(t *testing.T) {
	at := func(h, m, s int) time.Time { return time.Date(2026, 11, 3, h, m, s, 0, time.UTC) }
	if got := effectiveInterval(5*time.Second, nil); got != 5*time.Second {
		t.Errorf("no rows: %v", got)
	}
	minute := []time.Time{at(8, 1, 0), at(8, 2, 0)}
	if got := effectiveInterval(5*time.Second, minute); got != time.Minute {
		t.Errorf("minute-aligned: %v", got)
	}
	hour := []time.Time{at(8, 0, 0), at(9, 0, 0)}
	if got := effectiveInterval(time.Minute, hour); got != time.Hour {
		t.Errorf("hour-aligned: %v", got)
	}
	if got := effectiveInterval(5*time.Second, []time.Time{at(8, 0, 5)}); got != 5*time.Second {
		t.Errorf("unaligned: %v", got)
	}
	// A requested interval at or above the forced step is left alone.
	if got := effectiveInterval(5*time.Minute, minute); got != 5*time.Minute {
		t.Errorf("already coarse: %v", got)
	}
}

func TestReportedInterval(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"60000": time.Minute, "3600000": time.Hour, "": 0, "abc": 0, "0": 0, "-1": 0,
	} {
		h := http.Header{}
		if in != "" {
			h.Set(intervalHeader, in)
		}
		if got := reportedInterval(h); got != want {
			t.Errorf("header %q = %v, want %v", in, got, want)
		}
	}
}
