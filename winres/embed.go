// Package winres holds the pictures that end up in the exe. genicon writes
// them; mygo bakes localrdp.png in as the icon group, and the session variant
// travels inside the binary as a byte slice because an exe carries only one
// icon group.
package winres

import _ "embed"

// SessionIcon is the child window's icon: the same folder, plus an amber dot
// in the corner, so the two taskbar buttons are told apart at a glance.
//
//go:embed localrdp-session.png
var SessionIcon []byte
