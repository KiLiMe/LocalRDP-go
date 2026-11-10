//go:build !windows

package app

// Run opens the window, which needs Windows: the child session it shows
// is one of Windows.
func Run(opts Options) int {
	stderrf("%s\n", "localrdp: the window needs Windows")
	return 1
}
