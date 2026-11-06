#Requires -Version 7.0
<#
  一次性授权：给当前用户加上 LocalRDP 需要的两个"账户权限"
    SeTcbPrivilege              = 作为操作系统的一部分    -> WTSQueryUserToken
    SeAssignPrimaryTokenPrivilege = 替换进程级令牌        -> CreateProcessAsUser
  必须以管理员身份运行（脚本会自己提权）。
#>
[CmdletBinding()]
param([switch]$Remove)

$ErrorActionPreference = 'Stop'

# ---- 自提权 ----
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
           ).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host '需要管理员权限，正在提权…' -ForegroundColor Yellow
    $argList = @('-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"")
    if ($Remove) { $argList += '-Remove' }
    Start-Process pwsh -Verb RunAs -ArgumentList $argList -Wait
    return
}

Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class LsaApi {
    [StructLayout(LayoutKind.Sequential)]
    public struct LSA_UNICODE_STRING {
        public ushort Length; public ushort MaximumLength; public IntPtr Buffer;
    }
    [StructLayout(LayoutKind.Sequential)]
    public struct LSA_OBJECT_ATTRIBUTES {
        public int Length; public IntPtr RootDirectory; public IntPtr ObjectName;
        public int Attributes; public IntPtr SecurityDescriptor; public IntPtr SecurityQualityOfService;
    }
    [DllImport("advapi32.dll", SetLastError = true)]
    public static extern uint LsaOpenPolicy(ref LSA_UNICODE_STRING SystemName,
        ref LSA_OBJECT_ATTRIBUTES ObjectAttributes, uint DesiredAccess, out IntPtr PolicyHandle);
    [DllImport("advapi32.dll", SetLastError = true)]
    public static extern uint LsaAddAccountRights(IntPtr PolicyHandle, byte[] AccountSid,
        LSA_UNICODE_STRING[] UserRights, uint CountOfRights);
    [DllImport("advapi32.dll", SetLastError = true)]
    public static extern uint LsaRemoveAccountRights(IntPtr PolicyHandle, byte[] AccountSid,
        bool AllRights, LSA_UNICODE_STRING[] UserRights, uint CountOfRights);
    [DllImport("advapi32.dll")]
    public static extern uint LsaClose(IntPtr PolicyHandle);
    [DllImport("advapi32.dll")]
    public static extern int LsaNtStatusToWinError(uint Status);
}
"@

$rights = @('SeTcbPrivilege', 'SeAssignPrimaryTokenPrivilege')
$me     = [Security.Principal.WindowsIdentity]::GetCurrent()
$user   = $me.Name
$sid    = $me.User

function New-LsaString([string]$s) {
    $bytes = [Text.Encoding]::Unicode.GetBytes($s)
    $buf   = [Runtime.InteropServices.Marshal]::AllocHGlobal($bytes.Length)
    [Runtime.InteropServices.Marshal]::Copy($bytes, 0, $buf, $bytes.Length)
    $lsa = New-Object LsaApi+LSA_UNICODE_STRING
    $lsa.Length = [uint16]$bytes.Length
    $lsa.MaximumLength = [uint16]($bytes.Length + 2)
    $lsa.Buffer = $buf
    return $lsa
}

$sidBytes = [byte[]]::new($sid.BinaryLength)
$sid.GetBinaryForm($sidBytes, 0)

$oa = New-Object LsaApi+LSA_OBJECT_ATTRIBUTES
$oa.Length = [Runtime.InteropServices.Marshal]::SizeOf($oa)
$sysName = New-Object LsaApi+LSA_UNICODE_STRING
$policy = [IntPtr]::Zero
# POLICY_CREATE_ACCOUNT (0x10) | POLICY_LOOKUP_NAMES (0x800) = 0x810
$st = [LsaApi]::LsaOpenPolicy([ref]$sysName, [ref]$oa, 0x00000810, [ref]$policy)
if ($st -ne 0) { throw "LsaOpenPolicy 失败: $([LsaApi]::LsaNtStatusToWinError($st))" }

$lsaRights = @($rights | ForEach-Object { New-LsaString $_ })

if ($Remove) {
    $st = [LsaApi]::LsaRemoveAccountRights($policy, $sidBytes, $false, $lsaRights, [uint32]$lsaRights.Count)
} else {
    $st = [LsaApi]::LsaAddAccountRights($policy, $sidBytes, $lsaRights, [uint32]$lsaRights.Count)
}
$err = [LsaApi]::LsaNtStatusToWinError($st)
[void][LsaApi]::LsaClose($policy)

if ($st -ne 0) { throw ('Lsa 操作失败 ntstatus=0x{0:X8} win32={1}' -f $st, $err) }

Write-Host ''
Write-Host ("  {0} -> {1}" -f ($(if ($Remove) { '已移除' } else { '已授予' })), $user) -ForegroundColor Green
$rights | ForEach-Object { Write-Host "    · $_" }
Write-Host ''
Write-Host '  验证（本机策略）：' -ForegroundColor Cyan
$tmp = Join-Path $env:TEMP ("secpol_{0}.inf" -f $PID)
secedit /export /areas USER_RIGHTS /cfg $tmp | Out-Null
Get-Content $tmp | Select-String -Pattern ($rights -join '|') | ForEach-Object { Write-Host "    $($_.Line)" }
Remove-Item $tmp -Force -ErrorAction SilentlyContinue
Write-Host ''
Write-Host '  注意：特权写进的是"登录时生成的令牌"，当前已登录的会话不会自动生效。' -ForegroundColor Yellow
Write-Host '        请注销后重新登录（或重启）一次，之后新进程才有 SeTcbPrivilege。' -ForegroundColor Yellow
if (-not $Remove) { Write-Host '        撤销：把本脚本加 -Remove 再运行一次。' -ForegroundColor DarkGray }
