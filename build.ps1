#Requires -Version 7.0
<#
  LocalRDP 构建脚本
    pwsh .\build.ps1            -> 编译到 build\bin\Release\localrdp.exe
    pwsh .\build.ps1 -Clean     -> 先删 build 目录再编译
    pwsh .\build.ps1 -Test      -> 编译前先跑 go vet 与 go test
    pwsh .\build.ps1 -Version 2026.11.7.1
                                -> 断言模式：要求 rsrc_windows_amd64.syso 里的
                                   file-version 已经是这一串（version.txt 不参与）

  版本号只有一个来源：winres\version.txt。它被两处读走——
  这里写进程序（`localrdp status` 与 MCP 的 server version），
  由 tools\resync.ps1 写进 rsrc_windows_amd64.syso（exe 属性）。
  两边对不上时本脚本拒绝编译，免得 exe 属性和 status 报两个数。
#>
[CmdletBinding()]
param(
    [switch]$Clean,
    [switch]$Test,
    [string]$Version = ''
)

$ErrorActionPreference = 'Stop'
$Root    = $PSScriptRoot
$BuildDir = Join-Path $Root 'build'
$Exe     = Join-Path $BuildDir 'bin\Release\localrdp.exe'
$VersionFile = Join-Path $Root 'winres\version.txt'
$Syso        = Join-Path $Root 'rsrc_windows_amd64.syso'

# exe 属性里的版本来自 syso 的 VS_FIXEDFILEINFO，与 Go 变量无关，只能读出来对账。
function Get-SysoFileVersion([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $b = [IO.File]::ReadAllBytes($Path)
    for ($i = 0; $i -le $b.Length - 16; $i++) {
        # dwSignature = 0xFEEF04BD，小端序；其后是 struc 版本，再后是 file 版本的高/低 32 位。
        if ($b[$i] -eq 0xBD -and $b[$i + 1] -eq 0x04 -and $b[$i + 2] -eq 0xEF -and $b[$i + 3] -eq 0xFE) {
            $ms = [BitConverter]::ToUInt32($b, $i + 8)
            $ls = [BitConverter]::ToUInt32($b, $i + 12)
            return '{0}.{1}.{2}.{3}' -f ($ms -shr 16), ($ms -band 0xFFFF), ($ls -shr 16), ($ls -band 0xFFFF)
        }
    }
    return ''
}

if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) {
    Write-Host '  [ERR] 找不到 go.exe，请先安装 Go 1.27+。' -ForegroundColor Red
    exit 1
}

# 版本：-Version 是断言，缺省用 winres\version.txt 里的那一串。
$target = $Version
if (-not $target -and (Test-Path -LiteralPath $VersionFile)) {
    $target = (Get-Content -LiteralPath $VersionFile -Raw).Trim()
}
if ($target) {
    if ($target -notmatch '^\d+\.\d+\.\d+\.\d+$') {
        Write-Host "  [ERR] 版本号要是四段数字（tag 去掉 v）：'$target'。" -ForegroundColor Red
        exit 1
    }
    $sysoVersion = Get-SysoFileVersion $Syso
    if ($sysoVersion -ne $target) {
        $src = if ($Version) { '-Version' } else { 'winres\version.txt' }
        Write-Host "  [ERR] 版本对不上：$src 是 $target，rsrc_windows_amd64.syso 里是 '$sysoVersion'。" -ForegroundColor Red
        Write-Host '        先重生成资源（图标与版本信息都在里面）：pwsh .\tools\resync.ps1' -ForegroundColor Yellow
        exit 1
    }
}

if ($Clean -and (Test-Path -LiteralPath $BuildDir)) {
    Remove-Item -LiteralPath $BuildDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Exe) | Out-Null

Push-Location $Root
try {
    $busy = @(Get-Process -Name localrdp -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -eq $Exe } catch { $false }
    })
    if ($busy.Count -gt 0) {
        $ids = ($busy | ForEach-Object { $_.Id }) -join ', '
        Write-Host "  [ERR] 正在运行的 localrdp.exe（PID $ids）占着输出文件：先关掉它的窗口再编译。" -ForegroundColor Red
        exit 1
    }
    if ($Test) {
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] go vet 失败。' -ForegroundColor Red; exit $LASTEXITCODE }
        & go test ./...
        if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] 测试失败。' -ForegroundColor Red; exit $LASTEXITCODE }
    }
    $ldflags = '-s -w -H=windowsgui'
    if ($target) {
        # -X 只能改包级字符串变量；版本号只从这一处注入。
        $ldflags += " -X localrdp/internal/rdp.versionString=$target"
    }
    & go build -trimpath -ldflags $ldflags -o $Exe .
    if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] 编译失败。' -ForegroundColor Red; exit $LASTEXITCODE }
} finally {
    Pop-Location
}

$size = [math]::Round((Get-Item -LiteralPath $Exe).Length / 1MB, 2)
$ver = if ($target) { " v$target" } else { '' }
Write-Host "  [OK] $Exe$ver ($size MB)" -ForegroundColor Green
