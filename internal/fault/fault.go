// Package fault gives each failure the program knows a code. The message the
// user sees stays one line — the code and what it means — and the checks, the
// evidence and the fix live in docs/故障代码.md under the same code.
package fault

import "strings"

// Code is the tag a known failure carries: the hundreds digit says where it
// happens (E1 environment, E2 connecting, E3 inside the child session).
type Code string

const (
	// Before the session window opens: what the machine lacks.
	Edition          Code = "E101" // 家庭版没有远程桌面服务端
	RdpControl       Code = "E102" // mstscax.dll 的控件没注册
	InsideChild      Code = "E103" // 已经在子会话里运行
	ChildSessionsOff Code = "E104" // 子会话功能没开
	Transport        Code = "E105" // 子会话传输不可用
	TermService      Code = "E106" // TermService 没在运行

	// While the session window connects and logs in.
	NoChildSupport Code = "E201" // 控件不支持 ConnectToChildSession
	TLSHandshake   Code = "E202" // 本机传输的 TLS 握手被中断
	ConnectTimeout Code = "E203" // 等不到连接或登录
	LogonRejected  Code = "E204" // 账号或密码被拒
	TakenOver      Code = "E205" // 子会话被另一个连接接管
	Disconnected   Code = "E206" // 连接被断开（其它原因）
	ControlBroken  Code = "E207" // 控件起不来或报致命错误

	// Inside the child session, once it is up.
	LaunchFailed     Code = "E301" // 子会话里启动程序失败
	ClipBridgeFailed Code = "E302" // 子会话里的剪贴板桥起不来
)

// about is the one line the program shows for a code.
var about = map[Code]string{
	Edition:          "Windows 家庭版没有远程桌面服务端，换专业版/企业版/教育版/Server",
	RdpControl:       "远程桌面控件（mstscax.dll）没有注册，组件可能被精简或用的是精简版系统",
	InsideChild:      "当前进程就在子会话里，得从父会话启动",
	ChildSessionsOff: "Windows 的子会话功能是关的，需要执行一次开启（要管理员确认）",
	Transport:        "子会话传输建不起来，通常是被安全软件拦住或系统组件受损",
	TermService:      "TermService 没在运行，本机 RDP 服务端不可用",
	NoChildSupport:   "这个 RDP 控件不支持子会话，换用完整版 Windows 的 mstscax.dll",
	TLSHandshake:     "连接在 TLS 握手阶段被本机中断，多为 TermService 的传输状态坏了",
	ConnectTimeout:   "等到超时也没连上，见日志最后几行",
	LogonRejected:    "子会话拒绝了这次登录，账号或密码不对",
	TakenOver:        "子会话被另一个连接接管了（同时只允许一个）",
	Disconnected:     "连接被断开",
	ControlBroken:    "远程桌面控件起不来或报了致命错误",
	LaunchFailed:     "子会话起来了，但要在里面运行的程序没能启动",
	ClipBridgeFailed: "子会话里的剪贴板桥没起来，文件复制粘贴会不可用（文本不受影响）",
}

// doc is where the details for a code are kept, from the project root.
const doc = "docs/故障代码.md"

// Short is what the code means, in one line.
func (c Code) Short() string { return about[c] }

// String makes a code print as itself.
func (c Code) String() string { return string(c) }

// Known says whether this program hands out the code: Find relies on it to
// tell a fault line from a log line.
func (c Code) Known() bool {
	_, ok := about[c]
	return ok
}

// Line is what the program prints for a fault: the code, what it means, and
// where the rest is written. detail carries what only this run knows (0xD07,
// an HRESULT) and is left out when empty.
func Line(c Code, detail string) string {
	s := string(c) + " " + c.Short()
	if detail != "" {
		s += "（" + detail + "）"
	}
	return s + "，详见 " + doc
}

// Find picks the fault line out of what a session printed, so the window can
// show it after the session process is gone.
func Find(lines []string) (string, bool) {
	for _, l := range lines {
		token, _, _ := strings.Cut(strings.TrimSpace(l), " ")
		if Code(token).Known() {
			return strings.TrimSpace(l), true
		}
	}
	return "", false
}
