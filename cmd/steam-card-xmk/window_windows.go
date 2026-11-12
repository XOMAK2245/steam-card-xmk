//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
)

const windowTitle = "XMK Suite"

func init() { runtime.LockOSThread() } // the Win32 message loop must stay on the main thread

// runNative shows the UI in a native window (WebView2, created inside this very
// process) and blocks until the user closes it or quit is closed. It returns
// false if the WebView2 runtime is unavailable.
func runNative(url string, quit <-chan struct{}) bool {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(cache, "steam-card-xmk", "webview2"),
		WindowOptions: webview2.WindowOptions{
			Title: windowTitle, Width: 1100, Height: 760, Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.SetSize(1100, 760, webview2.HintNone)
	w.Navigate(url)
	go func() {
		<-quit
		w.Dispatch(w.Terminate)
	}()
	w.Run()
	return true
}

// showExisting brings the window of an already running instance to front.
func showExisting(url string) {
	u32 := syscall.NewLazyDLL("user32.dll")
	title, _ := syscall.UTF16PtrFromString(windowTitle)
	hwnd, _, _ := u32.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		openApp(url, nil)
		return
	}
	u32.NewProc("ShowWindow").Call(hwnd, 9) // SW_RESTORE
	u32.NewProc("SetForegroundWindow").Call(hwnd)
}
