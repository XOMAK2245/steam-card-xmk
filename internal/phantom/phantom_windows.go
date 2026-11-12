//go:build windows

package phantom

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var requiredFiles = []string{"steam_api64.dll"}

// workerCmd re-launches this very executable in hidden worker mode. The AppID
// is passed in the child's environment from the start (so the C runtime of
// steam_api64.dll inherits it) and also via steam_appid.txt in a per-app
// working directory, which Steam checks as well.
func workerCmd(dir string, appID uint32) *exec.Cmd {
	id := strconv.FormatUint(uint64(appID), 10)
	appDir := filepath.Join(dir, "apps", id)
	os.MkdirAll(appDir, 0o755)
	os.WriteFile(filepath.Join(appDir, "steam_appid.txt"), []byte(id), 0o644)

	exe, _ := os.Executable()
	cmd := exec.Command(exe, "--phantom-worker", id)
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(),
		"SteamAppId="+id,
		"SteamGameId="+id,
		"STEAM_CARD_XMK_WORKER_DIR="+dir,
	)
	return cmd
}

var (
	jobOnce sync.Once
	job     windows.Handle
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// afterStart puts the worker into a kill-on-close job object so that workers
// never outlive this application, whatever way it exits.
func afterStart(cmd *exec.Cmd) error {
	var err error
	jobOnce.Do(func() {
		job, err = windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	})
	if err != nil || job == 0 {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

// The worker exits gracefully (SteamAPI_Shutdown) when its stdin is closed.
func terminate(cmd *exec.Cmd, stdin io.Closer) { stdin.Close() }

func SteamRunning() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam\ActiveProcess`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	pid, _, err := k.GetIntegerValue("pid")
	return err == nil && pid != 0
}
