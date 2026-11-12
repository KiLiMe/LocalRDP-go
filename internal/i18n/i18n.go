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
	// The band.
	"New session…":        "新会话…",
	"Back to the session": "回到会话",
	"Settings":            "设置",
	"Host window":         "宿主窗口",
	"New session":         "新会话",

	// The console: the session form and what it does.
	"Account":   "账号",
	"User name": "用户名",
	"Password":  "密码",
	"Password is cached. Type a new one to replace it.": "已缓存密码，输入新密码则替换。",
	"Keep the password": "缓存密码",
	"Keep the password between runs, encrypted for this user.": "运行之间记住密码（按当前用户加密）。",
	"Title":                   "标题",
	"Resolution":              "分辨率",
	"Already a preset":        "已锁定预设",
	"Editing custom size":     "自定义编辑中",
	"Custom resolution":       "自定义分辨率",
	"Width":                   "宽",
	"Height":                  "高",
	"Session":                 "会话",
	"Session settings":        "会话设置",
	"Running state":           "运行状态",
	"Host window and display": "宿主窗口与显示连接设置",
	"Window and connection":   "窗口状态与连接选项",
	"Run a program":           "启动程序",
	"Program and arguments, as a command line":                                            "程序与参数，按命令行写法，例如 notepad.exe C:\\a.txt",
	"Starts in the child session once it connects. Leave it empty for the desktop alone.": "连上后在这个子会话里运行；留空则只显示桌面。",
	"Options":       "选项",
	"Window size":   "窗口大小",
	"Fixed size":    "固定大小",
	"Follow window": "跟随窗口",
	"Scale":         "缩放",
	"Stretch":       "拉伸",
	"Fixed: the window and the desktop stay at the width and height below (a screen that cannot hold that size cuts them down).":                         "固定：窗口与桌面恒为下面的宽×高，不能拖动；屏幕放不下时按工作区截断。",
	"Follow: the desktop takes the size of the window, so the window can be dragged and text shrinks and grows with it.":                                 "跟随：桌面取窗口的尺寸，窗口可以拖动，字随之变小变大。",
	"Stretch: the desktop keeps its resolution; a smaller window shrinks the whole picture to fit, a bigger window leaves it at its size in the middle.": "拉伸：桌面保持自己的分辨率；窗口比桌面小时整幅画面缩小铺满，比桌面大时保持原尺寸居中（不放大）。",
	"Scale percent from 100 to 500, empty for none: it sizes the desktop, and the picture is fitted into the window, never stretched past its size.":     "缩放百分比 100–500，留空即不设（默认空）；它决定桌面按多大像素排版，窗口只把画面缩到放得下，不会放大。",
	"The scale is between 100 and 500 percent, or empty.":                                                                                                "缩放要填 100–500 之间的百分比，或者留空。",
	"Full screen":               "全屏",
	"Keep session":              "保留会话",
	"Minimized":                 "最小化",
	"No inherited scaling":      "禁用缩放自动继承",
	"No inherited scaling hint": "缩放留空时子会话跟着屏幕的缩放（150% 等）排版；勾上则固定按 100%。",
	"Timeout":                   "超时",
	"Second":                    "秒",
	"Open child session":        "打开子会话",
	"Status":                    "状态",
	"Log off the child session": "注销子会话",
	"Forget password":           "清理密码",
	"Open the data folder":      "打开数据目录",

	// The host window's own bar.
	"Reconnect":    "重连",
	"Disconnect":   "断开",
	"Connected":    "已连接",
	"Disconnected": "已断开",
	"Connecting…":  "正在连接…",

	// The tab of the session the window opened.
	"Program":       "程序",
	"Open":          "运行中",
	"Done":          "完成",
	"Exit code":     "退出码",
	"Show session":  "显示窗口",
	"Close session": "关闭会话",
	"Refresh":       "刷新",
	"The session runs on this computer, in a child session of this account.": "会话在本机这个账号的子会话里运行。",

	// The panel: the environment and the log.
	"Environment":     "环境",
	"Child session":   "子会话",
	"Cached password": "密码缓存",
	"Saved":           "已保存",
	"Saved encrypted": "已加密保存",
	"Expand":          "展开",
	"Collapse":        "收起",
	"Not saved":       "未保存",
	"None":            "无",
	"Log":             "日志",
	"Copy":            "复制",
	"Clear":           "清空",
	"No log yet.":     "还没有日志。",

	// What the app says while it works.
	"Running":                                "执行中",
	"Opening…":                               "正在打开…",
	"Logging off…":                           "正在注销…",
	"Logging off failed.":                    "注销失败。",
	"Log copied":                             "日志已复制。",
	"Password forgotten":                     "已清理密码。",
	"Cancel":                                 "取消",
	"Save":                                   "保存",
	"Language":                               "语言",
	"Keep passwords":                         "记住密码",
	"Remember the last session":              "记住上次的会话",
	"Settings hint":                          "只对本机的窗口生效，保存后立即应用。",
	"Keep passwords hint":                    "按当前 Windows 账户加密保存密码，下次自动填入。",
	"Remember hint":                          "下次启动时接着打开上次的会话与表单参数。",
	"A password is needed for that account.": "该账号需要密码。",
	"The user name cannot hold a quote.":     "用户名不能包含引号。",
	"The size is at least 320 by 200.":       "尺寸至少为 320 × 200。",
	"The timeout is at least 5 seconds.":     "超时至少 5 秒。",
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
