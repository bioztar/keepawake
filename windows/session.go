package main

import (
	"fmt"
	"time"
)

// choice is what the user picked in the menu; chCustom is a --on duration that has no menu item.
type choice int

const (
	chOff choice = iota
	ch30m
	ch1h
	ch2h
	ch4h
	chUntil
	chCustom
)

var menuChoices = []struct {
	c     choice
	label string
	d     time.Duration // 0 = until stopped
}{
	{ch30m, "Keep awake 30 min", 30 * time.Minute},
	{ch1h, "Keep awake 1 hour", time.Hour},
	{ch2h, "Keep awake 2 hours", 2 * time.Hour},
	{ch4h, "Keep awake 4 hours", 4 * time.Hour},
	{chUntil, "Keep awake until I stop it", 0},
}

func choiceDuration(c choice) time.Duration {
	for _, m := range menuChoices {
		if m.c == c {
			return m.d
		}
	}
	return 0
}

// session is the on/off state. A zero deadline while on means "until stopped".
type session struct {
	choice   choice
	deadline time.Time
}

func (s session) on() bool { return s.choice != chOff }

// startSession turns on for d (0 = until stopped).
func startSession(c choice, d time.Duration, now time.Time) session {
	s := session{choice: c}
	if d > 0 {
		s.deadline = now.Add(d)
	}
	return s
}

// toggled is the left-click action: off -> on until stopped, on -> off.
func (s session) toggled(now time.Time) session {
	if s.on() {
		return session{}
	}
	return startSession(chUntil, 0, now)
}

func (s session) expired(now time.Time) bool {
	return s.on() && !s.deadline.IsZero() && !now.Before(s.deadline)
}

func (s session) tooltip(now time.Time) string {
	switch {
	case !s.on():
		return "keepawake — off"
	case s.deadline.IsZero():
		return "keepawake — on until stopped"
	}
	return "keepawake — on, " + leftText(s.deadline.Sub(now)) + " left"
}

// leftText renders time left as h:mm, rounded up to the minute so it never shows 0:00 while on.
func leftText(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%d:%02d", m/60, m%60)
}
