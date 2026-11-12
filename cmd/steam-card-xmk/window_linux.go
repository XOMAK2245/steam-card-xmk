//go:build linux

package main

// A CGO-free native window is not available on Linux (a GTK WebView would
// break the single static binary), so the UI uses a Chromium --app window.
func runNative(url string, quit <-chan struct{}) bool { return false }

func showExisting(url string) { openApp(url, nil) }
