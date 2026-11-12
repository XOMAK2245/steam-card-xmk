package discord

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUserAgent(t *testing.T) {
	ua := UserAgent()
	for _, want := range []string{"discord/", "Electron/", "Chrome/"} {
		if !strings.Contains(ua, want) {
			t.Errorf("UserAgent() = %q, missing %q", ua, want)
		}
	}
}

func TestSuperProperties(t *testing.T) {
	sp := SuperProperties()
	raw, err := base64.StdEncoding.DecodeString(sp)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	for _, key := range []string{"os", "browser", "release_channel", "client_version", "os_version", "os_arch", "system_locale"} {
		if _, ok := m[key]; !ok {
			t.Errorf("SuperProperties missing key %q", key)
		}
	}
	if m["browser"] != "Discord Client" {
		t.Errorf("browser = %v, want Discord Client", m["browser"])
	}
	if m["release_channel"] != "stable" {
		t.Errorf("release_channel = %v, want stable", m["release_channel"])
	}
}

func TestDefaultHeaders(t *testing.T) {
	h := DefaultHeaders("test-token-123")
	checks := map[string]string{
		"Authorization":    "test-token-123",
		"Content-Type":     "application/json",
		"Accept-Language":  "en-US",
		"Origin":           "https://discord.com",
		"Referer":          "https://discord.com/channels/@me",
	}
	for k, want := range checks {
		if got := h.Get(k); got != want {
			t.Errorf("header %q = %q, want %q", k, got, want)
		}
	}
	if h.Get("User-Agent") == "" {
		t.Error("User-Agent header is empty")
	}
	if h.Get("X-Super-Properties") == "" {
		t.Error("X-Super-Properties header is empty")
	}
}

func TestHeartbeatInterval(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := HeartbeatInterval()
		if d < 27*time.Second || d > 33*time.Second {
			t.Fatalf("HeartbeatInterval() = %v, want [27s, 33s]", d)
		}
	}
}

func TestQuestIsStream(t *testing.T) {
	tests := []struct {
		taskType string
		want     bool
	}{
		{"PLAY_ON_DESKTOP", false},
		{"STREAM_ON_DESKTOP", true},
		{"WATCH_VIDEO", true},
	}
	for _, tt := range tests {
		q := Quest{Config: QuestConfig{TaskConfig: QuestTaskConfig{TaskType: tt.taskType}}}
		if got := q.IsStream(); got != tt.want {
			t.Errorf("IsStream() for %q = %v, want %v", tt.taskType, got, tt.want)
		}
	}
}

