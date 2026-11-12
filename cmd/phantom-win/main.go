//go:build windows

// Command phantom-win is the Windows phantom worker. It sets SteamAppId /
// SteamGameId, loads steam_api64.dll that lives next to it, calls
// SteamAPI_Init and pumps callbacks until stdin is closed by the parent (or
// the parent dies), then calls SteamAPI_Shutdown. Build with -H windowsgui so
// that no console window appears.
package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

func init() { runtime.LockOSThread() }

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	appid := os.Args[1]
	os.Setenv("SteamAppId", appid)
	os.Setenv("SteamGameId", appid)

	exe, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	dll, err := windows.LoadDLL(filepath.Join(filepath.Dir(exe), "steam_api64.dll"))
	if err != nil {
		os.Exit(1)
	}
	initFn, err1 := dll.FindProc("SteamAPI_Init")
	cbFn, err2 := dll.FindProc("SteamAPI_RunCallbacks")
	shutFn, err3 := dll.FindProc("SteamAPI_Shutdown")
	if err1 != nil || err2 != nil || err3 != nil {
		os.Exit(1)
	}
	if r, _, _ := initFn.Call(); r&0xff == 0 {
		os.Exit(2)
	}

	// The parent keeps our stdin open; EOF means "shut down" (also fires if the
	// parent process dies, as the pipe handle is closed by the OS).
	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, os.Stdin)
		close(done)
	}()

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-done:
			shutFn.Call()
			return
		case <-t.C:
			cbFn.Call()
		}
	}
}
