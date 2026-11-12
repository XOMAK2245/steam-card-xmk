//go:build windows

package main

import "os/exec"

func openBrowser(url string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() }
