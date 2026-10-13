package main

import (
	"bytes"
	"errors"
	"strings"
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

func TestNextNap(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	every := 30 * time.Second
	cases := []struct {
		deadline time.Time
		nap      time.Duration
		done     bool
	}{
		{time.Time{}, every, false},
		{now.Add(time.Hour), every, false},
		{now.Add(every), every, false},
		{now.Add(10 * time.Second), 10 * time.Second, false},
		{now, 0, true},
		{now.Add(-time.Second), 0, true},
	}
	for _, c := range cases {
		nap, done := nextNap(every, c.deadline, now)
		if nap != c.nap || done != c.done {
			t.Errorf("nextNap(deadline %v) = %v, %v; want %v, %v", c.deadline.Sub(now), nap, done, c.nap, c.done)
		}
	}
}

type fake struct {
	awake, released bool
	nudges          int
	nudgeErr        error
	idle            time.Duration
	awakeErr        error
}

func (f *fake) KeepAwake() error                 { f.awake = true; return f.awakeErr }
func (f *fake) Release() error                   { f.released = true; return nil }
func (f *fake) Nudge() error                     { f.nudges++; return f.nudgeErr }
func (f *fake) IdleTime() (time.Duration, error) { return f.idle, nil }

type clock struct {
	t     time.Time
	naps  []time.Duration
	limit int // interrupt after this many waits (0 = never)
}

func (c *clock) now() time.Time { return c.t }
func (c *clock) wait(d time.Duration) bool {
	if c.limit > 0 && len(c.naps) >= c.limit {
		return false
	}
	c.naps = append(c.naps, d)
	c.t = c.t.Add(d)
	return true
}

func newLoop(f *fake, c *clock, every time.Duration, runFor time.Duration, out *bytes.Buffer) loop {
	var deadline time.Time
	if runFor > 0 {
		deadline = c.t.Add(runFor)
	}
	return loop{p: f, every: every, deadline: deadline, now: c.now, wait: c.wait, out: out}
}

func TestLoopDeadline(t *testing.T) {
	f, c, out := &fake{}, &clock{t: time.Unix(0, 0)}, &bytes.Buffer{}
	if code := newLoop(f, c, 30*time.Second, 75*time.Second, out).run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := []time.Duration{30 * time.Second, 30 * time.Second, 15 * time.Second}
	if len(c.naps) != len(want) {
		t.Fatalf("naps %v; want %v", c.naps, want)
	}
	for i := range want {
		if c.naps[i] != want[i] {
			t.Fatalf("naps %v; want %v", c.naps, want)
		}
	}
	if f.nudges != 4 || !f.awake || !f.released {
		t.Errorf("nudges=%d awake=%v released=%v", f.nudges, f.awake, f.released)
	}
	s := out.String()
	for _, w := range []string{"SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED|ES_DISPLAY_REQUIRED) ok", "time is up", "normal sleep restored"} {
		if !strings.Contains(s, w) {
			t.Errorf("output missing %q:\n%s", w, s)
		}
	}
	if strings.Contains(s, "WARNING") {
		t.Errorf("unexpected warning:\n%s", s)
	}
}

func TestLoopInterruptReleases(t *testing.T) {
	f, c, out := &fake{}, &clock{t: time.Unix(0, 0), limit: 2}, &bytes.Buffer{}
	if code := newLoop(f, c, 30*time.Second, 0, out).run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if f.nudges != 3 || !f.released {
		t.Errorf("nudges=%d released=%v", f.nudges, f.released)
	}
}

func TestLoopWarnsOnceWhenIdleNotReset(t *testing.T) {
	f, c, out := &fake{idle: 40 * time.Second}, &clock{t: time.Unix(0, 0)}, &bytes.Buffer{}
	newLoop(f, c, 30*time.Second, 5*time.Minute, out).run()
	if n := strings.Count(out.String(), "WARNING"); n != 1 {
		t.Errorf("%d warnings; want 1:\n%s", n, out.String())
	}
}

func TestLoopWarnsOnceWhenNudgeFails(t *testing.T) {
	f, c, out := &fake{nudgeErr: errors.New("access denied")}, &clock{t: time.Unix(0, 0)}, &bytes.Buffer{}
	if code := newLoop(f, c, 30*time.Second, 2*time.Minute, out).run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if n := strings.Count(out.String(), "WARNING"); n != 1 || !strings.Contains(out.String(), "access denied") {
		t.Errorf("want one nudge warning:\n%s", out.String())
	}
}

func TestLoopFailsWithoutExecutionState(t *testing.T) {
	f, c, out := &fake{awakeErr: errors.New("nope")}, &clock{t: time.Unix(0, 0)}, &bytes.Buffer{}
	if code := newLoop(f, c, 30*time.Second, time.Minute, out).run(); code != 1 {
		t.Fatalf("exit %d; want 1", code)
	}
	if f.nudges != 0 || f.released {
		t.Errorf("nudges=%d released=%v", f.nudges, f.released)
	}
}
