package main

import (
	"fmt"
	"runtime"
	"time"
)

// platform is the OS side; the real one is in win32_windows.go.
type platform interface {
	KeepAwake() error
	Release() error
	// Nudge sends invisible input and returns once Windows has processed it; how names the input used.
	Nudge() (how string, err error)
	IdleTime() (time.Duration, error)
}

// ui is the tray side; the real one is in tray_windows.go.
type ui interface {
	Show(s session, tooltip string)
	Notify(title, msg string)
}

const (
	logAwakeOK   = "SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED|ES_DISPLAY_REQUIRED) ok"
	logReleaseOK = "SetThreadExecutionState(ES_CONTINUOUS) ok - normal sleep restored"
	// A working nudge drops the system idle time to about zero.
	nudgeOKBelow = time.Second
)

type engine struct {
	p     platform
	ui    ui
	logf  func(format string, args ...any)
	every time.Duration // nudge interval

	s           session
	tip         string
	nextNudge   time.Time
	nudgeFailed bool // last nudge did not reset idle time
	warned      bool // the "Teams may go Away" notification is shown once per run
}

// skipBelow: if the user was active this recently, no nudge is needed.
func (e *engine) skipBelow() time.Duration {
	if s := e.every / 2; s < 15*time.Second {
		return s
	}
	return 15 * time.Second
}

// set switches to choice c for d (0 = until stopped); chOff turns off.
func (e *engine) set(c choice, d time.Duration, now time.Time, reason string) {
	if c == chOff {
		e.off(now, reason)
		return
	}
	if !e.s.on() {
		if err := e.p.KeepAwake(); err != nil {
			e.logf("ERROR SetThreadExecutionState failed: %v", err)
			e.ui.Notify("keepawake could not turn on", "Windows refused the keep-awake request: "+err.Error())
			return
		}
		e.logf(logAwakeOK)
		e.nextNudge = now
	}
	e.s = startSession(c, d, now)
	if e.s.deadline.IsZero() {
		e.logf("on until stopped (%s), nudge every %s", reason, e.every)
	} else {
		e.logf("on for %s until %s (%s), nudge every %s", d, e.s.deadline.Format("15:04:05"), reason, e.every)
	}
	e.tick(now)
}

func (e *engine) toggle(now time.Time) {
	if e.s.on() {
		e.off(now, "clicked")
	} else {
		e.set(chUntil, 0, now, "clicked")
	}
}

func (e *engine) off(now time.Time, reason string) {
	if !e.s.on() {
		return
	}
	e.s = session{}
	if err := e.p.Release(); err != nil {
		e.logf("ERROR could not restore normal sleep: %v", err)
	} else {
		e.logf(logReleaseOK)
	}
	e.logf("off (%s)", reason)
	e.show(now)
	e.ui.Notify("keepawake off", "PC can sleep again.")
}

// tick runs about once a second: ends timed sessions, nudges when due, refreshes the tooltip.
func (e *engine) tick(now time.Time) {
	if e.s.expired(now) {
		e.off(now, "time is up")
		return
	}
	if e.s.on() && !now.Before(e.nextNudge) {
		e.nudge()
		e.nextNudge = now.Add(e.every)
	}
	e.show(now)
}

func (e *engine) show(now time.Time) {
	if tip := e.s.tooltip(now); tip != e.tip {
		e.tip = tip
		e.ui.Show(e.s, tip)
	}
}

func (e *engine) nudge() {
	before, err := e.p.IdleTime()
	if err != nil {
		e.logf("WARNING cannot read idle time: %v", err)
	} else if before < e.skipBelow() {
		return // user is active; Windows already sees input
	}
	how, err := e.p.Nudge()
	if err != nil {
		e.nudgeBad(fmt.Sprintf("input nudge failed: %v", err))
		return
	}
	after, err := e.p.IdleTime()
	if err != nil {
		return
	}
	if after < before && after < nudgeOKBelow {
		e.logf("nudge ok (%s): idle %s -> %s", how, fmtIdle(before), fmtIdle(after))
		if e.nudgeFailed {
			e.logf("nudges reset idle time again")
		}
		e.nudgeFailed = false
		return
	}
	e.nudgeBad(fmt.Sprintf("nudge (%s) did not reset idle time: idle %s -> %s", how, fmtIdle(before), fmtIdle(after)))
}

func (e *engine) nudgeBad(what string) {
	e.nudgeFailed = true
	e.logf("WARNING %s. The PC stays awake, but Teams may go Away. Usual causes: locked screen, Remote Desktop, or an IT policy dropping simulated input.", what)
	if !e.warned {
		e.warned = true
		e.ui.Notify("Teams may go Away",
			"The PC stays awake, but Windows is ignoring keepawake's input (locked screen, Remote Desktop or IT policy). Details: Open log.")
	}
}

func fmtIdle(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// command is a UI action, run on the engine goroutine.
type command func(e *engine, now time.Time)

// run owns the engine on one locked OS thread: SetThreadExecutionState applies per thread.
// It returns after quit is closed and the PC is released.
func (e *engine) run(cmds <-chan command, quit <-chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case c := <-cmds:
			c(e, time.Now())
		case now := <-t.C:
			e.tick(now)
		case <-quit:
			e.off(time.Now(), "quit")
			return
		}
	}
}
