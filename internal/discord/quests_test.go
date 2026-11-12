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
		"Authorization":   "test-token-123",
		"Content-Type":    "application/json",
		"Accept-Language": "en-US",
		"Origin":          "https://discord.com",
		"Referer":         "https://discord.com/channels/@me",
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

func TestParseQuestsWithRealProgress(t *testing.T) {
	// Real-world scenario from Discord API with task_config_v2 and progress map (e.g. 52% on Basketball Zero)
	realJSON := `{
		"quests": [
			{
				"id": "1306354807490056263",
				"config": {
					"messages": {"quest_name": "Basketball Zero"},
					"task_config_v2": {
						"tasks": {
							"PLAY_ON_DESKTOP": {
								"target": 900
							}
						}
					}
				},
				"user_status": {
					"enrolled_at": "2026-11-12T20:00:00Z",
					"progress": {
						"PLAY_ON_DESKTOP": {
							"value": 468.0
						}
					}
				}
			},
			{
				"id": "1306354807490056264",
				"config": {
					"messages": {"quest_name": "KOF: Legend"},
					"task_config_v2": {
						"tasks": {
							"WATCH_VIDEO": {
								"target": 30
							}
						}
					}
				},
				"user_status": null
			}
		]
	}`

	qs, err := ParseQuests([]byte(realJSON))
	if err != nil {
		t.Fatalf("ParseQuests error: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("expected 2 quests, got %d", len(qs))
	}

	// First quest: Basketball Zero
	q1 := qs[0]
	if q1.Name() != "Basketball Zero" {
		t.Errorf("q1 name = %q, want Basketball Zero", q1.Name())
	}
	if q1.SecondsTarget != 900 {
		t.Errorf("q1 SecondsTarget = %f, want 900", q1.SecondsTarget)
	}
	if q1.SecondsDone != 468 {
		t.Errorf("q1 SecondsDone = %f, want 468", q1.SecondsDone)
	}
	if q1.ProgressRatio != 0.52 {
		t.Errorf("q1 ProgressRatio = %f, want 0.52", q1.ProgressRatio)
	}
	if q1.IsVideo {
		t.Errorf("q1 should not be video")
	}

	// Second quest: KOF: Legend (Video quest, unenrolled)
	q2 := qs[1]
	if q2.Name() != "KOF: Legend" {
		t.Errorf("q2 name = %q, want KOF: Legend", q2.Name())
	}
	if q2.SecondsTarget != 30 {
		t.Errorf("q2 SecondsTarget = %f, want 30", q2.SecondsTarget)
	}
	if q2.SecondsDone != 0 {
		t.Errorf("q2 SecondsDone = %f, want 0", q2.SecondsDone)
	}
	if !q2.IsVideo {
		t.Errorf("q2 should be video")
	}
}

func TestParseQuests(t *testing.T) {
	// 1. Standard Discord object format
	objJSON := `{"quests": [{"id": "q1", "config": {"messages": {"quest_name": "Discord Quest 1"}}}]}`
	qs, err := ParseQuests([]byte(objJSON))
	if err != nil {
		t.Fatalf("ParseQuests obj error: %v", err)
	}
	if len(qs) != 1 || qs[0].ID != "q1" || qs[0].Name() != "Discord Quest 1" {
		t.Fatalf("unexpected parsed quests from obj: %+v", qs)
	}

	// 2. Empty Discord object format
	emptyObjJSON := `{"quests": []}`
	qsEmpty, err := ParseQuests([]byte(emptyObjJSON))
	if err != nil {
		t.Fatalf("ParseQuests empty obj error: %v", err)
	}
	if qsEmpty == nil || len(qsEmpty) != 0 {
		t.Fatalf("expected empty non-nil slice, got: %+v", qsEmpty)
	}

	// 3. Fallback raw array format
	arrJSON := `[{"id": "q2", "config": {"messages": {"quest_name": "Discord Quest 2"}}}]`
	qsArr, err := ParseQuests([]byte(arrJSON))
	if err != nil {
		t.Fatalf("ParseQuests arr error: %v", err)
	}
	if len(qsArr) != 1 || qsArr[0].ID != "q2" {
		t.Fatalf("unexpected parsed quests from arr: %+v", qsArr)
	}

	// 4. Empty raw array
	emptyArrJSON := `[]`
	qsArrEmpty, err := ParseQuests([]byte(emptyArrJSON))
	if err != nil {
		t.Fatalf("ParseQuests empty arr error: %v", err)
	}
	if qsArrEmpty == nil || len(qsArrEmpty) != 0 {
		t.Fatalf("expected empty non-nil slice, got: %+v", qsArrEmpty)
	}
}
