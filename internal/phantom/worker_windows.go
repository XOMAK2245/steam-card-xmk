//go:build windows

package phantom

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

// RunWorker is the Windows phantom worker (the main executable started with
// --phantom-worker <appid>). It sets SteamAppId/SteamGameId, loads
// steam_api64.dll, calls SteamAPI_Init and pumps callbacks until the parent
// closes our stdin (or dies), then calls SteamAPI_Shutdown.
func RunWorker(appid string) int {
	runtime.LockOSThread() // Steamworks wants a stable thread
	os.Setenv("SteamAppId", appid)
	os.Setenv("SteamGameId", appid)

	dir := os.Getenv("STEAM_CARD_XMK_WORKER_DIR")
	if dir == "" {
		exe, _ := os.Executable()
		dir = filepath.Dir(exe)
	}
	dll, err := windows.LoadDLL(filepath.Join(dir, "steam_api64.dll"))
	if err != nil {
		return 1
	}
	initFn, e1 := dll.FindProc("SteamAPI_Init")
	cbFn, e2 := dll.FindProc("SteamAPI_RunCallbacks")
	shutFn, e3 := dll.FindProc("SteamAPI_Shutdown")
	if e1 != nil || e2 != nil || e3 != nil {
		return 1
	}
	if r, _, _ := initFn.Call(); r&0xff == 0 {
		return 2
	}

	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, os.Stdin) // EOF == shut down (also if the parent dies)
		close(done)
	}()
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-done:
			shutFn.Call()
			return 0
		case <-t.C:
			cbFn.Call()
		}
	}
}
