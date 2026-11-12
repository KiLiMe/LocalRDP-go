# MCP 接口

`localrdp mcp` 在 stdin/stdout 上跑一个 Model Context Protocol 服务器：JSON-RPC 2.0、protocol
`2025-06-18`、只用标准库（exe 仍是单文件）。AI 客户端把它当子进程启动，就能自己看环境、翻日志、
列会话、开关子会话、按按钮，不必再由人复制粘贴日志。

协议层在 `internal/mcp/mcp.go`（不认识 LocalRDP，只做 JSON-RPC 与 tools/resources 两个能力），
工具与资源在 `internal/rdp/mcp.go`（`cmdMcp`、资源）与 `internal/rdp/mcp_tools.go`（9 个工具）。

## 启动

```pwsh
.\build\windows-amd64\localrdp.exe mcp          # stdin/stdout，供客户端调用
.\build\windows-amd64\localrdp.exe mcp --help   # 工具与资源清单
```

- 一个客户端一个进程，stdin 读到 EOF 就退出。日志里留 `mcp: server up (stdio, protocol 2025-06-18)`
  与 `mcp: server down`。
- `initialize` 回的 `serverInfo` 是 `{"name":"localrdp","version":<版本>}`；版本就是
  `localrdp --version` 那个数（`Scripts/build.ps1` 经 `-X` 写进 `versionString`），没走打包脚本时是 `dev`。
- 不需要管理员权限就能起；只有被调用的工具会碰特权（`lrdp_start` 要 `SeTcbPrivilege`，见
  [手动授予特权.md](手动授予特权.md)）。

## 协议

- **一行一条消息**，`\n` 分隔，没有 `Content-Length` 头；最后一行不带换行也照答。
- 客户端在 `initialize` 里报的 `protocolVersion` 只作参考，回答固定是 `2025-06-18`。
- capabilities：`tools.listChanged=false`；`resources.subscribe=false`、`resources.listChanged=false`。
- 方法：`initialize`、`ping`、`tools/list`、`tools/call`、`resources/list`、`resources/read`、`shutdown`、
  `notifications/initialized`、`notifications/cancelled`；其余方法回 `-32601`。
- 没有 `id` 的消息是通知，不回包。
- 坏 JSON：回一条 `{"id":null,...}` 的错误（`-32700`）后继续读。逐行读而不是用 `json.Decoder`，
  因为 decoder 遇到一次语法错就永久失败，整条流会再也答不出东西。
- 工具自身失败不算协议错误：`result.content[0].text` 放错误文本，并带 `isError: true`。
  未知工具 / 未知资源是 `-32602`，`resources/read` 读失败是 `-32603`。
- 工具结果都是一段 JSON 文本（两空格缩进），包在 `content:[{"type":"text","text":...}]` 里。

## 工具

九个，全部只读或只做一次动作。名字都以 `lrdp_` 开头，`tools/list` 按名字排序输出。

| 工具 | 作用 | 参数（默认 / 范围） |
| --- | --- | --- |
| `lrdp_status` | 环境报告，同 `localrdp status` 的机器可读版 | 无 |
| `lrdp_log` | 读 `localrdp.log` 的尾部 | `lines`（120，1–5000）、`level`（INFO/WARN/ERROR）、`grep`（大小写不敏感）、`since_minutes`（0–43200） |
| `lrdp_sessions` | WTS 会话列表 | 无 |
| `lrdp_processes` | 按 exe 名过滤的进程 | `filter`（`localrdp`） |
| `lrdp_diag` | 一次性体检：status + 会话 + 进程 + 最近日志 | `log_lines`（40，1–500） |
| `lrdp_desktop` | 分离启动 `desktop`，返回 pid | `user`/`password`/`password_env`/`width`/`height`/`size`/`scale`/`no_scale_inherit`/`fullscreen`/`title`/`timeout`（5–3600 秒） |
| `lrdp_start` | 分离启动 `start -- <程序>`，返回 pid | `program`（必填）/`args`/`user`/`password`/`password_env`/`workdir` |
| `lrdp_click` | 按按钮，不用鼠标 | `button`（必填）/`pid` |
| `lrdp_stop` | 注销子会话，同 `localrdp stop` | 无 |

参数细节：

- 整数参数越界会被夹到区间里，不报错；`0` 表示“不传这个选项”（`width`/`height`/`scale`/`timeout`）。
- `lrdp_log` 只读日志尾部最多 16 MiB；空行跳过；`level` 按行内 `[LEVEL]` 匹配；`since_minutes` 用
  行首的 `2006-01-02 15:04:05.000` 时间戳判断，解析不出时间的行不按时间过滤。
- `lrdp_desktop` 的 `size` 取 `fit`/`fixed`/`scale`（界面里叫「固定 / 跟随 / 拉伸」，见
  [使用说明.md §5.2](使用说明.md)）；`scale` 只在 `size=scale` 时有意义。
- `lrdp_start` 的 `args` 是一整条命令行，按空格切成多个参数；工作目录走 `workdir`。
- `lrdp_click` 的 `button` 名字与 `localrdp click` 相同（会话窗口：`fullscreen`/`reconnect`/`disconnect`；
  主窗口：`open`/`status`/`stop`/`show`/`close-session`/`forget-password`/`data-folder`/`refresh`/
  `clear-log`/`copy-log`，见 [使用说明.md §12.1](使用说明.md)）。`pid` 只对会话窗口有意义，
  默认找当前开着的那个宿主进程。

## 返回形状

```text
lrdp_status          {version, edition, build, home_edition, elevated, child_sessions_enabled,
                      term_service_running, rdp_listener_enabled, rdp_control_registered,
                      inside_child_session, current_session_id, child_session_id,
                      child_session_alive, transport, transport_state, transport_pipe,
                      fault_code, next_action, ready}
lrdp_log             {path, count, lines}
lrdp_sessions        {sessions: [{id, name, state, user, is_current}]}
lrdp_processes       {filter, processes: [{pid, ppid, name, session_id, threads}]}
lrdp_diag            {status, processes, sessions | sessions_error, log_path, last_lines, warnings, errors}
lrdp_desktop/_start  {pid, command, note}
lrdp_click           {clicked, button, pid} 或 {clicked: false, button, reason}
lrdp_stop            {stopped, session_id} 或 {stopped: false, reason}
```

几个字段值得记：`child_session_id` 在没有子会话时是 `null`；`transport` 是 `0x%08X`、`transport_state`
是人话；`fault_code` 是 `E###`（空串表示没有已知故障，含义见 [故障代码.md](故障代码.md)）；`ready`
等于“没有阻塞项”，`next_action` 就是它给的建议。`lrdp_sessions` 的 `state` 取值：`active`、
`connected`、`connect-query`、`shadow`、`disconnected`、`idle`、`listen`、`reset`、`down`、`init`。

## 资源

| URI | 类型 | 内容 |
| --- | --- | --- |
| `localrdp://status` | `application/json` | 同 `lrdp_status` |
| `localrdp://log` | `text/plain` | `localrdp.log` 最后 300 行 |

资源是“读”的，工具是“问”的：客户端想盯着状态或日志就订阅这两个 URI，要有参数（过滤、行数）
就用工具。

## 客户端配置

Claude Desktop 的 `claude_desktop_config.json`、Cursor 的 `mcp.json` 同结构：

```json
{
  "mcpServers": {
    "localrdp": {
      "command": "C:\\src\\LocalRDP-go\\build\\windows-amd64\\localrdp.exe",
      "args": ["mcp"]
    }
  }
}
```

路径换成自己那份 exe；同一台机器上多个客户端各起一个进程是正常的（互不干扰，都能看同一个
`localrdp.log` 与同一批会话）。

## 约定与安全

- **stdout 只跑协议**。工具里不打印任何东西；`lrdp_desktop` / `lrdp_start` 拉起的子进程用
  `CREATE_NO_WINDOW` + 独立进程组启动，stdout/stderr 直接丢弃，不会污染 JSON-RPC 流。
  要看的输出仍在 `%LOCALAPPDATA%\LocalRDP\localrdp.log`。
- **密码不进命令行**。`password` 参数由服务器写进子进程环境变量 `LRDP_PW`，命令行只出现
  `--password-env LRDP_PW`；也可以传 `password_env` 指向一个自己准备好的变量。
- **没有鉴权**：协议走 stdio，能被谁调用取决于谁起得动这个 exe——即当前登录用户。
  不要把 `mcp` 挂到网络端口上（本项目也没做）。
- **有副作用的三个工具**：`lrdp_desktop` / `lrdp_start` 会真的开窗口、开子会话；
  `lrdp_stop` 会注销子会话（和界面「注销子会话」一样）。剩下的都是只读。
- 工具答案都是 JSON 文本，客户端直接读字段，不用再猜自由格式。

## 排障

- 典型顺序：`lrdp_diag` → `lrdp_log {"level":"ERROR"}` → `lrdp_sessions`（看子会话是否 `active`）→
  `lrdp_log {"grep":"OnDisconnected"}`。`lrdp_diag` 已经把前三样合在一起了，先看它。
- 客户端“工具调不动”：先看日志有没有 `mcp: server up`；再看客户端是不是按**一行一条**发消息；
  坏消息会在日志里留 `mcp: bad message: ...`。
- `lrdp_click` 回 `{"clicked": false, "reason": "no session window is open"}`（会话窗口）或
  `"the window is not open"`（主窗口）→ 窗口没开，先 `lrdp_desktop` 或双击 exe。
- `lrdp_stop` 回 `{"stopped": false, "reason": "no child session is active"}` → 本来就没有子会话，
  不是失败。
- `lrdp_start` 起的程序没出现：多半是缺两项特权，日志里是 `E1xx` 那串
  `WTSQueryUserToken unavailable` / `LaunchInSession failed`（见 [故障代码.md](故障代码.md)）。

## 加一个工具

1. `internal/rdp/mcp_tools.go` 的 `mcpTools()` 里追加一项：`Name`（`lrdp_` 前缀）、`Title`、
   `Description`（说清返回什么，客户端靠它选工具）、`Schema`、`Run`。
2. `Schema` 用 `mcp.ObjectSchema(props, required, false)` + `mcp.Prop("string"|"integer"|"boolean", "说明")`；
   参数在 `Run` 里用 `mcp.ArgString` / `ArgInt(args,key,def,min,max)` / `ArgBool` 取，别自己断言类型。
3. `Run` 返回 `mcp.JSON(v)` 的文本，错误直接 `return "", err`（协议层会转成 `isError`）。
4. 协议层不动；`tools/list` 自动按名字排序。跑 `go test ./internal/mcp ./internal/rdp`。