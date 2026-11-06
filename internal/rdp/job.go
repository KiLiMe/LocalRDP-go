package rdp

import (
	"os"
	"strings"
	"time"
)

const jobHeader = "LOCALRDP-JOB-1"

func writeJobFile(opt StartOptions) string {
	name := sprintf("lrdp-%d-%d.job", currentProcessID(), time.Now().UnixMilli()&0xFFFFFFFF)
	path := tempDir() + "\\" + name
	var b strings.Builder
	b.WriteString(jobHeader + "\n")
	b.WriteString("workdir=" + opt.WorkDir + "\n")
	b.WriteString("command=" + opt.Command + "\n")
	if opt.KeepSess {
		b.WriteString("keep_session=1\n")
	} else {
		b.WriteString("keep_session=0\n")
	}
	if opt.Desktop {
		b.WriteString("desktop=1\n")
	} else {
		b.WriteString("desktop=0\n")
	}
	if !writeUTF8File(path, b.String()) {
		logError("Failed to write job file " + path)
		return ""
	}
	return path
}

func readJobFile(path string, opt *StartOptions) bool {
	raw, ok := readUTF8File(path)
	if !ok {
		return false
	}
	header := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !header {
			if line != jobHeader {
				return false
			}
			header = true
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key, val := line[:eq], line[eq+1:]
		switch key {
		case "workdir":
			opt.WorkDir = val
		case "command":
			opt.Command = val
		case "keep_session":
			opt.KeepSess = val == "1"
		case "desktop":
			opt.Desktop = val == "1"
		}
	}
	return header
}

func deleteJobFile(path string) {
	if path != "" {
		os.Remove(path)
	}
}
