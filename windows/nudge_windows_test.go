package main

import (
	"testing"
	"time"
	"unsafe"
)

var (
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSetCursorPos         = user32.NewProc("SetCursorPos")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

type point struct{ x, y int32 }

func cursor() point {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func waitIdle(t *testing.T, d time.Duration) time.Duration {
	time.Sleep(d)
	idle, err := win32{}.IdleTime()
	if err != nil {
		t.Fatal(err)
	}
	return idle
}

// The shipped nudge must reset GetLastInputInfo (what Teams reads) without moving the pointer.
func TestNudgeResetsIdleWithoutMovingPointer(t *testing.T) {
	for i := 0; i < 3; i++ {
		before := waitIdle(t, 1500*time.Millisecond)
		start := cursor()
		how, err := win32{}.Nudge()
		after, _ := win32{}.IdleTime()
		end := cursor()
		t.Logf("nudge (%s): idle %dms -> %dms, cursor %v -> %v", how, before.Milliseconds(), after.Milliseconds(), start, end)
		if err != nil {
			t.Fatalf("Nudge: %v", err)
		}
		if !(after < before && after < nudgeOKBelow) {
			t.Fatalf("idle did not drop: %v -> %v", before, after)
		}
		if how != "mouse" {
			t.Errorf("needed the F15 fallback: %s", how)
		}
		if end != start {
			t.Errorf("pointer moved: %v -> %v", start, end)
		}
	}
}

func TestF15ResetsIdle(t *testing.T) {
	before := waitIdle(t, 1500*time.Millisecond)
	if err := sendF15(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(settle)
	after, _ := win32{}.IdleTime()
	t.Logf("F15: idle %dms -> %dms", before.Milliseconds(), after.Milliseconds())
	if !(after < before && after < nudgeOKBelow) {
		t.Fatalf("idle did not drop: %v -> %v", before, after)
	}
}

func sendMoveBy(dx int32) error {
	in := input{typ: inputMouse, mi: mouseInput{dx: dx, dwFlags: mouseeventfMove}}
	return sendInput(unsafe.Pointer(&in))
}

// Evidence only (no assertions): which inputs reset the idle timer, with and without
// "Enhance pointer precision" (mouse acceleration), including v0.1.0's 1 px right-and-back.
func TestProbeInputMethods(t *testing.T) {
	var orig [3]int32                                                                 // threshold1, threshold2, acceleration
	procSystemParametersInfo.Call(0x0003, 0, uintptr(unsafe.Pointer(&orig)), 0)       // SPI_GETMOUSE
	defer procSystemParametersInfo.Call(0x0004, 0, uintptr(unsafe.Pointer(&orig)), 0) // SPI_SETMOUSE, session only
	methods := []struct {
		name string
		do   func() error
	}{
		{"SetCursorPos same spot", func() error { p := cursor(); procSetCursorPos.Call(uintptr(p.x), uintptr(p.y)); return nil }},
		{"SendInput move 0,0 (v0.2)", sendMove0},
		{"SendInput +1 px then -1 (v0.1.0)", func() error {
			if err := sendMoveBy(1); err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)
			return sendMoveBy(-1)
		}},
		{"SendInput F15 down/up", sendF15},
	}
	for _, accel := range []int32{1, 0} {
		m := [3]int32{6, 10, accel}
		procSystemParametersInfo.Call(0x0004, 0, uintptr(unsafe.Pointer(&m)), 0)
		for _, mt := range methods {
			before := waitIdle(t, 1500*time.Millisecond)
			start := cursor()
			err := mt.do()
			time.Sleep(100 * time.Millisecond)
			after, _ := win32{}.IdleTime()
			t.Logf("PROBE accel=%d %-34s err=%v idle %5dms -> %4dms reset=%-5v cursor %v -> %v",
				accel, mt.name, err, before.Milliseconds(), after.Milliseconds(), after < before && after < time.Second, start, cursor())
		}
	}
}
