#Requires -Version 7.0
<#
  LocalRDP 一键启动器
    .\一键启动.cmd                 -> 菜单
    pwsh .\一键启动.ps1 desktop    -> 直接开子桌面
    pwsh .\一键启动.ps1 start pwsh -> 在子会话里跑 pwsh
    pwsh .\一键启动.ps1 status|stop|setup|shortcut
  密码用 DPAPI(当前用户) 加密缓存到 %LOCALAPPDATA%\LocalRDP\cred.dat，可用 -Forget 清除。
#>
[CmdletBinding()]
param(
    [ValidateSet('menu', 'desktop', 'start', 'status', 'stop', 'setup', 'build', 'shortcut')]
    [string]$Action = 'menu',
    [string]$Program = '',
    [switch]$Fullscreen,
    [switch]$KeepSession,
    [switch]$Forget,
    [switch]$NoCache
)

$ErrorActionPreference = 'Stop'
$Root    = $PSScriptRoot
$Exe     = Join-Path $Root 'build\bin\Release\localrdp.exe'
$DataDir = Join-Path $env:LOCALAPPDATA 'LocalRDP'
$CredFile = Join-Path $DataDir 'cred.dat'
$LogFile = Join-Path $DataDir 'localrdp.log'
$User    = '{0}\{1}' -f $env:USERDOMAIN, $env:USERNAME

function Say([string]$t, [string]$c = 'Gray') { Write-Host $t -ForegroundColor $c }
function Title([string]$t) {
    Write-Host ''
    Write-Host ('  ' + $t) -ForegroundColor Cyan
    Write-Host ('  ' + ('-' * ($t.Length))) -ForegroundColor DarkCyan
}

# ---------- 构建 ----------
function Ensure-Exe {
    if (Test-Path -LiteralPath $Exe) { return $true }
    Say '  未找到 localrdp.exe，开始编译…' 'Yellow'
    if (-not (Get-Command cmake.exe -ErrorAction SilentlyContinue)) {
        Say '  找不到 cmake.exe，请先安装 CMake 与 VS2022。' 'Red'; return $false
    }
    Push-Location $Root
    try { & (Join-Path $Root 'build.ps1') | Out-Host } finally { Pop-Location }
    return (Test-Path -LiteralPath $Exe)
}

# ---------- 密码缓存 ----------
function Get-StoredPassword {
    if (-not (Test-Path -LiteralPath $CredFile)) { return $null }
    try {
        Add-Type -AssemblyName System.Security.Cryptography.ProtectedData -ErrorAction SilentlyContinue
        $blob = [IO.File]::ReadAllBytes($CredFile)
        $raw  = [Security.Cryptography.ProtectedData]::Unprotect($blob, $null, 'CurrentUser')
        return [Text.Encoding]::UTF8.GetString($raw)
    } catch {
        Say "  缓存密码无法解密（可能换了账户或系统），忽略。($($_.Exception.Message))" 'Yellow'
        return $null
    }
}

function Save-StoredPassword([string]$pw) {
    New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
    Add-Type -AssemblyName System.Security.Cryptography.ProtectedData -ErrorAction SilentlyContinue
    $raw  = [Text.Encoding]::UTF8.GetBytes($pw)
    $blob = [Security.Cryptography.ProtectedData]::Protect($raw, $null, 'CurrentUser')
    [IO.File]::WriteAllBytes($CredFile, $blob)
    Say "  密码已缓存（DPAPI/当前用户）：$CredFile" 'DarkGray'
}

function Reset-Password {
    if (Test-Path -LiteralPath $CredFile) { Remove-Item -LiteralPath $CredFile -Force }
    Say '  已清除缓存的密码。' 'Yellow'
}

function Get-Password {
    if (-not $NoCache) { $pw = Get-StoredPassword; if ($pw) { return $pw } }
    Title "需要 $User 的密码（用于子会话登录，不会显示）"
    $sec = Read-Host -AsSecureString
    if (-not $sec -or $sec.Length -eq 0) { throw '没有输入密码。' }
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
    try { $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
    if (-not $NoCache) { Save-StoredPassword $plain }
    return $plain
}

# ---------- 特权检查 ----------
function Test-TcbPrivilege {
    $p = whoami /priv 2>$null
    return ($p -match 'SeTcbPrivilege' -and $p -match 'SeAssignPrimaryTokenPrivilege')
}

# ---------- 组装参数 ----------
function Invoke-LocalRdp([string[]]$LrdpArgs) {
    Say ("  > localrdp " + ($LrdpArgs -join ' ')) 'DarkGray'
    & $Exe @LrdpArgs | Out-Host
    return $LASTEXITCODE
}

function Get-ChildSessionId {
    $out = & $Exe status 2>$null
    foreach ($l in $out) {
        if ($l -match '^\s*ChildSessionId\s*=\s*(\S+)') { return $matches[1].Trim() }
    }
    return '(none)'
}

function Ensure-NoChildSession {
    $id = Get-ChildSessionId
    if ($id -eq '(none)' -or [string]::IsNullOrWhiteSpace($id)) { return }
    Say "  检测到残留子会话 id=$id（Windows 全机只允许 1 个）。" 'Yellow'
    $ans = Read-Host '  先注销它再继续？(Y/n)'
    if ($ans -eq 'n') { throw '已取消（可稍后手动 stop）。' }
    Invoke-LocalRdp @('stop') | Out-Null
    Start-Sleep -Seconds 2
    if ((Get-ChildSessionId) -ne '(none)') { throw '注销失败，请手动执行：localrdp stop' }
    Say '  子会话已注销。' 'Green'
}
function Start-ChildDesktop {
    if (-not (Ensure-Exe)) { throw '编译失败。' }
    Ensure-NoChildSession
    $pw = Get-Password
    $env:LRDP_PW = $pw
    $a = @('desktop', '--user', $User, '--password-env', 'LRDP_PW', '--title', 'LocalRDP 子桌面')
    if ($Fullscreen) { $a += '--fullscreen' }
    if ($KeepSession) { $a += '--keep-session' }
    $rc = Invoke-LocalRdp $a
    Remove-Item Env:LRDP_PW -ErrorAction SilentlyContinue
    return $rc
}

function Start-ChildProgram([string]$cmdline) {
    if ([string]::IsNullOrWhiteSpace($cmdline)) { throw '需要指定要运行的程序。' }
    if (-not (Ensure-Exe)) { throw '编译失败。' }
    Ensure-NoChildSession
    if (-not (Test-TcbPrivilege)) {
        Say '  警告：当前登录令牌没有 SeTcbPrivilege / SeAssignPrimaryTokenPrivilege，' 'Yellow'
        Say '        很可能无法把程序放进子会话（详见 使用说明.md 的"重新登录"一节）。' 'Yellow'
    }
    $pw = Get-Password
    $env:LRDP_PW = $pw
    $progArgs = @( $cmdline -split ' ' | Where-Object { $_ -ne '' } )
    $a = @('start', '--user', $User, '--password-env', 'LRDP_PW')
    if ($Fullscreen)  { $a += '--fullscreen' }
    if ($KeepSession) { $a += '--keep-session' }
    $a += '--'
    $a += $progArgs
    $rc = Invoke-LocalRdp $a
    Remove-Item Env:LRDP_PW -ErrorAction SilentlyContinue
    return $rc
}

function Show-Status { Invoke-LocalRdp @('status') | Out-Null; return $LASTEXITCODE }
function Stop-Session {
    $rc = Invoke-LocalRdp @('stop'); return $rc
}

function New-Shortcut {
    $desktop = [Environment]::GetFolderPath('Desktop')
    $lnk = Join-Path $desktop 'LocalRDP 子桌面.lnk'
    $sh = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
    $sh.TargetPath       = (Join-Path $Root '一键启动.cmd')
    $sh.WorkingDirectory = $Root
    $sh.IconLocation     = "$env:SystemRoot\System32\mstsc.exe,0"
    $sh.Description      = 'LocalRDP：本机 loopback 子会话桌面'
    $sh.Save()
    Say "  已创建快捷方式：$lnk" 'Green'
}

function Show-Menu {
    while ($true) {
        Title "LocalRDP 一键启动器    ($User)"
        $hasCred = Test-Path -LiteralPath $CredFile
        Say ("  密码缓存：{0}    特权：{1}" -f `
                $(if ($hasCred) { '已保存' } else { '未保存' }), `
                $(if (Test-TcbPrivilege) { '已就绪' } else { '需重新登录' })) 'DarkGray'
        Write-Host ''
        Write-Host '  1) 打开子桌面（带 Explorer 的完整第二桌面）'
        Write-Host '  2) 在子会话里运行程序'
        Write-Host '  3) 查看状态'
        $cid = Get-ChildSessionId
        Write-Host ('  4) 停止子会话' + $(if ($cid -ne '(none)') { "（注销会话 $cid）" } else { '（当前无）' }))
        Write-Host '  5) 清除缓存密码'
        Write-Host '  6) 授予特权 / 修复安装（管理员）'
        Write-Host '  7) 创建桌面快捷方式'
        Write-Host '  8) 查看日志'
        Write-Host '  0) 退出'
        $c = Read-Host '  选择'
        switch ($c) {
            '1' { Start-ChildDesktop | Out-Null }
            '2' {
                $p = Read-Host '  程序（例如 pwsh.exe 或 notepad.exe，可带参数）'
                Start-ChildProgram $p | Out-Null
            }
            '3' { Show-Status | Out-Null }
            '4' { Stop-Session | Out-Null }
            '5' { Reset-Password }
            '6' {
                $s = Join-Path $Root 'install-privileges.ps1'
                Start-Process pwsh -Verb RunAs -ArgumentList @('-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$s`"") -Wait
            }
            '7' { New-Shortcut }
            '8' {
                if (Test-Path -LiteralPath $LogFile) { Get-Content -LiteralPath $LogFile -Tail 40 | Out-Host }
                else { Say '  还没有日志。' 'Yellow' }
            }
            '0' { return }
            default { Say '  无效选择。' 'Red' }
        }
    }
}

if ($Forget) {
    Reset-Password
    if (-not $PSBoundParameters.ContainsKey('Action')) { return }
}
# 任何需要 exe 的动作，先确保它存在（原版只在 desktop/start 里检查，
# 导致菜单/status 在未编译时直接报 "not recognized"）。
if ($Action -in @('menu', 'desktop', 'start', 'status', 'stop')) {
    if (-not (Ensure-Exe)) { Say '  没有可用的 localrdp.exe，无法继续。' 'Red'; exit 1 }
}

switch ($Action) {
    'menu'     { Show-Menu }
    'desktop'  { exit (Start-ChildDesktop) }
    'start'    { exit (Start-ChildProgram $Program) }
    'status'   { exit (Show-Status) }
    'stop'     { exit (Stop-Session) }
    'build'    { if (Ensure-Exe) { Say "  OK: $Exe" 'Green' } else { Say '  失败' 'Red' } }
    'shortcut' { New-Shortcut }
    'setup'    {
        $s = Join-Path $Root 'install-privileges.ps1'
        Start-Process pwsh -Verb RunAs -ArgumentList @('-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$s`"") -Wait
    }
}
