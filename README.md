# keepawake

Keeps the computer awake and Teams/Slack presence "Available". macOS today; Windows in progress.

## macOS

Keeps the Mac awake (`caffeinate`) and nudges the pointer 1 px right and back every 30 seconds so Teams/Slack
presence stays "Available".

```bash
macos/keepawake        # until Ctrl+C
macos/keepawake 2h     # for 2 hours
```

First run compiles a tiny helper (needs Xcode command line tools). macOS asks once to allow
**Accessibility** for your terminal app (System Settings → Privacy & Security → Accessibility);
without it the Mac stays awake but presence can still go "Away".

Why a pointer move: new Teams reads the system input-idle timer, so `caffeinate`/Amphetamine alone
(power assertions), zero-distance moves and key-only events (F15) do not keep it green — a real
1 px move does. keepawake checks this itself and warns if macOS is dropping the events.
Teams can still show Away if the screen is locked, or if another device of yours (phone, web)
reports you idle.
