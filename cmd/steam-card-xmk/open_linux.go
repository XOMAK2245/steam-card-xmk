//go:build linux

package main

import "os/exec"

func browserCandidates() []string {
	var out []string
	for _, n := range []string{
		"chromium", "chromium-browser", "google-chrome-stable", "google-chrome",
		"brave", "brave-browser", "thorium-browser", "microsoft-edge-stable", "microsoft-edge", "vivaldi",
	} {
		if p, err := exec.LookPath(n); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func openDefault(url string) { exec.Command("xdg-open", url).Start() }
