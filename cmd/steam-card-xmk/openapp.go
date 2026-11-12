package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var appFlags = []string{"--window-size=1100,760", "--no-first-run", "--no-default-browser-check"}

// openApp shows the UI in a standalone app window (Chromium-family browser in
// --app mode, no tabs or address bar). If the window is closed after having
// been open for a while, onClosed is called so the application can exit.
// Falls back to the default browser if no suitable browser is found.
func openApp(url string, onClosed func()) {
	profile, err := os.UserCacheDir()
	if err != nil {
		profile = os.TempDir()
	}
	profile = filepath.Join(profile, "steam-card-xmk", "browser-profile")
	for _, bin := range browserCandidates() {
		args := append([]string{"--app=" + url, "--user-data-dir=" + profile}, appFlags...)
		cmd := exec.Command(bin, args...)
		if err := cmd.Start(); err != nil {
			continue
		}
		started := time.Now()
		go func() {
			cmd.Wait()
			// A quick exit means the browser handed off to another instance
			// or failed; only a long-lived window counts as "user closed it".
			if time.Since(started) > 5*time.Second && onClosed != nil {
				onClosed()
			}
		}()
		return
	}
	openDefault(url)
}
