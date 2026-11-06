//go:build !windows

package store

import "os"

// CurrentUser is the account the process runs as.
func CurrentUser() string { return os.Getenv("USER") }
