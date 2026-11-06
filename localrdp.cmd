@echo off
setlocal
rem LocalRDP one-click launcher. Double-click for menu; args pass through.
where pwsh >nul 2>nul || (echo [ERR] PowerShell 7 ^(pwsh^) not found. & pause & exit /b 1)
pwsh -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0localrdp-ui.ps1" %*
if errorlevel 1 pause
endlocal
