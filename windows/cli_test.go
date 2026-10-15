package main

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	ok := map[string]time.Duration{
		"90s": 90 * time.Second, "90": 90 * time.Second, "30m": 30 * time.Minute,
		"2h": 2 * time.Hour, "2H": 2 * time.Hour, " 5s ": 5 * time.Second, "1s": time.Second,
	}
	for in, want := range ok {
		got, err := parseDuration(in)
		if err != nil || got != want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "h", "0", "0m", "-5m", "1.5h", "2d", "abc", "1h30m", "9999999h"} {
		if d, err := parseDuration(in); err == nil {
			t.Errorf("parseDuration(%q) = %v; want error", in, d)
		}
	}
}

func TestParseEvery(t *testing.T) {
	if d, err := parseEvery(""); err != nil || d != 30*time.Second {
		t.Errorf("default = %v, %v", d, err)
	}
	if d, err := parseEvery("60"); err != nil || d != time.Minute {
		t.Errorf("60 = %v, %v", d, err)
	}
	for _, in := range []string{"0", "-1", "abc", "30s", "1.5", "86401"} {
		if _, err := parseEvery(in); err == nil {
			t.Errorf("parseEvery(%q): want error", in)
		}
	}
}
