// Package web serves the local control UI and JSON API.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
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
	Discord              *discord.Client
	discordMu            sync.Mutex
	discordCancel        context.CancelFunc // cancels active queue runner
	discordCurrent       string             // ID of currently running quest
	discordElapsed       float64            // Progress seconds for currently running quest
	discordTotal         float64            // Target seconds for currently running quest
	discordLastCompleted string             // ID of the quest that completed most recently
}

// InitDiscord creates the Discord client if a token is configured.
func (s *Server) InitDiscord() {
	s.Cfg.Update(func(c *config.Store) {
		token := c.Discord.Token
		if c.ActiveDiscordID != "" {
			for _, a := range c.DiscordAccounts {
				if a.ID == c.ActiveDiscordID && a.Token != "" {
					token = a.Token
					break
				}
			}
		}
		if token != "" {
			s.Discord = discord.NewClient(token, s.Eng.Logf)
			// Asynchronously refresh user profile in background
			go func(tok, id string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				client := discord.NewClient(tok, nil)
				u, err := client.FetchUser(ctx)
				if err == nil && u != nil {
					s.Cfg.Update(func(cfg *config.Store) {
						for i := range cfg.DiscordAccounts {
							if cfg.DiscordAccounts[i].ID == id || cfg.DiscordAccounts[i].Token == tok {
								cfg.DiscordAccounts[i].ID = u.ID
								cfg.DiscordAccounts[i].Username = u.Username
								cfg.DiscordAccounts[i].GlobalName = u.DisplayName()
								cfg.DiscordAccounts[i].Avatar = u.Avatar
								cfg.DiscordAccounts[i].AvatarURL = u.AvatarURL()
								if cfg.ActiveDiscordID == id || cfg.ActiveDiscordID == "default" {
									cfg.ActiveDiscordID = u.ID
								}
								break
							}
						}
					})
					s.publishDiscordState()
				}
			}(token, c.ActiveDiscordID)
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
		s.Cfg.Update(func(c *config.Store) {
			if c.ActiveSteamID != "" {
				c.RemoveSteamAccount(c.ActiveSteamID)
			} else {
				c.Session = config.Session{}
			}
		})
		s.Eng.Logf("Logged out active Steam account")
	})

	// Switch active Steam account
	mux.HandleFunc("POST /api/steam/switch", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SteamID string `json:"steamId"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.SteamID == "" {
			http.Error(w, "bad json", 400)
			return
		}
		s.Eng.Stop()
		s.Cfg.Update(func(c *config.Store) {
			c.SwitchSteamAccount(in.SteamID)
		})
		s.Eng.Logf("Switched to Steam account %s", in.SteamID)
		s.Eng.FetchOnce()
		writeJSON(w, s.Eng.State())
	})

	// Remove a Steam account
	mux.HandleFunc("POST /api/steam/remove", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SteamID string `json:"steamId"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.SteamID == "" {
			http.Error(w, "bad json", 400)
			return
		}
		s.Eng.Stop()
		s.Cfg.Update(func(c *config.Store) {
			c.RemoveSteamAccount(in.SteamID)
		})
		s.Eng.Logf("Removed Steam account %s", in.SteamID)
		s.Eng.FetchOnce()
		writeJSON(w, s.Eng.State())
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
			s.Cfg.Update(func(c *config.Store) {
				c.AddOrUpdateSteamAccount(config.SteamAccount{
					SteamID: sess.SteamID,
					Session: sess,
				}, true)
			})
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
		s.Cfg.Update(func(c *config.Store) {
			c.AddOrUpdateSteamAccount(config.SteamAccount{
				SteamID: sess.SteamID,
				Session: sess,
			}, true)
		})
		s.Eng.Logf("Cookies imported for Steam ID %s", sess.SteamID)
		s.Eng.FetchOnce()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// ---- Discord Quest endpoints ----
	s.registerDiscordRoutes(mux)

	return guard(mux)
}

func (s *Server) registerDiscordRoutes(mux *http.ServeMux) {
	// Save / add Discord account
	mux.HandleFunc("POST /api/discord/token", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || strings.TrimSpace(in.Token) == "" {
			http.Error(w, "bad json", 400)
			return
		}
		token := strings.TrimSpace(in.Token)
		c := discord.NewClient(token, s.Eng.Logf)
		u, err := c.FetchUser(r.Context())
		if err != nil {
			if errors.Is(err, discord.ErrUnauthorized) {
				http.Error(w, "Invalid Discord token", 401)
			} else {
				http.Error(w, err.Error(), 502)
			}
			return
		}

		acc := config.DiscordAccount{
			ID:         u.ID,
			Username:   u.Username,
			GlobalName: u.DisplayName(),
			Avatar:     u.Avatar,
			AvatarURL:  u.AvatarURL(),
			Token:      token,
		}

		s.Cfg.Update(func(cfg *config.Store) {
			cfg.AddOrUpdateDiscordAccount(acc, true)
		})

		s.discordMu.Lock()
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""
		s.Discord = c
		s.discordMu.Unlock()

		s.Eng.Logf("[Discord] Added account %s (%s)", acc.GlobalName, acc.ID)
		s.publishDiscordState()
		writeJSON(w, map[string]any{"ok": true, "account": acc})
	})

	// Switch active Discord account
	mux.HandleFunc("POST /api/discord/switch", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.ID == "" {
			http.Error(w, "bad json", 400)
			return
		}
		s.discordMu.Lock()
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""

		var newToken string
		s.Cfg.Update(func(cfg *config.Store) {
			cfg.SwitchDiscordAccount(in.ID)
			for _, a := range cfg.DiscordAccounts {
				if a.ID == cfg.ActiveDiscordID {
					newToken = a.Token
					break
				}
			}
		})
		if newToken != "" {
			s.Discord = discord.NewClient(newToken, s.Eng.Logf)
		} else {
			s.Discord = nil
		}
		s.discordMu.Unlock()

		s.Eng.Logf("[Discord] Switched to account %s", in.ID)
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Remove a Discord account
	mux.HandleFunc("POST /api/discord/remove", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.ID == "" {
			http.Error(w, "bad json", 400)
			return
		}
		s.discordMu.Lock()
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""

		var activeToken string
		s.Cfg.Update(func(cfg *config.Store) {
			cfg.RemoveDiscordAccount(in.ID)
			for _, a := range cfg.DiscordAccounts {
				if a.ID == cfg.ActiveDiscordID {
					activeToken = a.Token
					break
				}
			}
		})
		if activeToken != "" {
			s.Discord = discord.NewClient(activeToken, s.Eng.Logf)
		} else {
			s.Discord = nil
		}
		s.discordMu.Unlock()

		s.Eng.Logf("[Discord] Removed account %s", in.ID)
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Logout / remove active Discord account
	mux.HandleFunc("POST /api/discord/logout", func(w http.ResponseWriter, r *http.Request) {
		s.discordMu.Lock()
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""
		s.Discord = nil
		s.discordMu.Unlock()

		s.Cfg.Update(func(cfg *config.Store) {
			if cfg.ActiveDiscordID != "" {
				cfg.RemoveDiscordAccount(cfg.ActiveDiscordID)
			} else {
				cfg.Discord.Token = ""
			}
		})
		s.Eng.Logf("[Discord] Logged out active account")
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

	// Enroll a single quest
	mux.HandleFunc("POST /api/discord/enroll", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			QuestID string `json:"questId"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil || in.QuestID == "" {
			http.Error(w, "bad json", 400)
			return
		}
		if s.Discord == nil {
			http.Error(w, "no discord token", 400)
			return
		}
		s.Eng.Logf("[Discord] Enrolling in quest %s...", in.QuestID)
		if err := s.Discord.Enroll(r.Context(), in.QuestID); err != nil {
			s.Eng.Logf("[Discord] Enroll %s failed: %v", in.QuestID, err)
			http.Error(w, err.Error(), 502)
			return
		}
		s.Eng.Logf("[Discord] Quest %s enrolled successfully", in.QuestID)
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Start quest(s) - strictly sequential queue execution
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
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""

		if len(toRun) == 0 {
			s.discordMu.Unlock()
			s.publishDiscordState()
			writeJSON(w, map[string]bool{"ok": true})
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		s.discordCancel = cancel
		s.discordMu.Unlock()

		go s.processDiscordQueue(ctx, toRun)

		writeJSON(w, map[string]bool{"ok": true})
	})

	// Stop all quests
	mux.HandleFunc("POST /api/discord/stop", func(w http.ResponseWriter, r *http.Request) {
		s.discordMu.Lock()
		if s.discordCancel != nil {
			s.discordCancel()
			s.discordCancel = nil
		}
		s.discordCurrent = ""
		s.discordMu.Unlock()
		s.Eng.Logf("[Discord] Quests stopped")
		s.publishDiscordState()
		writeJSON(w, map[string]bool{"ok": true})
	})

	// Discord state
	mux.HandleFunc("GET /api/discord/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.getDiscordState())
	})
}

func (s *Server) processDiscordQueue(ctx context.Context, quests []discord.Quest) {
	defer func() {
		s.discordMu.Lock()
		s.discordCurrent = ""
		s.discordElapsed = 0
		s.discordTotal = 0
		s.discordCancel = nil
		s.discordMu.Unlock()
		s.publishDiscordState()
	}()

	for i, q := range quests {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.discordMu.Lock()
		s.discordCurrent = q.ID
		s.discordElapsed = q.SecondsDone
		s.discordTotal = q.SecondsTarget
		s.discordMu.Unlock()
		s.publishDiscordState()

		s.Eng.Logf("[Discord] [%d/%d] Starting quest: %s (%s)", i+1, len(quests), q.Name(), q.ID)
		err := s.Discord.RunQuest(ctx, q, func(elapsed, total float64) {
			s.discordMu.Lock()
			s.discordElapsed = elapsed
			s.discordTotal = total
			s.discordMu.Unlock()
			s.publishDiscordState()
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				s.Eng.Logf("[Discord] Quest %s canceled", q.Name())
				return
			}
			s.Eng.Logf("[Discord] Quest %s error: %v", q.Name(), err)
			// Abort only on invalid token or active >60s cooldown
			if errors.Is(err, discord.ErrUnauthorized) {
				s.Eng.Logf("[Discord] Stopping quest queue due to invalid token")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "rate limit cooldown active") {
				s.Eng.Logf("[Discord] Stopping quest queue due to rate limit cooldown")
				return
			}

			// If error is task-specific (HTTP 400, unsupported quest type, etc.), DO NOT break queue:
			// pause 5 seconds and proceed to the next quest!
			if i < len(quests)-1 {
				s.Eng.Logf("[Discord] Skipping to next quest in 5s...")
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
			continue
		} else {
			s.Eng.Logf("[Discord] Quest %s completed successfully!", q.Name())
			s.discordMu.Lock()
			s.discordLastCompleted = q.ID
			s.discordMu.Unlock()
			s.publishDiscordState()
		}

		// Pause between sequential quests (5-10s) with natural jitter
		if i < len(quests)-1 {
			pause := time.Duration(5000+rand.IntN(5000)) * time.Millisecond
			s.Eng.Logf("[Discord] Waiting %v before next quest...", pause.Round(time.Second))
			select {
			case <-ctx.Done():
				return
			case <-time.After(pause):
			}
		}
	}
}

func (s *Server) getDiscordState() map[string]any {
	s.discordMu.Lock()
	running := []string{}
	if s.discordCurrent != "" {
		running = append(running, s.discordCurrent)
	}
	hasToken := s.Discord != nil
	current := s.discordCurrent
	elapsed := s.discordElapsed
	total := s.discordTotal
	lastCompleted := s.discordLastCompleted
	s.discordMu.Unlock()

	accounts := []config.DiscordAccount{}
	activeID := ""
	s.Cfg.Update(func(cfg *config.Store) {
		accounts = append(accounts, cfg.DiscordAccounts...)
		activeID = cfg.ActiveDiscordID
	})

	safeAccounts := make([]map[string]any, 0, len(accounts))
	for _, a := range accounts {
		safeAccounts = append(safeAccounts, map[string]any{
			"id":         a.ID,
			"username":   a.Username,
			"globalName": a.GlobalName,
			"avatar":     a.Avatar,
			"avatarUrl":  a.AvatarURL,
		})
	}

	return map[string]any{
		"hasToken":      hasToken,
		"running":       running,
		"current":       current,
		"elapsed":       elapsed,
		"total":         total,
		"lastCompleted": lastCompleted,
		"accounts":      safeAccounts,
		"activeId":      activeID,
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
