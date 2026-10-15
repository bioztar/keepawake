# keepawake

Keeps the computer awake and Teams/Slack presence "Available" — macOS and Windows 10/11.

It does two things while on: holds a "don't sleep, don't turn the screen off" power request, and
every 30 seconds sends an invisible input event (a mouse move of zero distance — the pointer does
not move) so Windows, and therefore Teams, sees you as active.

> **Check your employer's rules first.** Some companies' IT or HR policies forbid tools that keep
> presence "Available" while you are away. This tool does not hide itself — if your policy says no,
> don't use it.

## Windows

1. **Download** `keepawake-windows-x64.exe` from [Releases](../../releases). That is the one for
   almost every PC — Intel *and* AMD ("x64"). Only Snapdragon/ARM laptops need
   `keepawake-windows-arm64.exe`. No install and no admin rights needed.
2. **Move it out of Downloads** into a folder of your own, e.g. `C:\Users\<you>\keepawake\`
   (create it). Company AppLocker rules often block programs in Downloads or temp folders but allow
   your user folder; Start with Windows also remembers where the exe is, so put it where it can stay.
3. **Double-click it.** The first time, SmartScreen may say *"Windows protected your PC"*: click
   **More info → Run anyway** (the exe is not code-signed, so this is expected).
4. A **coffee-cup icon** appears in the tray next to the clock (if you don't see it, click **^**;
   you can drag it next to the clock). No window opens.

| Icon | Meaning |
|---|---|
| grey | off — the PC sleeps normally |
| green | on — no sleep, no screen-off, Teams stays Available |

- **Click** the icon: on until you stop it / off again.
- **Right-click**: *Keep awake 30 min / 1 hour / 2 hours / 4 hours / Until I stop it*, *Stop*,
  *Start with Windows*, *Open log*, *Quit*. The tooltip shows the time left.
- When the time is up (or Stop/Quit) you get a small notification *"keepawake off — PC can sleep
  again"*.
- **Start with Windows** is on by default: after you sign in keepawake waits in the tray, **off**,
  until you click it. Untick it in the menu to stop that (stored in your user registry,
  `HKCU\...\CurrentVersion\Run` — no admin needed). Starting it a second time does not open a
  second copy.

**Diagnostics:** right-click → **Open log** (`%LOCALAPPDATA%\keepawake\keepawake.log`). After each
nudge keepawake reads the system idle time (`GetLastInputInfo`, what Teams uses) and logs it, e.g.
`nudge ok (mouse): idle 30.0s -> 0.1s`. If the idle time does not drop, it shows one notification
that Teams may go Away — the PC still stays awake. Usual causes: locked screen, Remote Desktop, or
an IT policy that drops simulated input. Teams can also show Away if another device of yours
(phone, web) reports you idle. There is no Microsoft sign-in; keepawake never talks to Teams.

Interval: `setx KEEPAWAKE_EVERY 60` (then sign out and in) sets the nudge interval in seconds
(default 30; Teams goes Away after 5 minutes idle).

### If the exe is blocked

If Windows says *"This app has been blocked by your system administrator"* (AppLocker/WDAC) even
from your user folder, use the script: download `keepawake-windows.zip`, unzip it, and run from
PowerShell in that folder:

```powershell
Unblock-File .\keepawake.ps1
powershell -ExecutionPolicy Bypass -File .\keepawake.ps1        # until Ctrl+C
powershell -ExecutionPolicy Bypass -File .\keepawake.ps1 2h     # for 2 hours (also 30m, 90s)
```

It runs in a console window (closing the window stops it) and does the same as the exe as long as
PowerShell allows `Add-Type`. On locked-down laptops PowerShell may run in **Constrained Language
Mode**; then no script can call Windows directly, and keepawake says so plainly:

- awake: covered only if Windows *presentation mode* is available (`PresentationSettings.exe`,
  most Pro/Enterprise laptops); keepawake turns it on and off for you.
- presence: **not covered** — there is no way to send real input from Constrained Language Mode,
  and tricks like `SendKeys` of a dummy key do not keep new Teams green.

If both the exe and the script are blocked, your IT policy does not allow this kind of tool.

## macOS

Keeps the Mac awake (`caffeinate`) and nudges the pointer 1 px right and back every 30 seconds.

```bash
macos/keepawake        # until Ctrl+C
macos/keepawake 2h     # for 2 hours
```

First run compiles a tiny helper (needs Xcode command line tools). macOS asks once to allow
**Accessibility** for your terminal app (System Settings → Privacy & Security → Accessibility);
without it the Mac stays awake but presence can still go "Away". `KEEPAWAKE_EVERY` works the same
as on Windows.

## Why an input event

New Teams goes Away when the system input-idle timer (`GetLastInputInfo`) passes 5 minutes. Power
requests alone (`SetThreadExecutionState`, `caffeinate`, PowerToys Awake) keep the PC awake but do
not touch that timer, and neither does moving the cursor with `SetCursorPos`. Only an input event
does. On Windows keepawake injects a zero-distance mouse move with `SendInput` (falls back to an F15
key tap if that ever does not reset the timer); CI measures that the idle time drops after each
nudge and that the pointer stays put. Teams still shows Away if the screen is locked.

## Building

```bash
cd windows
go test ./...                                  # unit tests run on any OS
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-H=windowsgui" -o keepawake.exe .
```

`keepawake.exe --on 5s --quit-when-off` turns on at start and exits when it turns off (used by the
CI smoke test). PowerShell tests: `Invoke-Pester windows/tests` (Pester 5). CI
(`.github/workflows/release.yml`) runs everything on `windows-latest` for every PR; pushing a tag
`v*` attaches `keepawake-windows-x64.exe`, `keepawake-windows-arm64.exe` and `keepawake-windows.zip`
(both exes + `keepawake.ps1`) to a GitHub Release (tags with a `-`, like `v0.2.0-rc.1`, become
pre-releases).

## License

[MIT](LICENSE)
