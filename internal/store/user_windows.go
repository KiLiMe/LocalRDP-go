package store

import (
	"os"
	"strings"
)

// CurrentUser is the account Windows runs this process as, as the host
// window names it: DOMAIN\user.
func CurrentUser() string {
	name := strings.TrimSpace(os.Getenv("USERNAME"))
	domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	if name == "" {
		return ""
	}
	if domain == "" {
		return name
	}
	return domain + `\` + name
}
