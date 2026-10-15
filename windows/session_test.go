package main

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func TestLeftText(t *testing.T) {
	cases := map[time.Duration]string{
		0: "0:00", -time.Second: "0:00", time.Second: "0:01", time.Minute: "0:01",
		time.Minute + time.Second: "0:02", 83 * time.Minute: "1:23", 4 * time.Hour: "4:00",
	}
	for d, want := range cases {
		if got := leftText(d); got != want {
			t.Errorf("leftText(%v) = %q; want %q", d, got, want)
		}
	}
}

func TestSessionTooltips(t *testing.T) {
	if got := (session{}).tooltip(t0); got != "keepawake — off" {
		t.Errorf("off: %q", got)
	}
	if got := startSession(chUntil, 0, t0).tooltip(t0.Add(time.Hour)); got != "keepawake — on until stopped" {
		t.Errorf("until: %q", got)
	}
	s := startSession(ch2h, 2*time.Hour, t0)
	if got := s.tooltip(t0.Add(37 * time.Minute)); got != "keepawake — on, 1:23 left" {
		t.Errorf("2h after 37m: %q", got)
	}
}

func TestSessionExpiry(t *testing.T) {
	for _, m := range menuChoices {
		s := startSession(m.c, m.d, t0)
		if !s.on() || s.choice != m.c {
			t.Fatalf("%s: not on", m.label)
		}
		if choiceDuration(m.c) != m.d {
			t.Errorf("%s: choiceDuration = %v", m.label, choiceDuration(m.c))
		}
		if m.d == 0 {
			if s.expired(t0.Add(365 * 24 * time.Hour)) {
				t.Errorf("%s expired", m.label)
			}
			continue
		}
		if s.expired(t0.Add(m.d - time.Second)) {
			t.Errorf("%s expired a second early", m.label)
		}
		if !s.expired(t0.Add(m.d)) {
			t.Errorf("%s not expired at its end", m.label)
		}
	}
	if (session{}).expired(t0) {
		t.Error("off session expired")
	}
}

func TestToggle(t *testing.T) {
	s := (session{}).toggled(t0)
	if s.choice != chUntil || !s.deadline.IsZero() {
		t.Fatalf("off -> %+v; want until stopped", s)
	}
	if s.toggled(t0).on() {
		t.Error("on -> still on")
	}
	if startSession(ch1h, time.Hour, t0).toggled(t0).on() {
		t.Error("timed on -> still on")
	}
}
