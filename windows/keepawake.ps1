<#
.SYNOPSIS
  keepawake - keep Windows awake and Teams/Slack presence "Available" (script fallback for keepawake.exe).
.DESCRIPTION
  .\keepawake.ps1            run until Ctrl+C
  .\keepawake.ps1 2h         stop after 2 hours (accepts 30m, 2h, 90s)
  $env:KEEPAWAKE_EVERY=60    nudge interval in seconds (default 30; Teams goes Away at 5 min idle)

  Needs Add-Type (FullLanguage mode). In Constrained Language Mode it says what it can and cannot cover.
#>
[CmdletBinding()]
param([Parameter(Position = 0)][string]$Duration)

$KeepAwakeUsage = @'
keepawake - keep Windows awake and Teams/Slack presence "Available".
  .\keepawake.ps1            run until Ctrl+C
  .\keepawake.ps1 2h         stop after 2 hours (accepts 30m, 2h, 90s)
  $env:KEEPAWAKE_EVERY=60    nudge interval in seconds (default 30; Teams goes Away at 5 min idle)
'@

function ConvertTo-KeepAwakeSeconds {
    # Same forms as macos/keepawake: 30m, 2h, 90s, or bare seconds.
    param([string]$Text)
    $t = "$Text".Trim().ToLower()
    if ($t -notmatch '^(\d{1,7})([hms]?)$') { throw "bad duration '$Text' (use e.g. 30m, 2h, 90s)" }
    $n = [long]$Matches[1]
    switch ($Matches[2]) {
        'h' { $n = $n * 3600 }
        'm' { $n = $n * 60 }
    }
    if ($n -le 0) { throw "bad duration '$Text' (use e.g. 30m, 2h, 90s)" }
    if ($n -gt 365 * 86400) { throw "duration '$Text' is too long (max 365 days)" }
    return [int]$n
}

function Get-KeepAwakeEvery {
    # KEEPAWAKE_EVERY in whole seconds; empty means 30.
    param([string]$Value)
    $v = "$Value".Trim()
    if ($v -eq '') { return 30 }
    if ($v -notmatch '^\d{1,5}$' -or [int]$v -le 0 -or [int]$v -gt 86400) {
        throw "bad KEEPAWAKE_EVERY '$Value' (whole seconds, e.g. 30)"
    }
    return [int]$v
}

function Get-KeepAwakeNap {
    # Seconds to sleep before the next nudge; -1 once the deadline has passed. $null deadline = no limit.
    param([int]$Every, $Deadline, [datetime]$Now)
    if ($null -eq $Deadline) { return $Every }
    $left = [int](($Deadline - $Now).TotalSeconds)
    if ($left -le 0) { return -1 }
    if ($left -lt $Every) { return $left }
    return $Every
}

function Write-KeepAwakeLog {
    param([string]$Message)
    Write-Host $Message
}

$KeepAwakeNativeSource = @'
using System;
using System.Runtime.InteropServices;
namespace KeepAwake {
    public static class Native {
        public const uint ES_CONTINUOUS = 0x80000000, ES_SYSTEM_REQUIRED = 1, ES_DISPLAY_REQUIRED = 2;
        [DllImport("kernel32.dll")] public static extern uint SetThreadExecutionState(uint flags);
        [DllImport("kernel32.dll")] static extern uint GetTickCount();
        [StructLayout(LayoutKind.Sequential)] struct LASTINPUTINFO { public uint cbSize; public uint dwTime; }
        [DllImport("user32.dll")] static extern bool GetLastInputInfo(ref LASTINPUTINFO info);
        [StructLayout(LayoutKind.Sequential)] struct MOUSEINPUT { public int dx, dy; public uint mouseData, dwFlags, time; public IntPtr dwExtraInfo; }
        [StructLayout(LayoutKind.Sequential)] struct KEYBDINPUT { public ushort wVk, wScan; public uint dwFlags, time; public IntPtr dwExtraInfo; }
        // INPUT is a union; MOUSEINPUT is its largest arm, so both layouts have sizeof(INPUT).
        [StructLayout(LayoutKind.Sequential)] struct INPUT { public uint type; public MOUSEINPUT mi; }
        [StructLayout(LayoutKind.Sequential)] struct KEYINPUT { public uint type; public KEYBDINPUT ki; public uint pad1, pad2; }
        [DllImport("user32.dll", SetLastError = true)] static extern uint SendInput(uint n, INPUT[] inputs, int size);
        [DllImport("user32.dll", SetLastError = true, EntryPoint = "SendInput")] static extern uint SendKeys(uint n, KEYINPUT[] inputs, int size);

        public static bool Awake() { return SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED) != 0; }
        public static bool Release() { return SetThreadExecutionState(ES_CONTINUOUS) != 0; }

        // A real (injected) mouse-move event with zero distance: resets the idle timer that
        // Teams reads (GetLastInputInfo) and the pointer does not move. If that did not reset
        // idle time, an F15 tap (no app uses F15) is tried. Returns 0 or a Win32 error code.
        public static int Nudge() {
            INPUT[] m = new INPUT[1];
            m[0].type = 0; m[0].mi.dwFlags = 1; // INPUT_MOUSE, MOUSEEVENTF_MOVE, dx = dy = 0
            if (SendInput(1, m, Marshal.SizeOf(typeof(INPUT))) != 1) return Marshal.GetLastWin32Error();
            System.Threading.Thread.Sleep(50);
            long idle = IdleMs();
            if (idle >= 0 && idle < 1000) return 0;
            KEYINPUT[] k = new KEYINPUT[2];
            k[0].type = 1; k[0].ki.wVk = 0x7E;                     // INPUT_KEYBOARD, VK_F15 down
            k[1].type = 1; k[1].ki.wVk = 0x7E; k[1].ki.dwFlags = 2; // KEYEVENTF_KEYUP
            if (SendKeys(2, k, Marshal.SizeOf(typeof(INPUT))) != 2) return Marshal.GetLastWin32Error();
            System.Threading.Thread.Sleep(50);
            return 0;
        }
        public static long IdleMs() {
            LASTINPUTINFO li = new LASTINPUTINFO(); li.cbSize = (uint)Marshal.SizeOf(typeof(LASTINPUTINFO));
            if (!GetLastInputInfo(ref li)) return -1;
            return (long)unchecked(GetTickCount() - li.dwTime);
        }
    }
}
'@

function Initialize-KeepAwakeNative {
    # Returns $null when the Win32 helpers are ready, else the reason they are not.
    $mode = "$($ExecutionContext.SessionState.LanguageMode)"
    if ($mode -ne 'FullLanguage') {
        if ($mode -eq 'ConstrainedLanguage') { return 'PowerShell is in Constrained Language Mode (usually set by IT policy), which blocks Add-Type' }
        return "PowerShell is in $mode mode, which blocks Add-Type"
    }
    try {
        if (-not ('KeepAwake.Native' -as [type])) { Add-Type -TypeDefinition $KeepAwakeNativeSource -ErrorAction Stop }
        return $null
    } catch {
        return "Add-Type failed: $($_.Exception.Message)"
    }
}

function New-KeepAwakeNativePlatform {
    @{
        Name    = 'native'
        Awake   = { [KeepAwake.Native]::Awake() }
        Release = { [KeepAwake.Native]::Release() }
        Nudge   = { [KeepAwake.Native]::Nudge() }
        IdleMs  = { [KeepAwake.Native]::IdleMs() }
        Now     = { Get-Date }
        Sleep   = { param($s) Start-Sleep -Seconds $s }
    }
}

function Find-PresentationSettings {
    if (-not $env:WINDIR) { return $null }
    foreach ($dir in 'Sysnative', 'System32') {
        $p = Join-Path $env:WINDIR "$dir\PresentationSettings.exe"
        if (Test-Path -LiteralPath $p) { return $p }
    }
    return $null
}

function Start-KeepAwakeLoop {
    # Platform: hashtable of scriptblocks Awake/Release/Nudge/IdleMs/Now/Sleep (tests pass fakes).
    param([hashtable]$Platform, [int]$Every, $Deadline)
    if (-not (& $Platform.Awake)) {
        Write-KeepAwakeLog 'keepawake: SetThreadExecutionState failed'
        return 1
    }
    Write-KeepAwakeLog 'keepawake: SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED|ES_DISPLAY_REQUIRED) ok'
    $until = ''
    if ($null -ne $Deadline) { $until = ", until $($Deadline.ToString('HH:mm'))" }
    Write-KeepAwakeLog "keepawake: on (nudge every ${Every}s$until). Ctrl+C to stop."
    $warned = $false
    try {
        while ($true) {
            $err = & $Platform.Nudge
            if ($err -ne 0 -and -not $warned) {
                Write-KeepAwakeLog "keepawake: WARNING - input nudge failed (Win32 error $err); the PC stays awake but Teams/Slack may go Away."
                $warned = $true
            }
            $idle = & $Platform.IdleMs
            if ($idle -gt 5000 -and -not $warned) {
                Write-KeepAwakeLog "keepawake: WARNING - the nudge is not resetting idle time (idle $([int]($idle / 1000))s); the PC stays awake"
                Write-KeepAwakeLog '  but Teams/Slack may go Away. Something (policy, remote session, locked screen) is dropping the input.'
                $warned = $true
            }
            $nap = Get-KeepAwakeNap -Every $Every -Deadline $Deadline -Now (& $Platform.Now)
            if ($nap -lt 0) {
                Write-KeepAwakeLog 'keepawake: time is up'
                return 0
            }
            & $Platform.Sleep $nap
        }
    } finally {
        if (& $Platform.Release) { Write-KeepAwakeLog 'keepawake: off (normal sleep restored)' }
        else { Write-KeepAwakeLog 'keepawake: could not restore normal sleep (it returns when PowerShell exits)' }
    }
}

function Start-KeepAwakeFallback {
    # No Win32 access: be explicit about what is and is not covered.
    param([string]$Reason, [int]$Every, $Deadline)
    Write-KeepAwakeLog "keepawake: $Reason."
    Write-KeepAwakeLog '  This script cannot call Windows directly, so it cannot send input or hold a power request.'
    $pres = Find-PresentationSettings
    if ($null -eq $pres) {
        Write-KeepAwakeLog '  awake:    NOT covered (PresentationSettings.exe is not available on this Windows edition)'
        Write-KeepAwakeLog '  presence: NOT covered - Teams/Slack will go Away after ~5 min without input'
        Write-KeepAwakeLog '  Nothing left to do here. Try keepawake.exe; if both are blocked, your IT policy does not allow this.'
        return 1
    }
    Write-KeepAwakeLog '  awake:    covered by Windows presentation mode (PresentationSettings.exe /start)'
    Write-KeepAwakeLog '  presence: NOT covered - Teams/Slack will go Away after ~5 min without input (no real input events possible here)'
    Start-Process -FilePath $pres -ArgumentList '/start' -WindowStyle Hidden
    $until = ''
    if ($null -ne $Deadline) { $until = " until $($Deadline.ToString('HH:mm'))" }
    Write-KeepAwakeLog "keepawake: on (presentation mode$until). Ctrl+C to stop."
    try {
        while ($true) {
            $nap = Get-KeepAwakeNap -Every $Every -Deadline $Deadline -Now (Get-Date)
            if ($nap -lt 0) {
                Write-KeepAwakeLog 'keepawake: time is up'
                return 0
            }
            Start-Sleep -Seconds $nap
        }
    } finally {
        Start-Process -FilePath $pres -ArgumentList '/stop' -WindowStyle Hidden
        Write-KeepAwakeLog 'keepawake: off (presentation mode stopped)'
    }
}

function Invoke-KeepAwake {
    param([string]$Duration)
    if ($Duration -in '-h', '--help', '/?', 'help') {
        Write-KeepAwakeLog $KeepAwakeUsage
        return 0
    }
    $deadline = $null
    try {
        if ($Duration -ne '') { $deadline = (Get-Date).AddSeconds((ConvertTo-KeepAwakeSeconds $Duration)) }
        $every = Get-KeepAwakeEvery $env:KEEPAWAKE_EVERY
    } catch {
        Write-KeepAwakeLog "keepawake: $($_.Exception.Message)"
        Write-KeepAwakeLog $KeepAwakeUsage
        return 2
    }
    $reason = Initialize-KeepAwakeNative
    if ($null -ne $reason) { return Start-KeepAwakeFallback -Reason $reason -Every $every -Deadline $deadline }
    return Start-KeepAwakeLoop -Platform (New-KeepAwakeNativePlatform) -Every $every -Deadline $deadline
}

# Dot-sourcing (". .\keepawake.ps1", as the Pester tests do) only loads the functions.
if ($MyInvocation.InvocationName -ne '.') { exit (Invoke-KeepAwake $Duration) }
