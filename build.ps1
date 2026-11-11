#Requires -Version 7.0
<#
  LocalRDP 构建脚本
    pwsh .\build.ps1            -> 打包到 build\windows-amd64\localrdp.exe，
                                   并压成 build\localrdp-<版本>-windows-amd64.zip
    pwsh .\build.ps1 -Clean     -> 先删 build 目录再打包
    pwsh .\build.ps1 -Test      -> 打包前先跑 go vet 与 go test
    pwsh .\build.ps1 -Version 2026.11.11.1
                                -> 断言模式：mygo.json 里的版本必须是这一串（发版时对 tag 用）

  版本号只有一个来源：mygo.json 的 version。它被两处读走——mygo build 写进
  exe 的版本信息，本脚本经 -X 写进程序（`localrdp status` 与 MCP 的 server version）。
  打包完从 exe 属性里读回来对账，对不上就报错，免得两处报两个数。
  图标与版本信息都由 mygo build 从 winres\localrdp.png 与 mygo.json 注入，仓库里没有 .syso。
#>
[CmdletBinding()]
param(
    [switch]$Clean,
    [switch]$Test,
    [string]$Version = ''
)

$ErrorActionPreference = 'Stop'
$Root     = $PSScriptRoot
$Config   = Join-Path $Root 'mygo.json'
$BuildDir = Join-Path $Root 'build'
$OutDir   = Join-Path $BuildDir 'windows-amd64'
$Exe      = Join-Path $OutDir 'localrdp.exe'

if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) {
    Write-Host '  [ERR] 找不到 go.exe，请先安装 Go 1.27+。' -ForegroundColor Red
    exit 1
}
if (-not (Test-Path -LiteralPath $Config)) {
    Write-Host "  [ERR] 找不到 $Config：版本、图标与产物名都在它里面。" -ForegroundColor Red
    exit 1
}

# 版本：mygo.json 是唯一来源，-Version 只是断言。
$target = [string](Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json).version
if ($target -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    Write-Host "  [ERR] mygo.json 的 version 要是四段数字（tag 去掉 v）：'$target'。" -ForegroundColor Red
    exit 1
}
if ($Version -and $Version -ne $target) {
    Write-Host "  [ERR] 版本对不上：-Version 是 $Version，mygo.json 里是 '$target'。" -ForegroundColor Red
    exit 1
}

if ($Clean -and (Test-Path -LiteralPath $BuildDir)) {
    Remove-Item -LiteralPath $BuildDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Push-Location $Root
try {
    $busy = @(Get-Process -Name localrdp -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -eq $Exe } catch { $false }
    })
    if ($busy.Count -gt 0) {
        $ids = ($busy | ForEach-Object { $_.Id }) -join ', '
        Write-Host "  [ERR] 正在运行的 localrdp.exe（PID $ids）占着输出文件：先关掉它的窗口再打包。" -ForegroundColor Red
        exit 1
    }
    if ($Test) {
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] go vet 失败。' -ForegroundColor Red; exit $LASTEXITCODE }
        & go test ./...
        if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] 测试失败。' -ForegroundColor Red; exit $LASTEXITCODE }
    }
    # -X 只能改包级字符串变量；程序里的版本只从这一处注入。mygo build 自己的
    # -ldflags 会盖掉 GOFLAGS，它专门留了 MYGO_LDFLAGS 追加在后面。
    $env:MYGO_LDFLAGS = "-X localrdp/internal/rdp.versionString=$target"
    try {
        & go tool mygo build -platform windows/amd64 -skip-installer -skip-dmg
        if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] mygo build 失败。' -ForegroundColor Red; exit $LASTEXITCODE }
    } finally {
        Remove-Item Env:\MYGO_LDFLAGS -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $Exe)) {
    Write-Host "  [ERR] 没等到产物 $Exe。" -ForegroundColor Red
    exit 1
}

# exe 属性里的版本来自 mygo.json，程序里的是 -X 注入的同一个数字；读回来对账。
$fileVersion = (Get-Item -LiteralPath $Exe).VersionInfo.FileVersion
if ($fileVersion -ne $target) {
    Write-Host "  [ERR] exe 属性里的版本是 '$fileVersion'，mygo.json 是 '$target'。" -ForegroundColor Red
    exit 1
}

$Zip = Join-Path $BuildDir "localrdp-$target-windows-amd64.zip"
if (Test-Path -LiteralPath $Zip) { Remove-Item -LiteralPath $Zip -Force }
Compress-Archive -Path (Join-Path $OutDir '*') -DestinationPath $Zip -CompressionLevel Optimal

$size = [math]::Round((Get-Item -LiteralPath $Exe).Length / 1MB, 2)
$zsize = [math]::Round((Get-Item -LiteralPath $Zip).Length / 1MB, 2)
Write-Host "  [OK] $Exe v$target ($size MB)" -ForegroundColor Green
Write-Host "  [OK] $Zip ($zsize MB)" -ForegroundColor Green
