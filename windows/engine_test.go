package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakePlatform struct {
	awake, nudges int
	released      int
	awakeErr      error
	idle          time.Duration // idle reported before a nudge
	idleAfter     time.Duration // idle reported after a nudge
	nudgeErr      error
	justNudged    bool
}

func (f *fakePlatform) KeepAwake() error {
	if f.awakeErr != nil {
		return f.awakeErr
	}
	f.awake++
	return nil
}
func (f *fakePlatform) Release() error { f.released++; return nil }
func (f *fakePlatform) Nudge() (string, error) {
	if f.nudgeErr != nil {
		return "", f.nudgeErr
	}
	f.nudges++
	f.justNudged = true
	return "mouse", nil
}
func (f *fakePlatform) IdleTime() (time.Duration, error) {
	if f.justNudged {
		f.justNudged = false
		return f.idleAfter, nil
	}
	return f.idle, nil
}

type fakeUI struct {
	tips    []string
	notes   []string
	lastOn  bool
	lastSel choice
}

func (u *fakeUI) Show(s session, tip string) {
	u.tips = append(u.tips, tip)
	u.lastOn = s.on()
	u.lastSel = s.choice
}
func (u *fakeUI) Notify(title, msg string) { u.notes = append(u.notes, title) }

func newTestEngine() (*engine, *fakePlatform, *fakeUI, *[]string) {
	p := &fakePlatform{idle: 30 * time.Second, idleAfter: 50 * time.Millisecond}
	u := &fakeUI{}
	var log []string
	e := &engine{p: p, ui: u, every: 30 * time.Second,
		logf: func(f string, a ...any) { log = append(log, fmt.Sprintf(f, a...)) }}
	return e, p, u, &log
}

func has(log []string, sub string) bool {
	for _, l := range log {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func TestEngineTimedSession(t *testing.T) {
	e, p, u, log := newTestEngine()
	e.set(ch30m, 30*time.Minute, t0, "menu")
	if p.awake != 1 || p.nudges != 1 || !u.lastOn || u.lastSel != ch30m {
		t.Fatalf("after on: awake=%d nudges=%d ui on=%v", p.awake, p.nudges, u.lastOn)
	}
	if !has(*log, logAwakeOK) || !has(*log, "nudge ok (mouse): idle 30.0s -> 0.1s") {
		t.Fatalf("log: %q", *log)
	}
	// Ticks every second for 30 minutes: one nudge per 30 s, then off exactly at the end.
	for s := 1; s < 30*60; s++ {
		e.tick(t0.Add(time.Duration(s) * time.Second))
	}
	if p.nudges != 60 || !e.s.on() {
		t.Fatalf("before end: nudges=%d on=%v", p.nudges, e.s.on())
	}
	e.tick(t0.Add(30 * time.Minute))
	if e.s.on() || p.released != 1 || u.lastOn {
		t.Fatal("not off at the end")
	}
	if !has(*log, logReleaseOK) || !has(*log, "off (time is up)") || u.notes[len(u.notes)-1] != "keepawake off" {
		t.Errorf("log %q notes %q", *log, u.notes)
	}
	if u.tips[0] != "keepawake — on, 0:30 left" || u.tips[len(u.tips)-1] != "keepawake — off" {
		t.Errorf("tips %q", u.tips)
	}
}

func TestEngineSwitchKeepsSingleRequest(t *testing.T) {
	e, p, _, _ := newTestEngine()
	e.set(ch1h, time.Hour, t0, "menu")
	e.set(chUntil, 0, t0.Add(time.Minute), "menu")
	if p.awake != 1 || p.released != 0 || e.s.choice != chUntil {
		t.Fatalf("awake=%d released=%d choice=%v", p.awake, p.released, e.s.choice)
	}
	e.tick(t0.Add(2 * time.Hour))
	if !e.s.on() {
		t.Fatal("until-stopped session ended by itself")
	}
	e.set(chOff, 0, t0.Add(2*time.Hour), "stop")
	if e.s.on() || p.released != 1 {
		t.Fatal("stop did not release")
	}
	e.set(chOff, 0, t0.Add(3*time.Hour), "stop")
	if p.released != 1 {
		t.Error("released twice")
	}
}

func TestEngineToggle(t *testing.T) {
	e, p, _, _ := newTestEngine()
	e.toggle(t0)
	if e.s.choice != chUntil || p.awake != 1 {
		t.Fatal("toggle on")
	}
	e.toggle(t0)
	if e.s.on() || p.released != 1 {
		t.Fatal("toggle off")
	}
}

func TestEngineSkipsNudgeWhileUserActive(t *testing.T) {
	e, p, _, _ := newTestEngine()
	p.idle = 2 * time.Second
	e.set(chUntil, 0, t0, "menu")
	if p.nudges != 0 {
		t.Fatal("nudged while the user is active")
	}
	p.idle = 31 * time.Second
	e.tick(t0.Add(30 * time.Second))
	if p.nudges != 1 {
		t.Fatal("no nudge after 30 s idle")
	}
}

func TestEngineWarnsOnceWhenIdleDoesNotDrop(t *testing.T) {
	e, p, u, log := newTestEngine()
	p.idleAfter = 31 * time.Second // input dropped (e.g. locked screen)
	e.set(chUntil, 0, t0, "menu")
	e.tick(t0.Add(30 * time.Second))
	warns := 0
	for _, n := range u.notes {
		if n == "Teams may go Away" {
			warns++
		}
	}
	if warns != 1 || !has(*log, "WARNING nudge (mouse) did not reset idle time: idle 30.0s -> 31.0s") {
		t.Fatalf("notes %q log %q", u.notes, *log)
	}
	p.idleAfter = 0
	e.tick(t0.Add(60 * time.Second))
	if !has(*log, "nudges reset idle time again") {
		t.Errorf("no recovery line: %q", *log)
	}
}

func TestEngineNudgeError(t *testing.T) {
	e, p, u, log := newTestEngine()
	p.nudgeErr = errors.New("Access is denied.")
	e.set(chUntil, 0, t0, "menu")
	if !e.s.on() || !has(*log, "input nudge failed: Access is denied.") || len(u.notes) != 1 {
		t.Fatalf("on=%v log %q notes %q", e.s.on(), *log, u.notes)
	}
}

func TestEngineKeepAwakeError(t *testing.T) {
	e, p, u, log := newTestEngine()
	p.awakeErr = errors.New("nope")
	e.set(ch1h, time.Hour, t0, "menu")
	if e.s.on() || len(u.notes) != 1 || !has(*log, "ERROR SetThreadExecutionState failed") {
		t.Fatalf("on=%v notes %q log %q", e.s.on(), u.notes, *log)
	}
}

func TestEngineRunQuitReleases(t *testing.T) {
	e, p, _, _ := newTestEngine()
	cmds, quit, done := make(chan command), make(chan struct{}), make(chan struct{})
	go func() { e.run(cmds, quit); close(done) }()
	cmds <- func(e *engine, now time.Time) { e.set(chUntil, 0, now, "menu") }
	close(quit)
	<-done
	if p.awake != 1 || p.released != 1 {
		t.Fatalf("awake=%d released=%d", p.awake, p.released)
	}
}
