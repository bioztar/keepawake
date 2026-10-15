package main

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	user32                      = syscall.NewLazyDLL("user32.dll")
	shell32                     = syscall.NewLazyDLL("shell32.dll")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	procGetTickCount            = kernel32.NewProc("GetTickCount")
	procSendInput               = user32.NewProc("SendInput")
	procGetLastInputInfo        = user32.NewProc("GetLastInputInfo")
	procShellNotifyIcon         = shell32.NewProc("Shell_NotifyIconW")
)

const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002

	inputMouse      = 0
	inputKeyboard   = 1
	mouseeventfMove = 0x0001
	keyeventfKeyUp  = 0x0002
	vkF15           = 0x7E
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

type keybdInput struct {
	vk, scan    uint16
	flags, time uint32
	extra       uintptr
}

// INPUT with the KEYBDINPUT arm, padded to sizeof(INPUT).
type keyInput struct {
	typ uint32
	ki  keybdInput
	_   [unsafe.Sizeof(mouseInput{}) - unsafe.Sizeof(keybdInput{})]byte
}

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

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

func sendInput(p unsafe.Pointer) error {
	r, _, err := procSendInput.Call(1, uintptr(p), unsafe.Sizeof(input{}))
	if r != 1 {
		return err
	}
	return nil
}

// sendMove0 is a relative mouse move of 0,0: real input to Windows, but the pointer does not move.
func sendMove0() error {
	in := input{typ: inputMouse, mi: mouseInput{dwFlags: mouseeventfMove}}
	return sendInput(unsafe.Pointer(&in))
}

func sendF15() error {
	in := keyInput{typ: inputKeyboard, ki: keybdInput{vk: vkF15}}
	if err := sendInput(unsafe.Pointer(&in)); err != nil {
		return err
	}
	in.ki.flags = keyeventfKeyUp
	return sendInput(unsafe.Pointer(&in))
}

// settle gives the raw input thread time to process injected input before GetLastInputInfo.
const settle = 50 * time.Millisecond

// Nudge resets the input-idle timer that Teams reads, without moving the pointer: a zero-distance
// mouse move, and F15 (no key has that) only if Windows ignored the move.
func (p win32) Nudge() (string, error) {
	if err := sendMove0(); err != nil {
		return "mouse", err
	}
	time.Sleep(settle)
	if idle, err := p.IdleTime(); err != nil || idle < nudgeOKBelow {
		return "mouse", nil
	}
	if err := sendF15(); err != nil {
		return "mouse+F15", err
	}
	time.Sleep(settle)
	return "mouse+F15", nil
}

func (win32) IdleTime() (time.Duration, error) {
	li := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&li))); r == 0 {
		return 0, errors.New("GetLastInputInfo failed")
	}
	tick, _, _ := procGetTickCount.Call()
	return time.Duration(uint32(tick)-li.dwTime) * time.Millisecond, nil
}

// NOTIFYICONDATAW (Vista+ layout).
type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.HWND
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout                    uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

const (
	nimModify   = 1
	nifInfo     = 0x10
	niifInfo    = 1
	systrayID   = 100 // fyne.io/systray's NOTIFYICONDATA uID
	systrayWndC = "SystrayClass"
)

var trayWnd windows.HWND

// findTrayWindow finds fyne.io/systray's hidden window in this process (it is not exported).
func findTrayWindow() windows.HWND {
	if trayWnd != 0 {
		return trayWnd
	}
	pid := uint32(os.Getpid())
	cb := syscall.NewCallback(func(h windows.HWND, _ uintptr) uintptr {
		var wpid uint32
		windows.GetWindowThreadProcessId(h, &wpid)
		if wpid != pid {
			return 1
		}
		var cls [64]uint16
		if n, _ := windows.GetClassName(h, &cls[0], int32(len(cls))); n > 0 && windows.UTF16ToString(cls[:n]) == systrayWndC {
			trayWnd = h
			return 0
		}
		return 1
	})
	windows.EnumWindows(cb, nil)
	return trayWnd
}

// balloon shows a tray notification (a toast on Windows 10/11) from the keepawake icon.
func balloon(title, msg string) error {
	h := findTrayWindow()
	if h == 0 {
		return errors.New("tray window not found")
	}
	nid := notifyIconData{Wnd: h, ID: systrayID, Flags: nifInfo, InfoFlags: niifInfo}
	nid.Size = uint32(unsafe.Sizeof(nid))
	copy(nid.InfoTitle[:len(nid.InfoTitle)-1], windows.StringToUTF16(title))
	copy(nid.Info[:len(nid.Info)-1], windows.StringToUTF16(msg))
	nid.InfoTitle[len(nid.InfoTitle)-1], nid.Info[len(nid.Info)-1] = 0, 0
	if r, _, err := procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return err
	}
	return nil
}

const (
	instanceMutex = `Local\keepawake-tray`
	showEvent     = `Local\keepawake-tray-show`
)

// claimInstance returns (true, event) for the first instance; the event fires when a second one starts.
// A second instance signals the event and gets (false, 0).
func claimInstance() (bool, windows.Handle, error) {
	name, _ := windows.UTF16PtrFromString(instanceMutex)
	// The handle stays open for the life of the process; that is what marks "running".
	_, err := windows.CreateMutex(nil, false, name)
	ev, _ := windows.UTF16PtrFromString(showEvent)
	if err == windows.ERROR_ALREADY_EXISTS {
		h, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, ev)
		if err == nil {
			windows.SetEvent(h)
			windows.CloseHandle(h)
		}
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	h, err := windows.CreateEvent(nil, 0, 0, ev)
	return true, h, err
}

func messageBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONINFORMATION)
}

func openInNotepad(path string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	exe, _ := windows.UTF16PtrFromString("notepad.exe")
	arg, _ := windows.UTF16PtrFromString(`"` + path + `"`)
	return windows.ShellExecute(0, verb, exe, arg, nil, windows.SW_SHOWNORMAL)
}
