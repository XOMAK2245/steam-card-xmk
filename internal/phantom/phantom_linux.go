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

var requiredFiles = []string{"steam_phantom", "libsteam_api.so"}

func workerCmd(dir string, appID uint32) *exec.Cmd {
	cmd := exec.Command(filepath.Join(dir, "steam_phantom"), strconv.FormatUint(uint64(appID), 10))
	cmd.Dir = dir
	return cmd
}

// RunWorker is only needed on Windows, where the worker is the main executable.
func RunWorker(appID string) int { return 2 }

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
