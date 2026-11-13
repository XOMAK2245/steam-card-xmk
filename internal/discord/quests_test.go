package discord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestFilterQuests(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour).Format(time.RFC3339)
	future := now.Add(24 * time.Hour).Format(time.RFC3339)

	all := []Quest{
		{ID: "active1", Config: QuestConfig{ExpiresAt: future}},
		{ID: "expired1", Config: QuestConfig{ExpiresAt: past}},
		{ID: "claimed1", Config: QuestConfig{ExpiresAt: future},
			UserStatus: &QuestUserStatus{ClaimedAt: "2026-01-01T00:00:00Z"}},
		{ID: "active2", Config: QuestConfig{ExpiresAt: ""}}, // no expiry = always active
	}
	// Populate IsClaimed
	for i := range all {
		all[i].PopulateMetrics()
	}

	active, claimed := filterQuests(all)

	if len(active) != 2 {
		t.Errorf("expected 2 active quests, got %d: %+v", len(active), active)
	}
	if len(claimed) != 1 {
		t.Errorf("expected 1 claimed quest, got %d: %+v", len(claimed), claimed)
	}
	for _, q := range active {
		if q.ID == "expired1" {
			t.Errorf("expired quest should not be in active list")
		}
	}
	if claimed[0].ID != "claimed1" {
		t.Errorf("expected claimed1, got %s", claimed[0].ID)
	}
}

func TestHeartbeatInterval(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := HeartbeatInterval()
		if d < 32*time.Second || d > 36*time.Second {
			t.Fatalf("HeartbeatInterval() = %v, want [32s, 36s]", d)
		}
	}
}

func TestRewardParsing(t *testing.T) {
	jsonBlob := `{
		"quests": [
			{
				"id": "q1",
				"config": {
					"messages": {"quest_name": "Game 1"},
					"reward_config": {
						"messages": {"reward_name": "840 Orbs"}
					}
				}
			},
			{
				"id": "q2",
				"config": {
					"messages": {"quest_name": "Game 2"},
					"reward_config": {
						"rewards": [{"messages": {"name": "Avatar Decoration"}}]
					}
				}
			},
			{
				"id": "q3",
				"config": {
					"messages": {"quest_name": "Game 3"},
					"reward_config": {
						"orbs_reward": {"amount": 500}
					}
				}
			}
		]
	}`

	qs, err := ParseQuests([]byte(jsonBlob))
	if err != nil {
		t.Fatalf("ParseQuests failed: %v", err)
	}
	if len(qs) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(qs))
	}
	if qs[0].RewardName != "840 Orbs" {
		t.Errorf("q1 RewardName = %q, want 840 Orbs", qs[0].RewardName)
	}
	if qs[1].RewardName != "Avatar Decoration" {
		t.Errorf("q2 RewardName = %q, want Avatar Decoration", qs[1].RewardName)
	}
	if qs[2].RewardName != "500 Orbs" {
		t.Errorf("q3 RewardName = %q, want 500 Orbs", qs[2].RewardName)
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

func TestRunQuestAlreadyCompletedNoClaim(t *testing.T) {
	c := NewClient("dummy", nil)
	q := Quest{
		ID: "q_done",
		UserStatus: &QuestUserStatus{
			EnrolledAt:            "2026-01-01T00:00:00Z",
			StreamProgressSeconds: 900,
		},
	}
	err := c.RunQuest(context.Background(), q, nil)
	if err != nil {
		t.Fatalf("expected nil error for completed quest without calling claim, got: %v", err)
	}
}

func TestEnrollPlatformDesktop(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/enroll") {
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(bodyBytes, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"enrolled":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldBase := apiBase
	apiBase = ts.URL + "/"
	defer func() { apiBase = oldBase }()

	c := NewClient("test_token", nil)
	if err := c.Enroll(context.Background(), "quest_test_123"); err != nil {
		t.Fatalf("Enroll failed: %v", err)
	}

	if gotBody["location"] != float64(1) {
		t.Errorf("expected location: 1, got: %v", gotBody["location"])
	}
	if gotBody["platform"] != float64(4) {
		t.Errorf("expected platform: 4 (PC / Desktop), got: %v", gotBody["platform"])
	}
}

func TestExtendedRewardParsing(t *testing.T) {
	jsonBlob := `{
		"quests": [
			{
				"id": "q_name",
				"config": {
					"messages": {
						"game_title": "Arknights: Endfield",
						"quest_name": "New Operator: Argent Flow"
					},
					"reward_config": {"name": "Special Skin Bundle"}
				}
			},
			{
				"id": "q_reward_list",
				"config": {
					"messages": {"quest_name": "Game Reward List"},
					"reward_config": {
						"reward_list": [{"name": "Ghost Profile Effect"}]
					}
				}
			},
			{
				"id": "q_regex_orbs_ru",
				"config": {
					"messages": {
						"game_title": "Arknights: Endfield",
						"quest_name": "New Operator: Argent Flow"
					},
					"reward_config": {
						"messages": {"completed": "Получить награду «700 Orbs»"}
					}
				}
			},
			{
				"id": "q_regex_kw",
				"config": {
					"messages": {"quest_name": "Game Regex Keyword"},
					"reward_config": {
						"messages": {"completed": "Earn Cyber Weapon Blueprint"}
					}
				}
			}
		]
	}`

	qs, err := ParseQuests([]byte(jsonBlob))
	if err != nil {
		t.Fatalf("ParseQuests failed: %v", err)
	}
	if len(qs) != 4 {
		t.Fatalf("expected 4 quests, got %d", len(qs))
	}
	if qs[0].RewardName != "Special Skin Bundle" {
		t.Errorf("q_name got %q, want 'Special Skin Bundle'", qs[0].RewardName)
	}
	if qs[0].GameTitle != "Arknights: Endfield" {
		t.Errorf("q_name GameTitle got %q, want 'Arknights: Endfield'", qs[0].GameTitle)
	}
	if qs[0].Name() != "Arknights: Endfield" {
		t.Errorf("q_name Name() got %q, want 'Arknights: Endfield'", qs[0].Name())
	}
	if qs[1].RewardName != "Ghost Profile Effect" {
		t.Errorf("q_reward_list got %q, want 'Ghost Profile Effect'", qs[1].RewardName)
	}
	if qs[2].RewardName != "700 Orbs" {
		t.Errorf("q_regex_orbs_ru got %q, want '700 Orbs'", qs[2].RewardName)
	}
	if qs[3].RewardName != "Cyber Weapon Blueprint" {
		t.Errorf("q_regex_kw got %q, want 'Cyber Weapon Blueprint'", qs[3].RewardName)
	}
}

func TestVideoQuestDetectionExtended(t *testing.T) {
	jsonBlob := `{
		"quests": [
			{
				"id": "v1",
				"config": {
					"messages": {"quest_name": "GTA VI Trailer"},
					"task_config_v2": {
						"tasks": {
							"WATCH_VIDEO_DESKTOP": {"target": 60}
						}
					}
				}
			},
			{
				"id": "v2",
				"config": {
					"messages": {"quest_name": "Monster Hunter Trailer"},
					"task_config_v2": {
						"tasks": {
							"WATCH_VIDEO_ON_MOBILE": {"target_seconds": 30}
						}
					}
				}
			},
			{
				"id": "v3",
				"config": {
					"messages": {"quest_name": "Anno 117 Preview"},
					"video_metadata": {"duration": 45}
				}
			}
		]
	}`

	qs, err := ParseQuests([]byte(jsonBlob))
	if err != nil {
		t.Fatalf("ParseQuests failed: %v", err)
	}
	if len(qs) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(qs))
	}

	if !qs[0].IsVideo || qs[0].SecondsTarget != 60 {
		t.Errorf("v1: IsVideo=%v, SecondsTarget=%v, want true/60", qs[0].IsVideo, qs[0].SecondsTarget)
	}
	if !qs[1].IsVideo || qs[1].SecondsTarget != 30 {
		t.Errorf("v2: IsVideo=%v, SecondsTarget=%v, want true/30", qs[1].IsVideo, qs[1].SecondsTarget)
	}
	if !qs[2].IsVideo || qs[2].SecondsTarget <= 0 {
		t.Errorf("v3: IsVideo=%v, SecondsTarget=%v, want true/30+", qs[2].IsVideo, qs[2].SecondsTarget)
	}
}

func TestFilterQuestsInvalidOrGraceDate(t *testing.T) {
	quests := []Quest{
		{
			ID: "valid_invalid_date_format",
			Config: QuestConfig{
				ExpiresAt: "not-a-valid-date-string",
			},
		},
		{
			ID: "still_active_near_expiry",
			Config: QuestConfig{
				ExpiresAt: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
			},
		},
	}

	active, claimed := filterQuests(quests)
	if len(active) != 2 {
		t.Errorf("expected 2 active quests (neither dropped), got %d (active=%+v, claimed=%+v)", len(active), active, claimed)
	}
}
