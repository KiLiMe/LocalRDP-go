# LocalRDP (Go 版)

把 Windows 的**子会话（child session）**当成第二张桌面用：用纯 Go 嵌宿 `mstscax.dll`，
通过非公开属性 `ConnectToChildSession` 直接连进 child session，得到一个**带 Explorer 的
完整第二桌面**，可以和当前桌面用 Alt+Tab 互相切换。

- 纯 Go，无 MSVC / CMake / ATL 依赖，`go build` 产出单文件 exe（约 10 MB）
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

`localrdp-ui.ps1` 在找不到 exe 时调用 `build.ps1` 编译（等价于上面的 `go build`）。

## AI 排查（MCP）

`localrdp mcp` 在 stdin/stdout 上跑 Model Context Protocol 服务器（JSON-RPC 2.0，
protocol `2025-06-18`，零第三方依赖），AI 客户端可直接读状态、翻日志、列会话、启停会话：

```json
{
  "mcpServers": {
    "localrdp": {
      "command": "D:\\Sou\\LocalRDP-go\\build\\bin\\Release\\localrdp.exe",
      "args": ["mcp"]
    }
  }
}
```

工具：`lrdp_status`、`lrdp_log`、`lrdp_sessions`、`lrdp_processes`、`lrdp_diag`、
`lrdp_desktop`、`lrdp_start`、`lrdp_stop`；资源：`localrdp://status`、`localrdp://log`。
详见 [使用说明.md 第 12 节](使用说明.md)。

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
main.go                入口：os.Exit(rdp.Run(os.Args))

internal/rdp/          package rdp：命令行、宿主、COM 容器、会话启动
  run.go               命令行解析 / 分发 / help
  gui_shim.go          GUI 开关解析，交给 internal/app
  host.go              宿主窗口、消息循环、定时器、全屏、断线
  ax.go                ActiveX 容器（站点 + 事件 sink）
  rdp.go               IMsRdpClient* 配置、显示同步、断开文案
  dispatch.go          IDispatch / VARIANT / DISPPARAMS
  env.go               环境探测、status
  session_launch.go    把进程放进指定会话（三条回退路径）
  session_entry.go     子会话内自举入口
  clip_bridge.go       CF_HDROP 文件桥
  start.go             start / stop 命令
  job.go log.go common.go winutil.go misc.go ptr.go

internal/win32/        app 与 rdp 共用的小工具：Hidden/HideWindow、ExePath/ExeDir、
                       UTF16Ptr、Itoa —— 两边不再各写一份

internal/mcp/          Model Context Protocol 服务端（JSON-RPC 2.0 over stdio）：
                       initialize / tools/list / tools/call / resources/*，只用标准库
```

`internal/`（分层，自底向上依赖）：

```text
win32/  纯 Win32 小工具，无依赖
rdp/    命令行与 RDP 宿主；只有 gui_shim.go 向上调 app 开窗口，其余不依赖 app/ui
i18n/   英文文案 → 中文映射；缺条目回落英文，缺口可见
store/  设置、记住的会话、DPAPI 加密密码（叶子包，不依赖其它内部包）
ui/     窗口绘制：标题栏、菜单、控制台表单、会话栏、日志（只读 State）
app/    窗口逻辑：动作、子进程管理、状态刷新、日志跟随
mcp/    MCP 协议层（工具表由 internal/rdp/mcp.go 注册）
```

脚本：`localrdp.cmd` / `localrdp-ui.ps1` / `install-privileges.ps1` / `build.ps1`

CI：`.github/workflows/ci.yml` 在 `windows-latest` 上跑 `go vet` / `gofmt -l` / `go test` / `go build`，
push 与 PR 都触发。

## 许可

基于 [charles-cty/LocalRDP](https://github.com/charles-cty/LocalRDP)（MIT）重写，见 `LICENSE`。
