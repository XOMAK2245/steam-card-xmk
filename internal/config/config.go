// Package config stores user settings and saved accounts on disk.
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

type SteamAccount struct {
	SteamID     string  `json:"steamId"`
	PersonaName string  `json:"personaName,omitempty"`
	AvatarURL   string  `json:"avatarUrl,omitempty"`
	Session     Session `json:"session"`
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

type DiscordAccount struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"globalName,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	AvatarURL  string `json:"avatarUrl,omitempty"`
	Token      string `json:"token"`
}

type Store struct {
	mu       sync.Mutex
	dir      string
	Session  Session  `json:"session"`
	Settings Settings `json:"settings"`
	Discord  Discord  `json:"discord,omitempty"`

	SteamAccounts   []SteamAccount   `json:"steam_accounts,omitempty"`
	ActiveSteamID   string           `json:"active_steam_id,omitempty"`
	DiscordAccounts []DiscordAccount `json:"discord_accounts,omitempty"`
	ActiveDiscordID string           `json:"active_discord_id,omitempty"`
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
	s.normalize()
	return s
}

func (s *Store) normalize() {
	s.Settings.normalize()

	// 1. Steam migration: if legacy single session exists but accounts list is empty
	if s.Session.SteamID != "" && len(s.SteamAccounts) == 0 {
		s.SteamAccounts = append(s.SteamAccounts, SteamAccount{
			SteamID: s.Session.SteamID,
			Session: s.Session,
		})
		s.ActiveSteamID = s.Session.SteamID
	}
	if s.ActiveSteamID == "" && len(s.SteamAccounts) > 0 {
		s.ActiveSteamID = s.SteamAccounts[0].SteamID
		s.Session = s.SteamAccounts[0].Session
	} else if s.ActiveSteamID != "" {
		for _, a := range s.SteamAccounts {
			if a.SteamID == s.ActiveSteamID {
				s.Session = a.Session
				break
			}
		}
	}

	// 2. Discord migration: if legacy single token exists but accounts list is empty
	if s.Discord.Token != "" && len(s.DiscordAccounts) == 0 {
		s.DiscordAccounts = append(s.DiscordAccounts, DiscordAccount{
			ID:       "default",
			Username: "Discord User",
			Token:    s.Discord.Token,
		})
		s.ActiveDiscordID = "default"
	}
	if s.ActiveDiscordID == "" && len(s.DiscordAccounts) > 0 {
		s.ActiveDiscordID = s.DiscordAccounts[0].ID
		s.Discord.Token = s.DiscordAccounts[0].Token
	} else if s.ActiveDiscordID != "" {
		for _, a := range s.DiscordAccounts {
			if a.ID == s.ActiveDiscordID {
				s.Discord.Token = a.Token
				break
			}
		}
	}
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
	s.normalize()
	s.mu.Unlock()
	return s.Save()
}

// Snapshot returns copies of session and settings.
func (s *Store) Snapshot() (Session, Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Session, s.Settings
}

// AddOrUpdateSteamAccount adds or updates an account and optionally makes it active.
func (s *Store) AddOrUpdateSteamAccount(acc SteamAccount, makeActive bool) {
	idx := -1
	for i, a := range s.SteamAccounts {
		if a.SteamID == acc.SteamID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		if acc.PersonaName != "" {
			s.SteamAccounts[idx].PersonaName = acc.PersonaName
		}
		if acc.AvatarURL != "" {
			s.SteamAccounts[idx].AvatarURL = acc.AvatarURL
		}
		s.SteamAccounts[idx].Session = acc.Session
	} else {
		s.SteamAccounts = append(s.SteamAccounts, acc)
	}
	if makeActive || s.ActiveSteamID == "" || s.ActiveSteamID == acc.SteamID {
		s.ActiveSteamID = acc.SteamID
		s.Session = acc.Session
	}
}

// SwitchSteamAccount switches active Steam account to steamID.
func (s *Store) SwitchSteamAccount(steamID string) bool {
	for _, a := range s.SteamAccounts {
		if a.SteamID == steamID {
			s.ActiveSteamID = steamID
			s.Session = a.Session
			return true
		}
	}
	return false
}

// RemoveSteamAccount removes an account. If active, switches to next available or clears.
func (s *Store) RemoveSteamAccount(steamID string) bool {
	idx := -1
	for i, a := range s.SteamAccounts {
		if a.SteamID == steamID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	s.SteamAccounts = append(s.SteamAccounts[:idx], s.SteamAccounts[idx+1:]...)
	if s.ActiveSteamID == steamID {
		if len(s.SteamAccounts) > 0 {
			s.ActiveSteamID = s.SteamAccounts[0].SteamID
			s.Session = s.SteamAccounts[0].Session
		} else {
			s.ActiveSteamID = ""
			s.Session = Session{}
		}
	}
	return true
}

// AddOrUpdateDiscordAccount adds or updates a Discord account.
func (s *Store) AddOrUpdateDiscordAccount(acc DiscordAccount, makeActive bool) {
	idx := -1
	for i, a := range s.DiscordAccounts {
		if a.ID == acc.ID || a.Token == acc.Token {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.DiscordAccounts[idx] = acc
	} else {
		s.DiscordAccounts = append(s.DiscordAccounts, acc)
	}
	if makeActive || s.ActiveDiscordID == "" || s.ActiveDiscordID == acc.ID {
		s.ActiveDiscordID = acc.ID
		s.Discord.Token = acc.Token
	}
}

// SwitchDiscordAccount switches active Discord account to id.
func (s *Store) SwitchDiscordAccount(id string) bool {
	for _, a := range s.DiscordAccounts {
		if a.ID == id {
			s.ActiveDiscordID = id
			s.Discord.Token = a.Token
			return true
		}
	}
	return false
}

// RemoveDiscordAccount removes a Discord account.
func (s *Store) RemoveDiscordAccount(id string) bool {
	idx := -1
	for i, a := range s.DiscordAccounts {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	s.DiscordAccounts = append(s.DiscordAccounts[:idx], s.DiscordAccounts[idx+1:]...)
	if s.ActiveDiscordID == id {
		if len(s.DiscordAccounts) > 0 {
			s.ActiveDiscordID = s.DiscordAccounts[0].ID
			s.Discord.Token = s.DiscordAccounts[0].Token
		} else {
			s.ActiveDiscordID = ""
			s.Discord.Token = ""
		}
	}
	return true
}
