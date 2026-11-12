# LocalRDP

用纯 Go 嵌宿 `mstscax.dll`、走非公开属性 `ConnectToChildSession`，为当前用户在本机开一个
带 Explorer 的完整第二子会话桌面，可与当前桌面 Alt+Tab 互切。

## 快速启动

```pwsh
pwsh .\Scripts\build.ps1                         # 1. 打包 → build\windows-amd64\localrdp.exe
.\build\windows-amd64\localrdp.exe               # 2. 打开图形界面，点「打开子会话」
.\build\windows-amd64\localrdp.exe status        # 3. 查看环境与子会话状态
```

更多用法见 [docs/使用说明.md](docs/使用说明.md)；状态栏/日志里的 `E###` 查
[docs/故障代码.md](docs/故障代码.md)。

## 依赖环境

| 项 | 要求 |
| --- | --- |
| 操作系统 | Windows 10 专业版及以上（build 19045 实测）；家庭版无 RDP 服务端，不可用 |
| Go | 1.27.3（`go.mod`） |
| PowerShell | 7+（构建脚本 `Scripts/build.ps1`） |
| 打包工具 | `go tool mygo` v0.7.4（钉在 `go.mod`）；不需要 MSVC / CMake / ATL，产物不依赖 VC 运行时 |
| 运行时特权 | `desktop` 零特权；`start -- 程序` 需 `SeTcbPrivilege` + `SeAssignPrimaryTokenPrivilege`（见 [docs/手动授予特权.md](docs/手动授予特权.md)） |

## 贡献

- 动手前读 `AGENTS.md` 与 [docs/代码导读.md](docs/代码导读.md)；分层依赖方向为 `app → ui → store → i18n`。
- 界面文案只改 `locales/en.json` 与 `locales/zh-Hans.json`，再跑 `go tool mygo generate` 生成 `locales/catalog_gen.go`。
- 版本号只改 `mygo.json` 的 `version`（SemVer），不要手改 `.syso`。
- 提交前必须全绿：

```pwsh
pwsh .\Scripts\build.ps1 -Test      # go vet + go test + 打包
go vet ./... ; gofmt -l .           # gofmt 有输出即失败
```

- 真开窗口的测试需显式开启：`$env:LRDP_LIVE = '1'; go test ./internal/rdp -run Live -v`。

基于 [charles-cty/LocalRDP](https://github.com/charles-cty/LocalRDP)（MIT）重写，许可见 `LICENSE`。