#Requires -Modules @{ ModuleName = 'Pester'; ModuleVersion = '5.0' }

BeforeAll {
    . (Join-Path $PSScriptRoot '..\keepawake.ps1')

    function New-FakePlatform {
        param([long]$IdleMs = 0, [int]$NudgeError = 0, [bool]$AwakeOk = $true, [int]$InterruptAfter = 0)
        $s = @{ Now = [datetime]'2026-10-13T12:00:00'; Naps = New-Object System.Collections.ArrayList
            Nudges = 0; Released = $false; IdleMs = $IdleMs; NudgeError = $NudgeError; AwakeOk = $AwakeOk
            InterruptAfter = $InterruptAfter }
        $p = @{
            State   = $s
            Awake   = { $s.AwakeOk }.GetNewClosure()
            Release = { $s.Released = $true; $true }.GetNewClosure()
            Nudge   = { $s.Nudges++; $s.NudgeError }.GetNewClosure()
            IdleMs  = { $s.IdleMs }.GetNewClosure()
            Now     = { $s.Now }.GetNewClosure()
            Sleep   = {
                param($sec)
                if ($s.InterruptAfter -gt 0 -and $s.Naps.Count -ge $s.InterruptAfter) { throw 'Ctrl+C' }
                [void]$s.Naps.Add($sec); $s.Now = $s.Now.AddSeconds($sec)
            }.GetNewClosure()
        }
        return $p
    }
}

Describe 'ConvertTo-KeepAwakeSeconds' {
    It 'parses <in> as <want>s' -TestCases @(
        @{ in = '90s'; want = 90 }, @{ in = '90'; want = 90 }, @{ in = '30m'; want = 1800 },
        @{ in = '2h'; want = 7200 }, @{ in = '2H'; want = 7200 }, @{ in = ' 5s '; want = 5 }, @{ in = '1s'; want = 1 }
    ) {
        ConvertTo-KeepAwakeSeconds $in | Should -Be $want
    }
    It 'rejects <in>' -TestCases @(
        @{ in = '' }, @{ in = 'h' }, @{ in = '0' }, @{ in = '0m' }, @{ in = '-5m' }, @{ in = '1.5h' },
        @{ in = '2d' }, @{ in = 'abc' }, @{ in = '1h30m' }, @{ in = '9999999h' }
    ) {
        { ConvertTo-KeepAwakeSeconds $in } | Should -Throw
    }
}

Describe 'Get-KeepAwakeEvery' {
    It 'defaults to 30' { Get-KeepAwakeEvery '' | Should -Be 30 }
    It 'reads 60' { Get-KeepAwakeEvery '60' | Should -Be 60 }
    It 'rejects <in>' -TestCases @(@{ in = '0' }, @{ in = '-1' }, @{ in = 'abc' }, @{ in = '30s' }, @{ in = '1.5' }, @{ in = '86401' }) {
        { Get-KeepAwakeEvery $in } | Should -Throw
    }
}

Describe 'Get-KeepAwakeNap' {
    BeforeAll { $now = [datetime]'2026-10-13T12:00:00' }
    It 'runs forever without a deadline' { Get-KeepAwakeNap -Every 30 -Deadline $null -Now $now | Should -Be 30 }
    It 'uses the interval when far from the deadline' { Get-KeepAwakeNap -Every 30 -Deadline $now.AddHours(1) -Now $now | Should -Be 30 }
    It 'uses the interval exactly at it' { Get-KeepAwakeNap -Every 30 -Deadline $now.AddSeconds(30) -Now $now | Should -Be 30 }
    It 'shortens the last nap' { Get-KeepAwakeNap -Every 30 -Deadline $now.AddSeconds(10) -Now $now | Should -Be 10 }
    It 'is done at the deadline' { Get-KeepAwakeNap -Every 30 -Deadline $now -Now $now | Should -Be -1 }
    It 'is done after the deadline' { Get-KeepAwakeNap -Every 30 -Deadline $now.AddSeconds(-1) -Now $now | Should -Be -1 }
}

Describe 'Start-KeepAwakeLoop' {
    BeforeEach { Mock Write-KeepAwakeLog {} }

    It 'nudges on schedule until the deadline, then restores sleep' {
        $p = New-FakePlatform
        $code = Start-KeepAwakeLoop -Platform $p -Every 30 -Deadline $p.State.Now.AddSeconds(75)
        $code | Should -Be 0
        $p.State.Naps | Should -Be @(30, 30, 15)
        $p.State.Nudges | Should -Be 4
        $p.State.Released | Should -BeTrue
        Should -Invoke Write-KeepAwakeLog -ParameterFilter { $Message -like '*SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED|ES_DISPLAY_REQUIRED) ok*' } -Times 1 -Exactly
        Should -Invoke Write-KeepAwakeLog -ParameterFilter { $Message -like '*WARNING*' } -Times 0 -Exactly
    }

    It 'restores sleep when interrupted' {
        $p = New-FakePlatform -InterruptAfter 2
        { Start-KeepAwakeLoop -Platform $p -Every 30 -Deadline $null } | Should -Throw 'Ctrl+C'
        $p.State.Nudges | Should -Be 3
        $p.State.Released | Should -BeTrue
    }

    It 'warns once when the nudge does not reset idle time' {
        $p = New-FakePlatform -IdleMs 40000
        Start-KeepAwakeLoop -Platform $p -Every 30 -Deadline $p.State.Now.AddMinutes(5) | Should -Be 0
        Should -Invoke Write-KeepAwakeLog -ParameterFilter { $Message -like '*WARNING*idle*' } -Times 1 -Exactly
    }

    It 'warns once when SendInput fails' {
        $p = New-FakePlatform -NudgeError 5
        Start-KeepAwakeLoop -Platform $p -Every 30 -Deadline $p.State.Now.AddMinutes(2) | Should -Be 0
        Should -Invoke Write-KeepAwakeLog -ParameterFilter { $Message -like '*WARNING*error 5*' } -Times 1 -Exactly
    }

    It 'fails without nudging when SetThreadExecutionState fails' {
        $p = New-FakePlatform -AwakeOk $false
        Start-KeepAwakeLoop -Platform $p -Every 30 -Deadline $null | Should -Be 1
        $p.State.Nudges | Should -Be 0
        $p.State.Released | Should -BeFalse
    }
}

Describe 'Invoke-KeepAwake' {
    BeforeEach { Mock Write-KeepAwakeLog {} }
    It 'exits 2 on a bad duration' { Invoke-KeepAwake 'soon' | Should -Be 2 }
    It 'exits 2 on a bad KEEPAWAKE_EVERY' {
        $old = $env:KEEPAWAKE_EVERY
        try { $env:KEEPAWAKE_EVERY = 'x'; Invoke-KeepAwake '5s' | Should -Be 2 } finally { $env:KEEPAWAKE_EVERY = $old }
    }
    It 'prints help' { Invoke-KeepAwake '--help' | Should -Be 0 }
    It 'falls back honestly when Win32 is unavailable' {
        Mock Initialize-KeepAwakeNative { 'PowerShell is running in ConstrainedLanguage mode' }
        Mock Find-PresentationSettings { $null }
        Invoke-KeepAwake '5s' | Should -Be 1
        Should -Invoke Write-KeepAwakeLog -ParameterFilter { $Message -like '*presence: NOT covered*' } -Times 1 -Exactly
    }
}
