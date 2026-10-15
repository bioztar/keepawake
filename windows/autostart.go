package main

// regValues is one registry key's string values; the real one is HKCU in registry_windows.go,
// tests use a map.
type regValues interface {
	Get(name string) (value string, ok bool, err error)
	Set(name, value string) error
	Delete(name string) error // no error if missing
}

const (
	runValueName  = "keepawake"
	prefAutostart = "Autostart"
	autostartFlag = "--autostart"
)

// autostart manages HKCU\...\CurrentVersion\Run (no admin needed). It is on by default; once the
// user unticks it, prefs remember "off" so later launches do not turn it back on.
type autostart struct {
	run, prefs regValues
	exe        string
}

func (a autostart) command() string { return `"` + a.exe + `" ` + autostartFlag }

func (a autostart) enabled() bool {
	v, ok, err := a.run.Get(runValueName)
	return err == nil && ok && v == a.command()
}

// ensureDefault runs at every launch: registers this exe unless the user turned autostart off.
// It also repoints the Run value if the exe was moved.
func (a autostart) ensureDefault() (changed bool, err error) {
	if v, ok, _ := a.prefs.Get(prefAutostart); ok && v == "off" {
		return false, nil
	}
	if a.enabled() {
		return false, nil
	}
	return true, a.run.Set(runValueName, a.command())
}

func (a autostart) set(on bool) error {
	if on {
		if err := a.run.Set(runValueName, a.command()); err != nil {
			return err
		}
		return a.prefs.Set(prefAutostart, "on")
	}
	if err := a.run.Delete(runValueName); err != nil {
		return err
	}
	return a.prefs.Set(prefAutostart, "off")
}
