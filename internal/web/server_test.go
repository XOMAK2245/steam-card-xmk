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
