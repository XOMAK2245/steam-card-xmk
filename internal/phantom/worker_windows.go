//go:build windows

package phantom

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ESteamAPIInitResult values returned by SteamAPI_InitFlat.
var initResults = map[int32]string{
	1: "failed (generic)",
	2: "no Steam client running (start Steam and log in)",
	3: "version mismatch (Steam client is older than steam_api64.dll)",
}

func fail(code int, format string, a ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	return code
}

// putCRTEnv mirrors a variable into the C runtime environment: Go's os.Setenv
// only calls SetEnvironmentVariableW, while steam_api64.dll reads getenv().
func putCRTEnv(k, v string) {
	p := windows.NewLazySystemDLL("ucrtbase.dll").NewProc("_putenv_s")
	if p.Find() != nil {
		return
	}
	kk, _ := windows.BytePtrFromString(k)
	vv, _ := windows.BytePtrFromString(v)
	p.Call(uintptr(unsafe.Pointer(kk)), uintptr(unsafe.Pointer(vv)))
}

// RunWorker is the Windows phantom worker (this executable started with
// --phantom-worker <appid>). It makes the Steam client believe the game is
// running: it loads steam_api64.dll, calls SteamAPI_InitFlat (the modern
// Steamworks SDK has no SteamAPI_Init export), pumps callbacks and, once the
// parent closes our stdin (or dies), calls SteamAPI_Shutdown.
func RunWorker(appid string) int {
	runtime.LockOSThread() // Steamworks wants a stable thread

	// The parent already passes these in the environment; set them again for safety.
	os.Setenv("SteamAppId", appid)
	os.Setenv("SteamGameId", appid)
	putCRTEnv("SteamAppId", appid)
	putCRTEnv("SteamGameId", appid)

	dir := os.Getenv("STEAM_CARD_XMK_WORKER_DIR")
	if dir == "" {
		exe, _ := os.Executable()
		dir = filepath.Dir(exe)
	}
	dllPath := filepath.Join(dir, "steam_api64.dll")
	dll, err := windows.LoadDLL(dllPath)
	if err != nil {
		return fail(1, "cannot load %s: %v", dllPath, err)
	}

	runCb, err := dll.FindProc("SteamAPI_RunCallbacks")
	if err != nil {
		return fail(1, "steam_api64.dll: %v", err)
	}
	shutdown, err := dll.FindProc("SteamAPI_Shutdown")
	if err != nil {
		return fail(1, "steam_api64.dll: %v", err)
	}

	ok := false
	if flat, err := dll.FindProc("SteamAPI_InitFlat"); err == nil {
		var msg [1024]byte
		r, _, _ := flat.Call(uintptr(unsafe.Pointer(&msg[0])))
		if code := int32(uint32(r)); code == 0 { // k_ESteamAPIInitResult_OK
			ok = true
		} else {
			text := windows.ByteSliceToString(msg[:])
			desc := initResults[code]
			if desc == "" {
				desc = "unknown error"
			}
			return fail(2, "SteamAPI_InitFlat failed for AppID %s: code %d, %s: %s", appid, code, desc, text)
		}
	} else {
		// Older SDKs: SteamAPI_Init / SteamAPI_InitSafe return a C++ bool.
		var tried []string
		for _, name := range []string{"SteamAPI_InitSafe", "SteamAPI_Init"} {
			p, err := dll.FindProc(name)
			if err != nil {
				tried = append(tried, name+": missing")
				continue
			}
			if r, _, _ := p.Call(); r&0xff != 0 {
				ok = true
				break
			}
			return fail(2, "%s() returned false for AppID %s (is Steam running and the game owned?)", name, appid)
		}
		if !ok {
			return fail(1, "steam_api64.dll has no usable init function (%v)", tried)
		}
	}

	fmt.Fprintf(os.Stderr, "Phantom running for AppID %s\n", appid)

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
			shutdown.Call()
			return 0
		case <-t.C:
			runCb.Call()
		}
	}
}
