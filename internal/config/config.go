// Package config stores user settings and the Steam session on disk.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Strategy string

const (
	Batch  Strategy = "batch"
	Single Strategy = "single"
)

type Session struct {
	SteamID      string `json:"steamId"`
	AccessToken  string `json:"accessToken,omitempty"`  // JWT used in steamLoginSecure
	RefreshToken string `json:"refreshToken,omitempty"` // lets us renew the access token
	LoginSecure  string `json:"loginSecure,omitempty"`  // manual cookie import
	SessionID    string `json:"sessionId"`
}

type Settings struct {
	Strategy    Strategy `json:"strategy"`
	MaxParallel int      `json:"maxParallel"` // batch mode: games at once (up to 32)
	PollMinutes int      `json:"pollMinutes"` // badge page refresh interval
	WorkerDir   string   `json:"workerDir,omitempty"`
}

type Discord struct {
	Token string `json:"token,omitempty"`
}

type Store struct {
	mu       sync.Mutex
	dir      string
	Session  Session  `json:"session"`
	Settings Settings `json:"settings"`
	Discord  Discord  `json:"discord,omitempty"`
}

func Default() Settings {
	return Settings{Strategy: Batch, MaxParallel: 32, PollMinutes: 5}
}

func Dir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "steam-card-xmk")
}

func Load() *Store {
	s := &Store{dir: Dir(), Settings: Default()}
	if b, err := os.ReadFile(filepath.Join(s.dir, "config.json")); err == nil {
		json.Unmarshal(b, s)
	}
	s.Settings.normalize()
	return s
}

func (st *Settings) normalize() {
	d := Default()
	switch st.Strategy {
	case "sequential", Single:
		st.Strategy = Single
	case "smart", Batch:
		st.Strategy = Batch
	default:
		st.Strategy = d.Strategy
	}
	if st.MaxParallel < 1 || st.MaxParallel > 32 {
		st.MaxParallel = d.MaxParallel
	}
	if st.PollMinutes < 5 {
		st.PollMinutes = 5 // protect against Steam rate limits
	}
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := filepath.Join(s.dir, "config.json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "config.json"))
}

func (s *Store) Update(f func(*Store)) error {
	s.mu.Lock()
	f(s)
	s.Settings.normalize()
	s.mu.Unlock()
	return s.Save()
}

// Snapshot returns copies of session and settings.
func (s *Store) Snapshot() (Session, Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Session, s.Settings
}
