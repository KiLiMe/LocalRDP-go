# AGENTS.md

根目录只放 `README.md`（面向使用者）和本文件；其余文档都在 `docs/`：`使用说明.md`、
`代码导读.md`、`手动授予特权.md`、`故障代码.md`。本文件只写**路径**与**约定**——用法、原理、
构建细节、故障说明一律放 `docs/`，一个事实只写一处。

## 路径

完整代码地图（每个包的职责、关键文件、进程链路、改动前必读的坑、阅读顺序）见
`docs/代码导读.md`。这里只画到包级：

```text
main.go                  入口：os.Exit(rdp.Run(os.Args))
internal/rdp/            命令行 + RDP 宿主（COM 容器、会话启动、顶栏、mcp）
internal/app/  ui/       窗口逻辑 / 只读 State 的绘制
internal/store/          设置与 DPAPI 密码
internal/i18n/           语言选择：中文还是英文
internal/fault/          已知故障：`E###` + 一句话（细节在 docs/故障代码.md）
internal/win32/ mcp/     Win32 小工具 / MCP 协议层
locales/                 消息：en.json 源 + zh-Hans.json 中文，catalog_gen.go 是生成的
tools/                   genicon（画图标）、barpreview（预览顶栏）
winres/                  图标源 PNG：mygo 缩成各档；会话窗口那张 go:embed
Scripts/build.ps1        构建：mygo 打包 + -X 注入；带 -Version 的发版路径多压一个 zip
mygo.json                exe 的名字、版本、图标、产物目录（mygo 的打包配置）
docs/                    使用说明 / 代码导读 / 手动授予特权 / 故障代码
```

## 约定

> 始终以干净、模块化的方式写代码，架构合理、职责分离。不要因为是我提的需求就硬塞功能。如果找不到自然的方式实现，且你认为该重构才能让整体更干净，那就提前停下来说明。

- 分层：依赖方向 `app → ui → store → i18n`；`rdp` 不 import `app`/`ui`（唯一的例外是
  `internal/rdp/gui_shim.go` 调 `app.Run` 开窗口）。GUI 按钮靠 `exec` 重新调用本 exe 的
  `desktop`/`start`/`status`/`stop` 衔接，所以 `internal/app` 不 import `rdp`。新增依赖前先确认
  没打穿。
- 文档：新增文档放进 `docs/`，并在 `docs/代码导读.md` 的目录树里加一行；文档里举路径一律用中性
  示例（`C:\src\LocalRDP-go`），不写本机绝对路径、用户名、机器名。
- 界面文案只写在 `locales/en.json` 与 `locales/zh-Hans.json` 里，改完跑 `go tool mygo generate`
  重新生成 `locales/catalog_gen.go`（生成物，别手改），代码里调 `locales.Settings()` 这样的
  accessor：拿英文当 key 查表的写法已经被这一层替掉了。
- 版本号是 SemVer（`主.次.修`，如 `1.4.2`），只认 `mygo.json` 的 `version`：改版本只改这一处，
  `pwsh .\Scripts\build.ps1` 会经 `MYGO_LDFLAGS` 把同一个数字注入程序，并从 exe 属性里读回来对账。
  图标与版本信息由 `go tool mygo build` 从 `winres/localrdp.png` 与 `mygo.json` 注入，仓库里没有
  `.syso`，也**不要**手工塞一个进去（有非 mygo 的 `.syso` 时 mygo 会让位给它，图标与版本就不会被注入）。
- 发版不手工：改 `mygo.json` 的 `version` → 提交推上 `main` → 打 `v<版本>` tag 推上去，剩下的
  交给 CI（见 `docs/使用说明.md` §2）。
- 只跑 Windows，Win32 调用走 `internal/win32` 与 `github.com/lxn/win`。
- 新增已知故障：`internal/fault/fault.go` 加常量 + 一句话，`docs/故障代码.md` 加一节；
  两边都要动，`internal/fault/fault_test.go` 守着。界面只显示代码与一句话。
- 提交前过与 CI 同款的检查：`go vet ./...`、`gofmt -l .`（有输出即失败）、
  `pwsh .\Scripts\build.ps1 -Test`。命令与两层测试（`LRDP_LIVE=1` 才跑 `TestLive…`）见
  `docs/代码导读.md` 的「构建与资源」与「测试与验证」。
