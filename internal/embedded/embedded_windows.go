//go:build windows

package embedded

import "embed"

//go:embed bin/windows
var files embed.FS

const sub = "bin/windows"
