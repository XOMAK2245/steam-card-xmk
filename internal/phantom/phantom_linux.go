//go:build linux

package phantom

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	workerName = "steam_phantom"
	libName    = "libsteam_api.so"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func afterStart(cmd *exec.Cmd) error { return nil }

func terminate(cmd *exec.Cmd, stdin io.Closer) {
	stdin.Close()
	cmd.Process.Signal(syscall.SIGTERM)
}

// SteamRunning reports whether a local Steam client process is alive.
func SteamRunning() bool {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".steam", "steam.pid"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.steam/steam.pid"),
	} {
		if b, err := os.ReadFile(p); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); err == nil {
					return true
				}
			}
		}
	}
	ents, _ := os.ReadDir("/proc")
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		if b, err := os.ReadFile("/proc/" + e.Name() + "/comm"); err == nil && strings.TrimSpace(string(b)) == "steam" {
			return true
		}
	}
	return false
}
