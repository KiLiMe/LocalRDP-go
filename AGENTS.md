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
internal/store/          设置与 DPAPI 密码
internal/i18n/           语言选择：中文还是英文
internal/win32/ mcp/     Win32 小工具 / MCP 协议层
locales/                 消息：en.json 源 + zh-Hans.json 中文，catalog_gen.go 是生成的
Scripts/build.ps1         构建：mygo 打包 + -X 注入 + zip 发布
mygo.json                exe 的名字、版本、图标、产物目录（mygo 的打包配置）
```

`internal/` 是分层的：依赖方向 `app → ui → store → i18n`，`rdp` 不 import `app`/`ui`（
唯一的例外是 `internal/rdp/gui_shim.go` 调 `app.Run` 开窗口）。GUI 按钮靠 `exec` 重新调用
本 exe 的 `desktop`/`start`/`status`/`stop` 衔接，所以 `internal/app` 不 import `rdp`。

## 代码约定

> 始终以干净、模块化的方式写代码，架构合理、职责分离。不要因为是我提的需求就硬塞功能。如果找不到自然的方式实现，且你认为该重构才能让整体更干净，那就提前停下来说明。

- 新增依赖前先确认依赖方向没被打穿，分层见上面「目录」一节。
- 界面文案只写在 `locales/en.json` 与 `locales/zh-Hans.json` 里，改完跑 `go tool mygo generate`
  重新生成 `locales/catalog_gen.go`（生成物，别手改），代码里调 `locales.Settings()` 这样的
  accessor：拿英文当 key 查表的写法已经被这一层替掉了。
- 版本号是 SemVer（`主.次.修`，如 `1.4.2`），只认 `mygo.json` 的 `version`：改版本只改这一处，`pwsh .\Scripts\build.ps1` 会经 `MYGO_LDFLAGS`
  把同一个数字注入程序，并从 exe 属性里读回来对账。图标与版本信息由 `go tool mygo build`
  从 `winres/localrdp.png` 与 `mygo.json` 注入，仓库里没有 `.syso`，也**不要**手工塞一个进去
  （有非 mygo 的 `.syso` 时 mygo 会让位给它，图标与版本就不会被注入了）。
- 只跑 Windows，Win32 调用走 `internal/win32` 与 `github.com/lxn/win`。
- 新增已知故障：`internal/fault/fault.go` 加常量 + 一句话，`docs/故障代码.md` 加一节；
  两边都要动，`internal/fault/fault_test.go` 守著。界面只显示代码与一句话。

## 构建与验证

```pwsh
pwsh .\Scripts\build.ps1 -Test      # go vet + go test + 打包到 build\windows-amd64\localrdp.exe 与 zip
go vet ./... ; gofmt -l .           # CI 同款检查，gofmt 有输出即失败
$env:LRDP_LIVE = '1'; go test ./internal/rdp -run Live -v   # 真开窗口的那层，见下
```

测试分两层：默认的 `go test` 只跑纯函数（不建窗口、不连会话，CI 就靠这一层）；
`*_live_windows_test.go` 里的 `TestLive…` 会真的起一个子会话、量窗口矩形再注销，
所以只有设了 `LRDP_LIVE=1` 才跑。跑之前先关掉自己的窗口（有一个会话窗口开着时它们会跳过），
注销偶尔要几十秒，是会话在收尾，不是卡死。

发版不手工：改 `mygo.json` 的 `version` → 提交推上 `main` → `git tag -a v<版本>` 推上去。
`.github/workflows/ci.yml` 只有一个 workflow，`release` job 只认 `v*` tag，负责用
`build.ps1 -Version <tag 去 v>` 打包并建 Release。
