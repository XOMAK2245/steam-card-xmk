//go:build linux

package embedded

import "embed"

// steam_phantom is compiled from worker/steam_phantom.c by `make worker-linux`
// before building; libsteam_api.so is committed.
//
//go:embed bin/linux
var files embed.FS

const sub = "bin/linux"
