//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func browserCandidates() []string {
	var out []string
	roots := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")}
	rels := []string{
		`Microsoft\Edge\Application\msedge.exe`, // preinstalled on Windows 10/11
		`Google\Chrome\Application\chrome.exe`,
		`BraveSoftware\Brave-Browser\Application\brave.exe`,
		`Chromium\Application\chrome.exe`,
	}
	for _, rel := range rels {
		for _, r := range roots {
			if r == "" {
				continue
			}
			p := filepath.Join(r, rel)
			if _, err := os.Stat(p); err == nil {
				out = append(out, p)
			}
		}
	}
	for _, n := range []string{"msedge.exe", "chrome.exe", "brave.exe"} {
		if p, err := exec.LookPath(n); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func openDefault(url string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() }
