package main

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestHKCUKey(t *testing.T) {
	path := fmt.Sprintf(`Software\keepawake-test-%d`, os.Getpid())
	defer registry.DeleteKey(registry.CURRENT_USER, path)
	k := hkcuKey(path)
	if _, ok, err := k.Get(runValueName); ok || err != nil {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}
	if err := k.Delete(runValueName); err != nil {
		t.Fatalf("delete on missing key: %v", err)
	}
	if err := k.Set(runValueName, `"C:\a b\keepawake.exe" --autostart`); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := k.Get(runValueName); !ok || err != nil || v != `"C:\a b\keepawake.exe" --autostart` {
		t.Fatalf("Get = %q %v %v", v, ok, err)
	}
	a := autostart{run: k, prefs: hkcuKey(path + `\prefs`), exe: `C:\a b\keepawake.exe`}
	if !a.enabled() {
		t.Fatal("not enabled")
	}
	if err := a.set(false); err != nil || a.enabled() {
		t.Fatalf("set(false): %v", err)
	}
	if err := k.Delete(runValueName); err != nil {
		t.Fatalf("delete missing value: %v", err)
	}
}
