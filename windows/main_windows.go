package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var version = "dev"

// SetThreadExecutionState is per thread: keep main on one OS thread.
func init() { runtime.LockOSThread() }

func main() {
	os.Exit(realMain(os.Args[1:]))
}

func realMain(args []string) int {
	var deadline time.Time
	switch {
	case len(args) > 1:
		fmt.Fprint(os.Stderr, usage)
		return 2
	case len(args) == 1:
		switch args[0] {
		case "-h", "--help", "/?", "help":
			fmt.Print(usage)
			return 0
		case "-v", "--version", "version":
			fmt.Println("keepawake", version)
			return 0
		}
		d, err := parseDuration(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "keepawake: %v\n\n%s", err, usage)
			return 2
		}
		deadline = time.Now().Add(d)
	}
	every, err := parseEvery(os.Getenv("KEEPAWAKE_EVERY"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "keepawake: %v\n", err)
		return 2
	}

	sig := make(chan os.Signal, 1)
	// Ctrl+C -> Interrupt; closing the console window / logoff -> SIGTERM.
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	wait := func(d time.Duration) bool {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
			return true
		case <-sig:
			return false
		}
	}
	return loop{p: win32{}, every: every, deadline: deadline, now: time.Now, wait: wait, out: os.Stdout}.run()
}
