#Requires -Version 7.0
<#
  LocalRDP 构建脚本
    pwsh .\build.ps1            -> 编译到 build\bin\Release\localrdp.exe
    pwsh .\build.ps1 -Clean     -> 先删 build 目录再编译
    pwsh .\build.ps1 -Test      -> 编译前先跑 go vet 与 go test
#>
[CmdletBinding()]
param(
    [switch]$Clean,
    [switch]$Test
)

$ErrorActionPreference = 'Stop'
$Root    = $PSScriptRoot
$BuildDir = Join-Path $Root 'build'
$Exe     = Join-Path $BuildDir 'bin\Release\localrdp.exe'

if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) {
    Write-Host '  [ERR] 找不到 go.exe，请先安装 Go 1.27+。' -ForegroundColor Red
    exit 1
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
    & go build -trimpath -ldflags '-s -w -H=windowsgui' -o $Exe .
    if ($LASTEXITCODE -ne 0) { Write-Host '  [ERR] 编译失败。' -ForegroundColor Red; exit $LASTEXITCODE }
} finally {
    Pop-Location
}

$size = [math]::Round((Get-Item -LiteralPath $Exe).Length / 1MB, 2)
Write-Host "  [OK] $Exe ($size MB)" -ForegroundColor Green
