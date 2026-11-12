# LocalRDP

用纯 Go 嵌宿 `mstscax.dll`、走非公开属性 `ConnectToChildSession`，给当前用户在本机开一个带 Explorer 的完整第二桌面。

## 快速启动

```pwsh
pwsh .\Scripts\build.ps1                         # 1. 打包 → build\windows-amd64\localrdp.exe
.\build\windows-amd64\localrdp.exe               # 2. 打开图形界面，点「打开子会话」
.\build\windows-amd64\localrdp.exe status        # 3. 查看环境与子会话状态
```

## 依赖环境

| 项 | 要求 |
| --- | --- |
| 操作系统 | Windows 10 专业版及以上（build 19045 实测）；家庭版无 RDP 服务端，不可用 |
| Go | 1.27.3（`go.mod`） |
| PowerShell | 7+（构建脚本 `Scripts/build.ps1`） |
| 打包工具 | `go tool mygo` v0.7.4（钉在 `go.mod`）；不需要 MSVC / CMake / ATL，产物不依赖 VC 运行时 |
| 运行时特权 | `desktop` 零特权；`start -- 程序` 需 `SeTcbPrivilege` + `SeAssignPrimaryTokenPrivilege`（见 [docs/手动授予特权.md](docs/手动授予特权.md)） |

## 许可

基于 [charles-cty/LocalRDP](https://github.com/charles-cty/LocalRDP)（MIT）重写，同样以 MIT 分发，许可见 `LICENSE`。