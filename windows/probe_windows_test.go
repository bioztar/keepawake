package main

import (
	"testing"
	"time"
	"unsafe"
)

// Evidence for the nudge choice: which injected inputs reset GetLastInputInfo, and which move the cursor.

var procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")

type keybdInput struct {
	vk, scan    uint16
	flags, time uint32
	extra       uintptr
}

type keyInputT struct {
	typ uint32
	ki  keybdInput
	pad [8]byte
}

func sendKey(vk uint16, up bool) error {
	in := keyInputT{typ: 1, ki: keybdInput{vk: vk}}
	if up {
		in.ki.flags = 2
	}
	r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(input{}))
	if r != 1 {
		return err
	}
	return nil
}

func cursor() point {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

// mouse params [threshold1, threshold2, acceleration]; acceleration 1 = "Enhance pointer precision".
func getMouse() [3]int32 {
	var m [3]int32
	procSystemParametersInfo.Call(0x0003, 0, uintptr(unsafe.Pointer(&m)), 0)
	return m
}

func setMouse(m [3]int32) {
	procSystemParametersInfo.Call(0x0004, 0, uintptr(unsafe.Pointer(&m)), 0) // no SPIF_UPDATEINIFILE: session only
}

func TestProbeInputMethods(t *testing.T) {
	orig := getMouse()
	defer setMouse(orig)
	t.Logf("mouse params at start: %v (Enhance pointer precision = %v)", orig, orig[2] != 0)
	methods := []struct {
		name string
		do   func() error
	}{
		{"SetCursorPos same spot", func() error { p := cursor(); procSetCursorPos.Call(uintptr(p.x), uintptr(p.y)); return nil }},
		{"SendInput rel move 0,0", func() error { return sendMove(0) }},
		{"SendInput rel +1 then -1 (v0.1.0)", func() error {
			if err := sendMove(1); err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)
			return sendMove(-1)
		}},
		{"SendInput F15 down/up", func() error {
			if err := sendKey(0x7E, false); err != nil {
				return err
			}
			return sendKey(0x7E, true)
		}},
	}
	for _, epp := range []int32{1, 0} {
		m := orig
		m[2] = epp
		if epp == 1 && m[0] == 0 && m[1] == 0 {
			m[0], m[1] = 6, 10
		}
		setMouse(m)
		for _, mt := range methods {
			time.Sleep(1500 * time.Millisecond)
			before, _ := win32{}.IdleTime()
			start := cursor()
			err := mt.do()
			time.Sleep(100 * time.Millisecond)
			after, _ := win32{}.IdleTime()
			end := cursor()
			t.Logf("PROBE epp=%d %-36s err=%v idle %5dms -> %5dms dropped=%v cursor %v -> %v",
				epp, mt.name, err, before.Milliseconds(), after.Milliseconds(), after < before && after < time.Second, start, end)
		}
	}
}
