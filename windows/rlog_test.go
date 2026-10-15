package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "keepawake.log")
	r, err := openRolling(p, 100)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("x", 39) + "\n" // 40 bytes
	for i := 0; i < 5; i++ {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	cur, _ := os.ReadFile(p)
	old, _ := os.ReadFile(p + ".1")
	if len(cur) != 40 || len(old) != 80 {
		t.Errorf("current %d bytes, .1 %d bytes; want 40 and 80", len(cur), len(old))
	}
	// Reopening appends and keeps counting the existing size.
	r, _ = openRolling(p, 100)
	r.Write([]byte(line))
	r.Write([]byte(line))
	r.Close()
	cur, _ = os.ReadFile(p)
	old, _ = os.ReadFile(p + ".1")
	if len(cur) != 40 || len(old) != 80 {
		t.Errorf("after reopen: current %d, .1 %d; want 40 and 80", len(cur), len(old))
	}
}
