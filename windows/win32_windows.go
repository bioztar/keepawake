package main

import (
	"errors"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	user32                      = syscall.NewLazyDLL("user32.dll")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	procGetTickCount            = kernel32.NewProc("GetTickCount")
	procSendInput               = user32.NewProc("SendInput")
	procGetLastInputInfo        = user32.NewProc("GetLastInputInfo")
	procGetCursorPos            = user32.NewProc("GetCursorPos")
	procSetCursorPos            = user32.NewProc("SetCursorPos")
)

const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002

	inputMouse      = 0
	mouseeventfMove = 0x0001
)

type mouseInput struct {
	dx, dy      int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// INPUT with the MOUSEINPUT arm of the union (the largest arm); 40 bytes on 64-bit.
type input struct {
	typ uint32
	mi  mouseInput
}

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type point struct{ x, y int32 }

type win32 struct{}

func setState(flags uint32) error {
	r, _, err := procSetThreadExecutionState.Call(uintptr(flags))
	if r == 0 {
		return err
	}
	return nil
}

func (win32) KeepAwake() error {
	return setState(esContinuous | esSystemRequired | esDisplayRequired)
}

func (win32) Release() error { return setState(esContinuous) }

func sendMove(dx int32) error {
	in := input{typ: inputMouse, mi: mouseInput{dx: dx, dwFlags: mouseeventfMove}}
	r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if r != 1 {
		return err
	}
	return nil
}

// Nudge moves the pointer 1 px right and back with real input events. That resets the
// input-idle timer new Teams reads; power requests alone do not.
func (win32) Nudge() error {
	var start point
	havePos, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&start)))
	if err := sendMove(1); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	if err := sendMove(-1); err != nil {
		return err
	}
	// Pointer acceleration can leave it a pixel off; put it back exactly.
	if havePos != 0 {
		var now point
		if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&now))); r != 0 && now != start {
			procSetCursorPos.Call(uintptr(start.x), uintptr(start.y))
		}
	}
	return nil
}

func (win32) IdleTime() (time.Duration, error) {
	li := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&li))); r == 0 {
		return 0, errors.New("GetLastInputInfo failed")
	}
	tick, _, _ := procGetTickCount.Call()
	return time.Duration(uint32(tick)-li.dwTime) * time.Millisecond, nil
}
