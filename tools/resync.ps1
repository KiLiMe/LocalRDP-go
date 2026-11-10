#Requires -Version 7.0
<#
  重新生成 exe 里的资源：winres\localrdp.ico（图标）与 rsrc_windows_amd64.syso
  （图标 + 版本信息）。版本号取自 winres\version.txt。

    pwsh .\tools\resync.ps1

  只有改了 tools\genicon\main.go 的图形、或者换了版本号时才需要跑。
  产物已入库，日常 go build / build.ps1 不需要 go-winres。
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Root        = Split-Path -Parent $PSScriptRoot
$VersionFile = Join-Path $Root 'winres\version.txt'

if (-not (Get-Command go-winres -ErrorAction SilentlyContinue)) {
    Write-Host '  [ERR] 找不到 go-winres：go install github.com/tc-hib/go-winres@latest' -ForegroundColor Red
    exit 1
}

$version = (Get-Content -LiteralPath $VersionFile -Raw).Trim()
if ($version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    Write-Host "  [ERR] winres\version.txt 里不是四段版本号：'$version'" -ForegroundColor Red
    exit 1
}

Push-Location $Root
try {
    & go run tools\genicon\main.go
    if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] genicon 失败。' -ForegroundColor Red; exit $LASTEXITCODE }

    & go-winres simply --icon winres\localrdp.ico --manifest none --product-name LocalRDP `
        --file-description 'Local loopback remote desktop in a child session' `
        --file-version $version --product-version $version --out rsrc --arch amd64
    if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] go-winres 失败。' -ForegroundColor Red; exit $LASTEXITCODE }
} finally {
    Pop-Location
}

Write-Host "  [OK] winres\localrdp.ico 与 rsrc_windows_amd64.syso（v$version）" -ForegroundColor Green
