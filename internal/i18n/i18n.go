// Package i18n translates the interface: the strings are English in the
// code and the map here turns them into what the window shows, so a string
// left out of the map stays English and can be seen.
package i18n

import (
	"os"
	"strings"
	"sync"
)

// zh maps the interface's English to Chinese. A string missing here shows
// as it is in the code, on purpose: a gap is visible, not silent.
var zh = map[string]string{
	"LocalRDP console": "LocalRDP 控制台",

	// The band.
	"New session…": "新会话…",
	"Settings":     "设置",
	"Host window":  "宿主窗口",
	"Bridge":       "桥",
	"About bridge": "关于桥",

	// The console: the session form and what it does.
	"Session":            "会话",
	"Session list":       "会话列表",
	"Options":            "选项",
	"Privileges ready":   "特权已就绪",
	"Privileges missing": "特权未就绪",
	"Timeout (seconds)":  "超时（秒）",
	"New session":        "新会话",
	"Account":            "账号",
	"User name":          "用户名",
	"Password":           "密码",
	"Password is cached. Type a new one to replace it.": "已缓存密码，输入新密码则替换。",
	"Keep the password": "缓存密码",
	"Keep the password between runs, encrypted for this user.": "运行之间记住密码（按当前用户加密）。",
	"Mode":          "模式",
	"Program":       "程序",
	"Run a program": "启动程序",
	"Starts in the child session once it connects. Leave it empty for the desktop alone.": "连上后在这个子会话里运行；留空则只显示桌面。",
	"Program and arguments, as a command line":                                            "程序与参数，按命令行写法，例如 notepad.exe C:\\a.txt",
	"Title":              "标题",
	"Width":              "宽",
	"Height":             "高",
	"Timeout":            "超时",
	"Second":             "秒",
	"Full screen":        "全屏",
	"Keep session":       "保留会话",
	"Hidden":             "隐藏窗口",
	"Minimized":          "最小化",
	"Actions":            "操作",
	"Open child session": "打开子会话",
	"Open the child session with these settings.": "用上面的参数打开子桌面或启动程序。",
	"Status":                    "状态",
	"Log off the child session": "注销子会话",
	"Grant privileges":          "授予特权",
	"Forget password":           "清理密码",
	"Shortcut":                  "快捷方式",
	"Open the data folder":      "打开数据目录",

	// The status panel.
	"Environment":     "环境",
	"Cached password": "密码缓存",
	"Privileges":      "特权",
	"Child session":   "子会话",
	"Saved":           "已保存",
	"Not saved":       "未保存",
	"Ready":           "已就绪",
	"Missing":         "未就绪",
	"None":            "无",
	"Refresh":         "刷新",
	"SeTcbPrivilege and SeAssignPrimaryTokenPrivilege are granted.":                                   "SeTcbPrivilege 与 SeAssignPrimaryTokenPrivilege 已具备。",
	"start needs SeTcbPrivilege and SeAssignPrimaryTokenPrivilege; grant them, then sign out and in.": "start 需要 SeTcbPrivilege 与 SeAssignPrimaryTokenPrivilege；授予后须注销重登。",
	"desktop needs no privilege: privileges only matter for start.":                                   "desktop 不需要特权，特权只对 start 有用。",

	// The log panel.
	"Log":  "日志",
	"Copy": "复制",

	// What the form complains about.
	"The user name is required.":                             "用户名不能为空。",
	"start needs a program, as notepad.exe.":                 "start 模式需要填写要运行的程序，例如 notepad.exe。",
	"The width must be an even number from 200 to 7680.":     "宽度须为 200–7680 之间的偶数。",
	"The height must be a number from 200 to 4320.":          "高度须为 200–4320 之间的数字。",
	"The timeout must be a number of seconds from 5 to 600.": "超时须为 5–600 秒。",

	// What the window says while it works.
	"Running":                            "执行中",
	"Done":                               "完成",
	"Exit code":                          "退出码",
	"The privileged script is missing: ": "找不到特权脚本：",
	"Granting the privileges…":           "正在授予特权…",
	"Privileges are granted; sign out and in for them to take effect.": "已授予特权；注销并重新登录后生效。",
	"The cached password is cleared.":                                  "已清理缓存的密码。",
	"The shortcut is on the desktop.":                                  "已在桌面创建快捷方式。",
	"Cancel":                                                           "取消",
	"Save":                                                             "保存",
	"Language":                                                         "语言",
	"Sessions":                                                         "会话",
	"Nothing is open.":                                                 "没有打开的会话。",
	"A password is needed for that account.":                           "该账号需要密码。",
	"Log copied":                                                       "日志已复制。",
	"Logging off failed.":                                              "注销失败。",
	"Logging off…":                                                     "正在注销…",
	"Opening…":                                                         "正在打开…",
	"Password forgotten":                                               "已清理密码。",
	"Shortcut made":                                                    "已创建快捷方式。",
	"The privileges script was not found.":                             "找不到特权脚本。",
	"The script did not finish.":                                       "脚本未执行完成。",
	"The size is at least 320 by 200.":                                 "尺寸至少为 320 × 200。",
	"The timeout is at least 5 seconds.":                               "超时至少 5 秒。",
	"The user name cannot hold a quote.":                               "用户名不能包含引号。",
	"No sessions yet.":                                                 "还没有会话。",
	"No sessions are open.":                                            "没有打开的会话。",
	"Sign out this session":                                            "注销此会话",
	"Close":                                                            "关闭",
	"Focus":                                                            "切换",
	"Copy the log":                                                     "复制日志",
	"Clear the log":                                                    "清空日志",
	"Open the form":                                                    "打开表单",
	"Mode: ":                                                           "模式：",
	"desktop — the whole child desktop":                                "desktop —— 整个子桌面",
	"Close all sessions":                                               "关闭全部会话",
	"Close the chosen session":                                         "关闭选中的会话",
	"Version":                                                          "版本",
	"Local loopback Remote Desktop, in child sessions of this account.":                        "本机 loopback 远程桌面，以当前账号的子会话运行。",
	"Not a security boundary: the child session shares this account's files, tokens and apps.": "不是安全边界：子会话与此账号共享文件、令牌与已安装的程序。",
	"Sessions are kept in": "会话保存在",
	"The bridge connects this desktop's clipboard to the child session's.": "桥负责把本桌面的剪贴板接到子会话。",
	"No session is open.": "没有打开的会话。",
	"Bridge role: ":       "桥角色：",
	"Built":               "构建时间",
}

var (
	mu   sync.RWMutex
	lang = "en"
)

// Set picks the language: "zh" for Chinese, anything else English.
func Set(l string) {
	mu.Lock()
	defer mu.Unlock()
	if l == "zh" {
		lang = "zh"
		return
	}
	lang = "en"
}

// Lang is the language in use.
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

// T translates s into the language in use.
func T(s string) string {
	if s == "" {
		return s
	}
	mu.RLock()
	defer mu.RUnlock()
	if lang != "zh" {
		return s
	}
	if v, ok := zh[s]; ok {
		return v
	}
	return s
}

// Detect is the language the system asks for, used until the user picks
// one: Chinese on a Chinese Windows, English elsewhere.
func Detect() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(os.Getenv(k))
		if strings.HasPrefix(v, "zh") {
			return "zh"
		}
		if v != "" {
			return "en"
		}
	}
	return systemLang()
}
