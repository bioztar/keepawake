package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultEvery = 30 * time.Second

// parseDuration reads --on values: 30m, 2h, 90s, or bare seconds (same forms as macos/keepawake).
func parseDuration(s string) (time.Duration, error) {
	orig := s
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, errors.New("empty duration")
	}
	unit := time.Second
	switch s[len(s)-1] {
	case 'h':
		unit, s = time.Hour, s[:len(s)-1]
	case 'm':
		unit, s = time.Minute, s[:len(s)-1]
	case 's':
		s = s[:len(s)-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("bad duration %q (use e.g. 30m, 2h, 90s)", orig)
	}
	if int64(n) > int64(365*24*time.Hour/unit) {
		return 0, fmt.Errorf("duration %q is too long (max 365 days)", orig)
	}
	return time.Duration(n) * unit, nil
}

// parseEvery reads KEEPAWAKE_EVERY (whole seconds); empty means the default.
func parseEvery(env string) (time.Duration, error) {
	env = strings.TrimSpace(env)
	if env == "" {
		return defaultEvery, nil
	}
	n, err := strconv.Atoi(env)
	if err != nil || n <= 0 || n > 24*3600 {
		return 0, fmt.Errorf("bad KEEPAWAKE_EVERY %q (whole seconds, e.g. 30)", env)
	}
	return time.Duration(n) * time.Second, nil
}
