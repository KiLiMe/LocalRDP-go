# LocalRDP (Go 版)

把 Windows 的**子会话（child session）**当成第二张桌面用：用纯 Go 嵌宿 `mstscax.dll`，
通过非公开属性 `ConnectToChildSession` 直接连进 child session，得到一个**带 Explorer 的
完整第二桌面**，可以和当前桌面用 Alt+Tab 互相切换。

- 纯 Go，无 MSVC / CMake / ATL 依赖，`go build` 产出单文件 exe（约 10 MB）
- ActiveX 宿主、事件 sink、DISPATCH 封装全部自写
- 只有 `desktop` 需要零特权；`start -- 程序` 需要一次性授予两个账户特权

**完整文档：[docs/使用说明.md](docs/使用说明.md)**；出错时状态栏/日志里的 `E###` 查
[故障代码.md](docs/故障代码.md)。

## 快速开始

```pwsh
go build -trimpath -ldflags '-s -w -H=windowsgui' -o build\bin\Release\localrdp.exe .
.\build\bin\Release\localrdp.exe          # 图形界面（推荐）
.\build\bin\Release\localrdp.exe status
.\build\bin\Release\localrdp.exe desktop --user "$env:USERDOMAIN\$env:USERNAME" --password-env LRDP_PW
.\build\bin\Release\localrdp.exe stop
```

`-H=windowsgui` 让 exe 成为图形子系统程序：双击不会有黑框控制台。从终端调用时程序自己
`AttachConsole` 到父终端，`status` 的输出照常看得见；被重定向到管道或文件时用继承来的句柄，
`| Out-File`、`$out = & localrdp.exe status` 也照常工作。

图形界面里「会话设置 → 启动程序」填要跑的命令行（如 `notepad.exe`），连上后它就在子会话里
启动，程序退出了桌面还在；留空则只开桌面。

## AI 排查（MCP）

`localrdp mcp` 在 stdin/stdout 上跑 Model Context Protocol 服务器（JSON-RPC 2.0，
protocol `2025-06-18`，零第三方依赖），AI 客户端可直接读状态、翻日志、列会话、启停会话：

```json
{
  "mcpServers": {
    "localrdp": {
      "command": "C:\\src\\LocalRDP-go\\build\\bin\\Release\\localrdp.exe",
      "args": ["mcp"]
    }
  }
}
```

工具：`lrdp_status`、`lrdp_log`、`lrdp_sessions`、`lrdp_processes`、`lrdp_diag`、
`lrdp_desktop`、`lrdp_start`、`lrdp_stop`、`lrdp_click`；资源：`localrdp://status`、`localrdp://log`。
详见 [docs/使用说明.md 第 12 节](docs/使用说明.md)。

不开鼠标也能按按钮：每个界面按钮都有命令行入口，`localrdp click --list` 全部列出。

```pwsh
localrdp click fullscreen     # 会话窗口顶栏：全屏 / 退出全屏
localrdp click reconnect      # 会话窗口顶栏：重连
localrdp click open           # 主窗口：打开子会话
```

## 能力矩阵

| 能力 | 状态 |
| --- | --- |
| `desktop` 完整子桌面 | ✅ |
| `status` / `stop` | ✅ |
| 窗口缩放 / DPI / 全屏（Ctrl+Alt+F） | ✅（登录后同步显示设置） |
| 会话窗口顶栏：全屏 / 重连 / 断开 | ✅ 自绘 40px 栏，重连不关窗口 |
| 窗口大小三档（固定 / 跟随 / 缩放） | ✅ `--size fit\|fixed\|scale` |
| 文本剪贴板 | ✅（RDP 自带重定向） |
| `start -- 程序` | ⚠️ 需 `SeTcb` + `SeAssignPrimaryToken` |
| 文件剪贴板桥 | ⚠️ 同上 |
| `--scale N` | ✅ 桌面按 N% 渲染（见 Q5） |

需要特权时（`start` 模式）：按 **[手动授予特权.md](docs/手动授予特权.md)** 手动授予两项账户权限，
然后注销重登一次，特权才进入令牌。程序里不再提供授权按钮。

## 目录

完整代码地图（每个包做什么、启动链路、改动前必读的坑、阅读顺序）见
**[docs/代码导读.md](docs/代码导读.md)**。这里只画到包级：

```text
main.go                  入口：os.Exit(rdp.Run(os.Args))
internal/rdp/            命令行 + RDP 宿主（COM 容器、会话启动、顶栏、mcp）
internal/app/  ui/       窗口逻辑 / 只读 State 的绘制
internal/store/ i18n/    设置与 DPAPI 密码 / 中英文案
internal/win32/ mcp/     Win32 小工具 / MCP 协议层
build.ps1                构建：版本对账 + -X 注入
rsrc_windows_amd64.syso  exe 的图标与版本信息（go build 自动链入）
```

`internal/` 是分层的：依赖方向 `app → ui → store → i18n`，`rdp` 不 import `app`/`ui`（
唯一的例外是 `internal/rdp/gui_shim.go` 调 `app.Run` 开窗口）。GUI 按钮靠 `exec` 重新调用
本 exe 的 `desktop`/`start`/`status`/`stop` 衔接，所以 `internal/app` 不 import `rdp`。

## 许可

基于 [charles-cty/LocalRDP](https://github.com/charles-cty/LocalRDP)（MIT）重写，见 `LICENSE`。
