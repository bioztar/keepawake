package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/systray"
)

var version = "dev"

const usage = `keepawake - tray app that keeps Windows awake and Teams "Available" while on.

Double-click to start; it waits in the tray (grey = off, green = on).
Left-click the icon: on until stopped / off. Right-click: timers, Start with Windows, Open log, Quit.

Options:
  --on 2h            start already on (30m, 2h, 90s, or "until")
  --quit-when-off    exit when it turns off (for scripts and tests)
  set KEEPAWAKE_EVERY=60   nudge interval in seconds (default 30)

Log: %LOCALAPPDATA%\keepawake\keepawake.log`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("keepawake", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	onFlag := fs.String("on", "", "")
	quitOff := fs.Bool("quit-when-off", false, "")
	autoLaunch := fs.Bool("autostart", false, "")
	help := fs.Bool("help", false, "")
	if err := fs.Parse(args); err != nil || *help || fs.NArg() > 0 {
		messageBox("keepawake", usage)
		if *help {
			return 0
		}
		return 2
	}
	startChoice, startFor := chOff, time.Duration(0)
	if v := strings.TrimSpace(*onFlag); v != "" {
		if strings.EqualFold(v, "until") {
			startChoice = chUntil
		} else {
			d, err := parseDuration(v)
			if err != nil {
				messageBox("keepawake", "--on: "+err.Error())
				return 2
			}
			startChoice, startFor = chCustom, d
			for _, m := range menuChoices {
				if m.d == d {
					startChoice = m.c
				}
			}
		}
	}
	every, err := parseEvery(os.Getenv("KEEPAWAKE_EVERY"))
	if err != nil {
		messageBox("keepawake", err.Error())
		return 2
	}

	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "keepawake")
	os.MkdirAll(dir, 0o755)
	logPath := filepath.Join(dir, "keepawake.log")
	var out io.Writer = io.Discard
	if f, err := openRolling(logPath, 512<<10); err == nil {
		defer f.Close()
		out = f
	}
	logger := log.New(out, "", log.LstdFlags)
	log.SetOutput(out) // fyne.io/systray reports its own errors via the std logger

	first, showEv, err := claimInstance()
	if err != nil {
		logger.Printf("WARNING single-instance check failed: %v", err)
	}
	if !first && err == nil {
		logger.Printf("second launch (pid %d): told the running keepawake to show itself; exiting", os.Getpid())
		return 0
	}

	exe, _ := os.Executable()
	logger.Printf("keepawake %s started (pid %d, %s, autostart launch=%v)", version, os.Getpid(), exe, *autoLaunch)
	a := autostart{run: hkcuKey(runKeyPath), prefs: hkcuKey(`Software\keepawake`), exe: exe}
	if changed, err := a.ensureDefault(); err != nil {
		logger.Printf("WARNING Start with Windows: %v", err)
	} else if changed {
		logger.Printf("Start with Windows: on (HKCU\\%s\\%s = %s)", runKeyPath, runValueName, a.command())
	}

	t := &tray{
		log: logger, logPath: logPath, auto: a, show: showEv, quitOff: *quitOff,
		welcome: !*autoLaunch && startChoice == chOff,
		cmds:    make(chan command, 16), quit: make(chan struct{}), done: make(chan struct{}),
	}
	t.eng = &engine{p: win32{}, ui: t, logf: logger.Printf, every: every}
	if startChoice != chOff {
		t.startOn = func(e *engine, now time.Time) {
			e.set(startChoice, startFor, now, "--on")
			if !e.s.on() && *quitOff {
				t.exitCode = 1
				go systray.Quit()
			}
		}
	}
	systray.Run(t.onReady, t.onExit)
	return t.exitCode
}
