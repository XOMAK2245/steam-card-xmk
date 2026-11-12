// Package web serves the local control UI and JSON API.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/XOMAK2245/steam-card-xmk/internal/auth"
	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/farm"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	Eng  *farm.Engine
	Cfg  *config.Store
	QR   auth.QRLogin
	Quit chan struct{}
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
	act("quit", func() { go func() { time.Sleep(200 * time.Millisecond); close(s.Quit) }() })
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
			c.Settings.ParallelHours, c.Settings.PollMinutes = in.ParallelHours, in.PollMinutes
		})
		s.Eng.Logf("Settings saved (strategy: %s)", in.Strategy)
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
	return guard(mux)
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
