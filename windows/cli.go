package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const defaultEvery = 30 * time.Second

// parseDuration accepts the same forms as macos/keepawake: 30m, 2h, 90s, or bare seconds.
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

// nextNap is how long to sleep before the next nudge; done reports that the deadline has passed.
// A zero deadline means run until interrupted.
func nextNap(every time.Duration, deadline, now time.Time) (nap time.Duration, done bool) {
	if deadline.IsZero() {
		return every, false
	}
	left := deadline.Sub(now)
	if left <= 0 {
		return 0, true
	}
	if left < every {
		return left, false
	}
	return every, false
}

// platform is the OS side; the real one is in win32_windows.go.
type platform interface {
	KeepAwake() error
	Release() error
	Nudge() error
	IdleTime() (time.Duration, error)
}

// idleWarnAfter: a working nudge drops system idle time to ~0.
const idleWarnAfter = 5 * time.Second

type loop struct {
	p        platform
	every    time.Duration
	deadline time.Time
	now      func() time.Time
	wait     func(time.Duration) bool // false = interrupted
	out      io.Writer
}

func (l loop) run() int {
	if err := l.p.KeepAwake(); err != nil {
		fmt.Fprintf(l.out, "keepawake: SetThreadExecutionState failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(l.out, "keepawake: SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED|ES_DISPLAY_REQUIRED) ok")
	defer func() {
		if err := l.p.Release(); err != nil {
			fmt.Fprintf(l.out, "keepawake: could not restore normal sleep: %v\n", err)
		} else {
			fmt.Fprintln(l.out, "keepawake: off (normal sleep restored)")
		}
	}()

	until := ""
	if !l.deadline.IsZero() {
		until = ", until " + l.deadline.Format("15:04")
	}
	fmt.Fprintf(l.out, "keepawake: on (nudge every %s%s). Ctrl+C to stop.\n", l.every, until)

	warned := false
	for {
		if err := l.p.Nudge(); err != nil && !warned {
			fmt.Fprintf(l.out, "keepawake: WARNING - mouse nudge failed (%v); the PC stays awake but Teams/Slack may go Away.\n", err)
			warned = true
		}
		if idle, err := l.p.IdleTime(); err == nil && idle > idleWarnAfter && !warned {
			fmt.Fprintf(l.out, "keepawake: WARNING - the nudge is not resetting idle time (idle %s); the PC stays awake\n", idle.Round(time.Second))
			fmt.Fprintln(l.out, "  but Teams/Slack may go Away. Something (policy, remote session, locked screen) is dropping the input.")
			warned = true
		}
		nap, done := nextNap(l.every, l.deadline, l.now())
		if done {
			fmt.Fprintln(l.out, "keepawake: time is up")
			return 0
		}
		if !l.wait(nap) {
			fmt.Fprintln(l.out)
			return 0
		}
	}
}

const usage = `keepawake - keep Windows awake and Teams/Slack presence "Available".
  keepawake.exe            run until Ctrl+C
  keepawake.exe 2h         stop after 2 hours (accepts 30m, 2h, 90s)
  set KEEPAWAKE_EVERY=60   nudge interval in seconds (default 30; Teams goes Away at 5 min idle)
`
