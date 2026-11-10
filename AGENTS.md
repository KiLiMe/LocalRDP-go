# AGENTS.md

本文件是项目的说明索引与工程约定。根目录只放 `README.md`（面向使用者）和本文件，
其余文档都在 `docs/`。

## 文档

| 文件 | 内容 |
| --- | --- |
| `README.md` | 项目是什么、快速开始、能力矩阵 |
| `docs/使用说明.md` | 完整手册：编译、两种模式、窗口与顶栏、原理、FAQ、MCP、安全 |
| `docs/代码导读.md` | 代码地图：目录、每个包的职责、进程链路、改动前必读的坑、阅读顺序 |
| `docs/手动授予特权.md` | `start` 模式需要的 `SeTcbPrivilege` + `SeAssignPrimaryTokenPrivilege`：手动授予 / 验证 / 撤销 |
| `docs/故障代码.md` | `E###`：每个已知故障的含义、判据、处理;程序只报代码与一句话，细节在这里 |

新增文档放进 `docs/` 并在上表加一行；一个事实只写一处，别在 README 与手册里各维护一份。
文档里举路径一律用中性示例（`C:\src\LocalRDP-go`），不写本机绝对路径、用户名、机器名。

## 目录

完整代码地图（每个包做什么、启动链路、改动前必读的坑、阅读顺序）见
`docs/代码导读.md`。这里只画到包级：

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

## 代码约定

> 始终以干净、模块化的方式写代码，架构合理、职责分离。不要因为是我提的需求就硬塞功能。如果找不到自然的方式实现，且你认为该重构才能让整体更干净，那就提前停下来说明。

- 新增依赖前先确认依赖方向没被打穿，分层见上面「目录」一节。
- 版本号只认 `winres/version.txt`：改版本走 `pwsh .\tools\resync.ps1` + `pwsh .\build.ps1` 对账，
  不手改 `rsrc_windows_amd64.syso`。
- 只跑 Windows，Win32 调用走 `internal/win32` 与 `github.com/lxn/win`。
- 新增已知故障：`internal/fault/fault.go` 加常量 + 一句话，`docs/故障代码.md` 加一节；
  两边都要动，`internal/fault/fault_test.go` 守著。界面只显示代码与一句话。

## 构建与验证

```pwsh
pwsh .\build.ps1 -Test      # go vet + go test + 编译到 build\bin\Release\localrdp.exe
go vet ./... ; gofmt -l .   # CI 同款检查，gofmt 有输出即失败
```
