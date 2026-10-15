package main

import (
	"errors"
	"testing"
)

type fakeReg struct {
	m      map[string]string
	setErr error
}

func newFakeReg() *fakeReg { return &fakeReg{m: map[string]string{}} }

func (f *fakeReg) Get(name string) (string, bool, error) { v, ok := f.m[name]; return v, ok, nil }
func (f *fakeReg) Set(name, v string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.m[name] = v
	return nil
}
func (f *fakeReg) Delete(name string) error { delete(f.m, name); return nil }

func TestAutostartOnByDefault(t *testing.T) {
	run, prefs := newFakeReg(), newFakeReg()
	a := autostart{run: run, prefs: prefs, exe: `C:\Users\v\keepawake\keepawake.exe`}
	if a.enabled() {
		t.Fatal("enabled before first launch")
	}
	changed, err := a.ensureDefault()
	if err != nil || !changed {
		t.Fatalf("first launch: changed=%v err=%v", changed, err)
	}
	if got := run.m["keepawake"]; got != `"C:\Users\v\keepawake\keepawake.exe" --autostart` {
		t.Errorf("Run value = %q", got)
	}
	if !a.enabled() {
		t.Error("not enabled after first launch")
	}
	if changed, _ := a.ensureDefault(); changed {
		t.Error("second launch rewrote the Run value")
	}
}

func TestAutostartUserTurnsOff(t *testing.T) {
	run, prefs := newFakeReg(), newFakeReg()
	a := autostart{run: run, prefs: prefs, exe: `C:\k\keepawake.exe`}
	a.ensureDefault()
	if err := a.set(false); err != nil {
		t.Fatal(err)
	}
	if _, ok := run.m["keepawake"]; ok || a.enabled() {
		t.Fatal("Run value still there")
	}
	if changed, _ := a.ensureDefault(); changed || a.enabled() {
		t.Fatal("next launch turned autostart back on")
	}
	if err := a.set(true); err != nil || !a.enabled() || prefs.m["Autostart"] != "on" {
		t.Fatalf("turn back on: err=%v enabled=%v", err, a.enabled())
	}
}

func TestAutostartFollowsMovedExe(t *testing.T) {
	run, prefs := newFakeReg(), newFakeReg()
	autostart{run: run, prefs: prefs, exe: `C:\Users\v\Downloads\keepawake.exe`}.ensureDefault()
	moved := autostart{run: run, prefs: prefs, exe: `C:\Users\v\Apps\keepawake.exe`}
	if moved.enabled() {
		t.Fatal("moved exe counts as enabled")
	}
	if changed, err := moved.ensureDefault(); !changed || err != nil || !moved.enabled() {
		t.Fatalf("moved: changed=%v err=%v", changed, err)
	}
}

func TestAutostartSetError(t *testing.T) {
	run := newFakeReg()
	run.setErr = errors.New("access denied")
	a := autostart{run: run, prefs: newFakeReg(), exe: `C:\k.exe`}
	if _, err := a.ensureDefault(); err == nil {
		t.Error("want error")
	}
	if err := a.set(true); err == nil {
		t.Error("want error")
	}
}
