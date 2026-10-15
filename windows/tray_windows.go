package main

import (
	_ "embed"
	"log"
	"time"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

var (
	//go:embed icons/on.ico
	iconOn []byte
	//go:embed icons/off.ico
	iconOff []byte
)

// tray is the ui for the engine plus the menu wiring.
type tray struct {
	log      *log.Logger
	logPath  string
	eng      *engine
	auto     autostart
	show     windows.Handle // fires when a second keepawake is launched
	startOn  command        // from --on, or nil
	welcome  bool           // manual launch: say where the icon is
	quitOff  bool           // --quit-when-off
	exitCode int

	cmds  chan command
	quit  chan struct{}
	done  chan struct{}
	items map[choice]*systray.MenuItem
	stop  *systray.MenuItem
	wasOn bool
}

func (t *tray) Show(s session, tip string) {
	if s.on() {
		systray.SetIcon(iconOn)
	} else {
		systray.SetIcon(iconOff)
	}
	systray.SetTooltip(tip)
	for c, it := range t.items {
		if s.on() && s.choice == c {
			it.Check()
		} else {
			it.Uncheck()
		}
	}
	if s.on() {
		t.stop.Enable()
	} else {
		t.stop.Disable()
	}
	if t.quitOff && t.wasOn && !s.on() {
		go systray.Quit()
	}
	t.wasOn = s.on()
}

func (t *tray) Notify(title, msg string) {
	t.log.Printf("notification: %s - %s", title, msg)
	if err := balloon(title, msg); err != nil {
		t.log.Printf("WARNING notification failed: %v", err)
	}
}

func (t *tray) send(c command) { t.cmds <- c }

func (t *tray) onReady() {
	systray.SetIcon(iconOff)
	systray.SetTitle("keepawake")
	systray.SetTooltip(session{}.tooltip(time.Now()))
	t.items = map[choice]*systray.MenuItem{}
	for _, m := range menuChoices {
		it := systray.AddMenuItemCheckbox(m.label, "", false)
		t.items[m.c] = it
		c, d := m.c, m.d
		go func() {
			for range it.ClickedCh {
				t.send(func(e *engine, now time.Time) { e.set(c, d, now, "menu") })
			}
		}()
	}
	t.stop = systray.AddMenuItem("Stop", "Let the PC sleep again")
	t.stop.Disable()
	systray.AddSeparator()
	auto := systray.AddMenuItemCheckbox("Start with Windows", "Start keepawake (off) when you sign in", t.auto.enabled())
	openLog := systray.AddMenuItem("Open log", t.logPath)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "")

	systray.SetOnTapped(func() { t.send(func(e *engine, now time.Time) { e.toggle(now) }) })

	go func() {
		t.eng.run(t.cmds, t.quit)
		close(t.done)
	}()
	go func() {
		for range t.stop.ClickedCh {
			t.send(func(e *engine, now time.Time) { e.off(now, "stopped") })
		}
	}()
	go func() {
		for range auto.ClickedCh {
			on := !auto.Checked()
			if err := t.auto.set(on); err != nil {
				t.log.Printf("ERROR Start with Windows -> %v: %v", on, err)
				t.Notify("keepawake", "Could not change Start with Windows: "+err.Error())
			} else {
				t.log.Printf("Start with Windows: %v (%s)", on, runKeyPath)
			}
			if t.auto.enabled() {
				auto.Check()
			} else {
				auto.Uncheck()
			}
		}
	}()
	go func() {
		for range openLog.ClickedCh {
			if err := openInNotepad(t.logPath); err != nil {
				t.log.Printf("WARNING open log: %v", err)
			}
		}
	}()
	go func() {
		for range quit.ClickedCh {
			wasOn := make(chan bool)
			t.send(func(e *engine, now time.Time) {
				on := e.s.on()
				e.off(now, "quit")
				wasOn <- on
			})
			if <-wasOn {
				time.Sleep(2 * time.Second) // let "PC can sleep again" show before the icon goes
			}
			systray.Quit()
		}
	}()
	if t.show != 0 {
		go func() {
			for {
				if ev, err := windows.WaitForSingleObject(t.show, windows.INFINITE); err != nil || ev != windows.WAIT_OBJECT_0 {
					return
				}
				t.log.Printf("another keepawake was started; it exited")
				t.Notify("keepawake is already running", "It's in the tray: click the icon to turn it on or off, right-click for timers.")
			}
		}()
	}
	if t.welcome {
		t.Notify("keepawake is in the tray", "Click the icon to keep the PC awake (green = on). Right-click for timers. Not visible? Click ^ next to the clock.")
	}
	if t.startOn != nil {
		t.send(t.startOn)
	}
}

func (t *tray) onExit() {
	close(t.quit)
	<-t.done
	t.log.Printf("exit")
}
