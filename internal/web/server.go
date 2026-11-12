// Package web serves the local control UI and JSON API.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/XOMAK2245/steam-card-xmk/internal/auth"
	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/discord"
	"github.com/XOMAK2245/steam-card-xmk/internal/farm"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	Eng    *farm.Engine
	Cfg    *config.Store
	QR     auth.QRLogin
	Quit   chan struct{}
	QuitFn func() // safe to call repeatedly; falls back to closing Quit

	// Discord quest support
	Discord       *discord.Client
	discordMu     sync.Mutex
	discordQuests map[string]context.CancelFunc // questID → cancel
}

// InitDiscord creates the Discord client if a token is configured.
func (s *Server) InitDiscord() {
	sess, _ := s.Cfg.Snapshot()
	_ = sess
	// Read discord config directly
	s.Cfg.Update(func(c *config.Store) {
		if c.Discord.Token != "" {
			s.Discord = discord.NewClient(c.Discord.Token, s.Eng.Logf)
		}
	})
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Eng.State()) })
	mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Eng.Logs()) })
	mux.HandleFunc("GET /api/events", s.events)

	act := func(name string, f func()) {
		mux.HandleFunc("POST /api/"+name, func(w http.ResponseWriter, r *http.Request) { f(); writeJSON(w, map[string]bool{"ok": true}) })
	}
	act("start", s.Eng.Start)
	act("pause", s.Eng.Pause)
	act("stop", s.Eng.Stop)
	act("refresh", func() { s.Eng.RefreshNow(); s.Eng.FetchOnce() })
	act("quit", func() {
		go func() {
			time.Sleep(300 * time.Millisecond) // let the HTTP response go out
			if s.QuitFn != nil {
				s.QuitFn()
			} else {
				close(s.Quit)
			}
		}()
	})
	act("logout", func() {
		s.Eng.Stop()
		s.Cfg.Update(func(c *config.Store) { c.Session = config.Session{} })
		s.Eng.Logf("Logged out")
	})

	mux.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var in config.Settings
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad json", 400)
			return
		}
		s.Cfg.Update(func(c *config.Store) {
			c.Settings.Strategy, c.Settings.MaxParallel = in.Strategy, in.MaxParallel
			c.Settings.PollMinutes = in.PollMinutes
		})
		s.Eng.Logf("Settings saved (strategy: %s, max parallel: %d)", in.Strategy, in.MaxParallel)
		s.Eng.Reconcile()
		writeJSON(w, s.Eng.State())
	})

	mux.HandleFunc("POST /api/login/qr", func(w http.ResponseWriter, r *http.Request) {
		err := s.QR.Start(func(sess config.Session) {
			s.Cfg.Update(func(c *config.Store) { c.Session = sess })
			s.Eng.Logf("Logged in as Steam ID %s", sess.SteamID)
			s.Eng.FetchOnce()
		})
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		writeJSON(w, s.QR.State())
	})
	mux.HandleFunc("GET /api/login/qr", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.QR.State()) })
	mux.HandleFunc("GET /api/login/qr.png", func(w http.ResponseWriter, r *http.Request) {
		st := s.QR.State()
		if st.URL == "" {
			http.NotFound(w, r)
			return
		}
		png, _ := qrcode.Encode(st.URL, qrcode.Medium, 320)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(png)
	})
	mux.HandleFunc("POST /api/login/cookies", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ LoginSecure, SessionID string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad json", 400)
			return
		}
		sess, err := auth.ParseManual(in.LoginSecure, in.SessionID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.Cfg.Update(func(c *config.Store) { c.Session = sess })
		s.Eng.Logf("Cookies imported for Steam ID %s", sess.SteamID)
		s.Eng.FetchOnce()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// ---- Discord Quest endpoints ----
	s.registerDiscordRoutes(mux)

	return guard(mux)
}

func (s *Server) registerDiscordRoutes(mux *http.ServeMux) {
	// Save / validate Discord token
	mux.HandleFunc("POST /api/discord/token", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.Token == "" {
			http.Error(w, "bad json", 400)
			return
		}
		c := discord.NewClient(in.Token, s.Eng.Logf)
		_, err := c.FetchQuests(r.Context())
		if err != nil {
			if errors.Is(err, discord.ErrUnauthorized) {
				http.Error(w, "Invalid Discord token", 401)
			} else {
				http.Error(w, err.Error(), 502)
			}
			return
		}
		s.Cfg.Update(func(cfg *config.Store) { cfg.Discord.Token = in.Token })
		s.Discord = c
		s.Eng.Logf("[Discord] Token saved and validated")
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Remove Discord token
	mux.HandleFunc("POST /api/discord/logout", func(w http.ResponseWriter, r *http.Request) {
		// Stop any running quests
		s.discordMu.Lock()
		for id, cancel := range s.discordQuests {
			cancel()
			delete(s.discordQuests, id)
		}
		s.discordMu.Unlock()
		s.Discord = nil
		s.Cfg.Update(func(cfg *config.Store) { cfg.Discord.Token = "" })
		s.Eng.Logf("[Discord] Token removed")
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// List quests
	mux.HandleFunc("GET /api/discord/quests", func(w http.ResponseWriter, r *http.Request) {
		if s.Discord == nil {
			writeJSON(w, []struct{}{})
			return
		}
		quests, err := s.Discord.FetchQuests(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		writeJSON(w, quests)
	})

	// Start quest(s)
	mux.HandleFunc("POST /api/discord/start", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			QuestID string `json:"questId"`
			All     bool   `json:"all"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad json", 400)
			return
		}
		if s.Discord == nil {
			http.Error(w, "no discord token", 400)
			return
		}
		quests, err := s.Discord.FetchQuests(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		var toRun []discord.Quest
		for _, q := range quests {
			// Skip already claimed quests
			if q.UserStatus != nil && q.UserStatus.ClaimedAt != "" {
				continue
			}
			if in.All || q.ID == in.QuestID {
				toRun = append(toRun, q)
			}
		}
		s.discordMu.Lock()
		if s.discordQuests == nil {
			s.discordQuests = map[string]context.CancelFunc{}
		}
		for _, q := range toRun {
			if _, running := s.discordQuests[q.ID]; running {
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			s.discordQuests[q.ID] = cancel
			go s.runDiscordQuest(ctx, q)
		}
		s.discordMu.Unlock()
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Stop all quests
	mux.HandleFunc("POST /api/discord/stop", func(w http.ResponseWriter, r *http.Request) {
		s.discordMu.Lock()
		for id, cancel := range s.discordQuests {
			cancel()
			delete(s.discordQuests, id)
		}
		s.discordMu.Unlock()
		s.Eng.Logf("[Discord] All quests stopped")
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Discord state
	mux.HandleFunc("GET /api/discord/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.getDiscordState())
	})
}

func (s *Server) runDiscordQuest(ctx context.Context, quest discord.Quest) {
	defer func() {
		s.discordMu.Lock()
		delete(s.discordQuests, quest.ID)
		s.discordMu.Unlock()
		s.publishDiscordState()
	}()

	s.Eng.Logf("[Discord] Starting quest: %s", quest.Name())
	err := s.Discord.RunQuest(ctx, quest, func(elapsed, total time.Duration) {
		s.publishDiscordState()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		s.Eng.Logf("[Discord] Quest %s error: %v", quest.Name(), err)
	} else if err == nil {
		s.Eng.Logf("[Discord] Quest %s completed!", quest.Name())
	}
}

func (s *Server) getDiscordState() map[string]any {
	s.discordMu.Lock()
	running := make([]string, 0, len(s.discordQuests))
	for id := range s.discordQuests {
		running = append(running, id)
	}
	s.discordMu.Unlock()
	return map[string]any{
		"hasToken": s.Discord != nil,
		"running":  running,
	}
}

func (s *Server) publishDiscordState() {
	b, _ := json.Marshal(s.getDiscordState())
	s.Eng.PublishRaw(farm.Event{Type: "discord", Data: string(b)})
}

// guard blocks DNS-rebinding (Host check) and cross-site POSTs (custom header).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method == "POST" && r.Header.Get("X-XMK") != "1" {
			http.Error(w, "forbidden", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch, cancel := s.Eng.Subscribe()
	defer cancel()
	send := func(typ, data string) {
		fmt.Fprintf(w, "event: %s\n", typ)
		for _, l := range strings.Split(data, "\n") {
			fmt.Fprintf(w, "data: %s\n", l)
		}
		fmt.Fprint(w, "\n")
		fl.Flush()
	}
	send("state", s.Eng.StateJSONReady())
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			send(ev.Type, ev.Data)
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}
