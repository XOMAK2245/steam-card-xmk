// Package discord implements Discord Quest auto-completion with anti-detection measures.
package discord

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"runtime"
)

const (
	discordVersion  = "0.0.94"
	electronVersion = "33.3.1"
	chromeVersion   = "130.0.6723.191"
)

// UserAgent returns a realistic Discord desktop client User-Agent string.
func UserAgent() string {
	switch runtime.GOOS {
	case "windows":
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) discord/" +
			discordVersion + " Chrome/" + chromeVersion + " Electron/" + electronVersion + " Safari/537.36"
	default: // linux
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) discord/" +
			discordVersion + " Chrome/" + chromeVersion + " Electron/" + electronVersion + " Safari/537.36"
	}
}

// SuperProperties returns base64-encoded JSON mimicking the official Discord client.
func SuperProperties() string {
	osName := "Linux"
	osVer := "6.1.0"
	if runtime.GOOS == "windows" {
		osName = "Windows"
		osVer = "10.0.19045"
	}
	props := map[string]any{
		"os":              osName,
		"browser":         "Discord Client",
		"release_channel": "stable",
		"client_version":  discordVersion,
		"os_version":      osVer,
		"os_arch":         runtime.GOARCH,
		"system_locale":   "en-US",
	}
	b, _ := json.Marshal(props)
	return base64.StdEncoding.EncodeToString(b)
}

// DefaultHeaders returns HTTP headers matching the official Discord desktop client.
func DefaultHeaders(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", token)
	h.Set("User-Agent", UserAgent())
	h.Set("X-Super-Properties", SuperProperties())
	h.Set("Content-Type", "application/json")
	h.Set("Accept-Language", "en-US")
	h.Set("Origin", "https://discord.com")
	h.Set("Referer", "https://discord.com/channels/@me")
	return h
}

