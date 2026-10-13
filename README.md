# keepawake

Keeps the computer awake and Teams/Slack presence "Available" — macOS and Windows 10/11.

It does two things: holds a "don't sleep" power request, and every 30 seconds moves the pointer
1 px right and back. You won't notice the move; Teams will.

> **Check your employer's rules first.** Some companies' IT or HR policies forbid tools that keep
> presence "Available" while you are away. This tool does not hide itself — if your policy says no,
> don't use it.

## Windows

Download the zip for your PC from [Releases](../../releases) — `windows-amd64` for almost all
laptops, `windows-arm64` for Snapdragon/ARM ones — and unzip it. It contains `keepawake.exe` and
`keepawake.ps1`. No install and no admin rights needed.

**Try the exe first:**

```powershell
.\keepawake.exe        # until Ctrl+C (or close the window)
.\keepawake.exe 2h     # for 2 hours (also 30m, 90s)
```

Double-clicking works too (runs until you close the window). The first time, Windows SmartScreen
may say *"Windows protected your PC"*: click **More info → Run anyway**. The exe is not code-signed,
so this is expected.

**If the exe is blocked** (e.g. *"This app has been blocked by your system administrator"* —
AppLocker/WDAC), **use the script** instead, from PowerShell in the unzipped folder:

```powershell
Unblock-File .\keepawake.ps1
powershell -ExecutionPolicy Bypass -File .\keepawake.ps1        # until Ctrl+C
powershell -ExecutionPolicy Bypass -File .\keepawake.ps1 2h     # for 2 hours
```

The script does the same as the exe as long as PowerShell allows `Add-Type`. On locked-down
laptops PowerShell may run in **Constrained Language Mode**; then no script can call Windows
directly, and keepawake says so plainly:

- awake: covered only if Windows *presentation mode* is available (`PresentationSettings.exe`,
  most Pro/Enterprise laptops); keepawake turns it on and off for you.
- presence: **not covered** — there is no way to send real input from Constrained Language Mode,
  and tricks like `SendKeys` of a dummy key do not keep new Teams green.

If both the exe and the script are blocked, your IT policy does not allow this kind of tool.

Interval: `set KEEPAWAKE_EVERY=60` (cmd) or `$env:KEEPAWAKE_EVERY=60` (PowerShell) sets the nudge
interval in seconds (default 30; Teams goes Away after 5 minutes idle). Stopping (Ctrl+C, closing
the window, or the time running out) returns normal sleep.

Self-check: after each nudge keepawake reads the system idle time (`GetLastInputInfo`); if it did
not drop, it prints a warning — the PC stays awake but presence may still go Away (seen with remote
desktop sessions or a locked screen).

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

## Why a pointer move

New Teams reads the system input-idle timer, so power requests alone (`caffeinate`, Amphetamine,
`SetThreadExecutionState`, PowerToys Awake), zero-distance moves and key-only events (F15) do not
keep it green — a real 1 px move does. keepawake checks this itself and warns if the OS is dropping
the events. Teams can still show Away if the screen is locked, or if another device of yours (phone,
web) reports you idle.

## Building

```bash
cd windows
go test ./...                                  # unit tests run on any OS
GOOS=windows GOARCH=amd64 go build -o keepawake.exe .
```

PowerShell tests: `Invoke-Pester windows/tests` (Pester 5). CI (`.github/workflows/release.yml`)
runs everything on `windows-latest` for every PR; pushing a tag `v*` builds `keepawake.exe` for
amd64 + arm64, zips each with `keepawake.ps1`, and attaches them to a GitHub Release (tags with a
`-`, like `v0.1.0-rc.1`, become pre-releases).

## License

[MIT](LICENSE)
