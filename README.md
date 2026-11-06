# LocalRDP (Go 版)

把 Windows 的**子会话（child session）**当成第二张桌面用：用纯 Go 嵌宿 `mstscax.dll`，
通过非公开属性 `ConnectToChildSession` 直接连进 child session，得到一个**带 Explorer 的
完整第二桌面**，可以和当前桌面用 Alt+Tab 互相切换。

- 纯 Go，无 MSVC / CMake / ATL 依赖，`go build` 产出单文件 exe（约 2.2 MB）
- ActiveX 宿主、事件 sink、DISPATCH 封装全部自写
- 只有 `desktop` 需要零特权；`start -- 程序` 需要一次性授予两个账户特权

**完整文档：[使用说明.md](使用说明.md)**

## 快速开始

```pwsh
go build -trimpath -ldflags '-s -w' -o build\bin\Release\localrdp.exe .
.\localrdp.cmd                     # 菜单（推荐）
.\build\bin\Release\localrdp.exe status
.\build\bin\Release\localrdp.exe desktop --user "$env:USERDOMAIN\$env:USERNAME" --password-env LRDP_PW
.\build\bin\Release\localrdp.exe stop
```

`localrdp-ui.ps1` 在找不到 exe 时会自动执行上面的 `go build`。

## 能力矩阵

| 能力 | 状态 |
| --- | --- |
| `desktop` 完整子桌面 | ✅ |
| `status` / `stop` | ✅ |
| 窗口缩放 / DPI / 全屏（Ctrl+Alt+F） | ✅（登录后同步显示设置） |
| 文本剪贴板 | ✅（RDP 自带重定向） |
| `start -- 程序` | ⚠️ 需 `SeTcb` + `SeAssignPrimaryToken` |
| 文件剪贴板桥 | ⚠️ 同上 |
| `--scale` | ⚠️ 本机该属性不可用 |

需要特权时：

```pwsh
pwsh .\install-privileges.ps1     # 管理员，自动提权
# 然后注销重登一次，特权才进入令牌
```

## 目录

```text
main.go            命令行 / 分发
host.go            宿主窗口、消息循环、定时器、全屏、断线
ax.go              ActiveX 容器（站点 + 事件 sink）
rdp.go             IMsRdpClient* 配置、显示同步、断开文案
dispatch.go        IDispatch / VARIANT / DISPPARAMS
env.go             环境探测、status
session_launch.go  把进程放进指定会话（三条回退路径）
session_entry.go   子会话内自举入口
clip_bridge.go     CF_HDROP 文件桥
job.go log.go common.go winutil.go misc.go
localrdp.cmd / localrdp-ui.ps1 / install-privileges.ps1
```

## 许可

基于 [charles-cty/LocalRDP](https://github.com/charles-cty/LocalRDP)（MIT）重写，见 `LICENSE`。
