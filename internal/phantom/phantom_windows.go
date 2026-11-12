//go:build windows

package phantom

import (
	"io"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	workerName = "phantom.exe"
	libName    = "steam_api64.dll"
)

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
