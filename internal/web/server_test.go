package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/farm"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	cfg := config.Load()
	s := &Server{Eng: farm.New(cfg), Cfg: cfg, Quit: make(chan struct{})}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, url string) (string, int) {
	t.Helper()
	// httptest listens on 127.0.0.1, which passes the Host guard.
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String(), r.StatusCode
}

func TestLogsIsArrayNeverNull(t *testing.T) {
	ts := newTestServer(t)
	body, code := get(t, ts.URL+"/api/logs")
	if code != 200 || strings.TrimSpace(body) == "null" {
		t.Fatalf("code=%d body=%q", code, body)
	}
	var logs []string
	if err := json.Unmarshal([]byte(body), &logs); err != nil || logs == nil {
		t.Fatalf("not a JSON array: %q (%v)", body, err)
	}
}

func TestEngineLogsNonNil(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	e := farm.New(config.Load())
	if e.Logs() == nil {
		t.Fatal("Logs() returned nil")
	}
}

func TestStateValidJSON(t *testing.T) {
	ts := newTestServer(t)
	body, code := get(t, ts.URL+"/api/state")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	var st map[string]any
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatal(err)
	}
	if st["loggedIn"] != false {
		t.Errorf("expected loggedIn=false: %v", st["loggedIn"])
	}
	if _, ok := st["games"].([]any); !ok {
		t.Errorf("games must be an array: %v", st["games"])
	}
}

func TestIndexShowsLoginBox(t *testing.T) {
	ts := newTestServer(t)
	body, _ := get(t, ts.URL+"/")
	if !strings.Contains(body, `<section id="loginBox" class="card">`) {
		t.Error("loginBox must not be hidden by default")
	}
	if !strings.Contains(body, "btnLoginShow") {
		t.Error("missing sign-in button")
	}
}

func TestPostNeedsHeader(t *testing.T) {
	ts := newTestServer(t)
	r, _ := http.Post(ts.URL+"/api/stop", "application/json", nil)
	if r.StatusCode != 403 {
		t.Errorf("expected 403, got %d", r.StatusCode)
	}
}

func TestDiscordEndpointsInitial(t *testing.T) {
	ts := newTestServer(t)
	body, code := get(t, ts.URL+"/api/discord/state")
	if code != 200 {
		t.Fatalf("code=%d, want 200", code)
	}
	var st map[string]any
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if st["hasToken"] != false {
		t.Errorf("hasToken = %v, want false", st["hasToken"])
	}

	body, code = get(t, ts.URL+"/api/discord/quests")
	if code != 200 {
		t.Fatalf("code=%d, want 200", code)
	}
	if strings.TrimSpace(body) != "[]" {
		t.Errorf("quests = %q, want []", body)
	}
}

func post(t *testing.T, url string, body string) (string, int) {
	t.Helper()
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-XMK", "1")
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String(), r.StatusCode
}

func TestSteamAccountSwitchAndRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	cfg := config.Load()
	cfg.Update(func(c *config.Store) {
		c.AddOrUpdateSteamAccount(config.SteamAccount{
			SteamID:     "76561198000000001",
			PersonaName: "User1",
		}, true)
		c.AddOrUpdateSteamAccount(config.SteamAccount{
			SteamID:     "76561198000000002",
			PersonaName: "User2",
		}, false)
	})
	s := &Server{Eng: farm.New(cfg), Cfg: cfg, Quit: make(chan struct{})}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	// State check
	body, code := get(t, ts.URL+"/api/state")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	var st farm.State
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatal(err)
	}
	if st.SteamID != "76561198000000001" || st.PersonaName != "User1" {
		t.Fatalf("unexpected state: %+v", st)
	}
	if len(st.SteamAccounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(st.SteamAccounts))
	}

	// Switch to account 2
	_, code = post(t, ts.URL+"/api/steam/switch", `{"steamId":"76561198000000002"}`)
	if code != 200 {
		t.Fatalf("switch code=%d", code)
	}
	body, _ = get(t, ts.URL+"/api/state")
	json.Unmarshal([]byte(body), &st)
	if st.SteamID != "76561198000000002" || st.PersonaName != "User2" {
		t.Fatalf("expected switched to User2, got %+v", st)
	}

	// Remove account 1
	_, code = post(t, ts.URL+"/api/steam/remove", `{"steamId":"76561198000000001"}`)
	if code != 200 {
		t.Fatalf("remove code=%d", code)
	}
	body, _ = get(t, ts.URL+"/api/state")
	json.Unmarshal([]byte(body), &st)
	if len(st.SteamAccounts) != 1 || st.SteamAccounts[0].SteamID != "76561198000000002" {
		t.Fatalf("expected 1 account remaining, got %+v", st.SteamAccounts)
	}
}

func TestDiscordAccountSwitchAndRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	cfg := config.Load()
	cfg.Update(func(c *config.Store) {
		c.AddOrUpdateDiscordAccount(config.DiscordAccount{
			ID:         "111111",
			Username:   "disc1",
			GlobalName: "GamerOne",
			Token:      "tok1",
		}, true)
		c.AddOrUpdateDiscordAccount(config.DiscordAccount{
			ID:         "222222",
			Username:   "disc2",
			GlobalName: "GamerTwo",
			Token:      "tok2",
		}, false)
	})
	s := &Server{Eng: farm.New(cfg), Cfg: cfg, Quit: make(chan struct{})}
	s.InitDiscord()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	body, code := get(t, ts.URL+"/api/discord/state")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	var dst struct {
		HasToken bool                     `json:"hasToken"`
		ActiveID string                   `json:"activeId"`
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(body), &dst); err != nil {
		t.Fatal(err)
	}
	if !dst.HasToken || dst.ActiveID != "111111" || len(dst.Accounts) != 2 {
		t.Fatalf("unexpected discord state: %+v", dst)
	}

	// Switch discord account
	_, code = post(t, ts.URL+"/api/discord/switch", `{"id":"222222"}`)
	if code != 200 {
		t.Fatalf("switch code=%d", code)
	}
	body, _ = get(t, ts.URL+"/api/discord/state")
	json.Unmarshal([]byte(body), &dst)
	if dst.ActiveID != "222222" {
		t.Fatalf("expected activeId 222222, got %s", dst.ActiveID)
	}

	// Remove discord account
	_, code = post(t, ts.URL+"/api/discord/remove", `{"id":"111111"}`)
	if code != 200 {
		t.Fatalf("remove code=%d", code)
	}
	body, _ = get(t, ts.URL+"/api/discord/state")
	json.Unmarshal([]byte(body), &dst)
	if len(dst.Accounts) != 1 || dst.Accounts[0]["id"] != "222222" {
		t.Fatalf("expected 1 discord account, got %+v", dst.Accounts)
	}
}
