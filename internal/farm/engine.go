package farm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/XOMAK2245/steam-card-xmk/internal/auth"
	"github.com/XOMAK2245/steam-card-xmk/internal/badges"
	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/phantom"
)

type Status string

const (
	Stopped Status = "stopped"
	Running Status = "running"
	Paused  Status = "paused"
)

type GameView struct {
	badges.Game
	State string `json:"state"` // queued | active | done | failed
}

type State struct {
	Status       Status          `json:"status"`
	SteamRunning bool            `json:"steamRunning"`
	LoggedIn     bool            `json:"loggedIn"`
	SteamID      string          `json:"steamId,omitempty"`
	Strategy     string          `json:"strategy"`
	Settings     config.Settings `json:"settings"`
	Games        []GameView      `json:"games"`
	Active       []uint32        `json:"active"`
	LastPoll     *time.Time      `json:"lastPoll,omitempty"`
	NextPoll     *time.Time      `json:"nextPoll,omitempty"`
	Note         string          `json:"note,omitempty"` // human-readable problem, if any
	WorkerError  string          `json:"workerError,omitempty"`
}

type Event struct {
	Type string // "state" | "log"
	Data string
}

type Engine struct {
	cfg *config.Store
	mgr *phantom.Manager

	// injectable for tests
	steamRunning func() bool
	fetch        func(ctx context.Context) ([]badges.Game, error)

	mu        sync.Mutex
	status    Status
	games     []badges.Game
	done      map[uint32]badges.Game
	failed    map[uint32]bool
	failCount map[uint32]int
	hoursBase map[uint32]float64   // Steam-reported hours at last refresh
	baseTime  map[uint32]time.Time // when hoursBase was captured
	lastPoll  time.Time
	nextPoll  time.Time
	note      string
	cancel    context.CancelFunc
	wasSteam  bool

	logs []string
	subs map[chan Event]struct{}
}

func New(cfg *config.Store) *Engine {
	e := &Engine{
		cfg: cfg, status: Stopped,
		steamRunning: phantom.SteamRunning,
		done:         map[uint32]badges.Game{}, failed: map[uint32]bool{}, failCount: map[uint32]int{},
		hoursBase: map[uint32]float64{}, baseTime: map[uint32]time.Time{},
		subs: map[chan Event]struct{}{}, wasSteam: true,
	}
	_, st := cfg.Snapshot()
	e.mgr = phantom.NewManager(st.WorkerDir, e.Logf)
	e.fetch = e.fetchBadges
	go e.watchExits()
	return e
}

// ---- events / logging ----

func (e *Engine) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 64)
	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()
	return ch, func() {
		e.mu.Lock()
		delete(e.subs, ch)
		e.mu.Unlock()
	}
}

func (e *Engine) publish(ev Event) {
	for ch := range e.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (e *Engine) Logf(format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	e.mu.Lock()
	e.logs = append(e.logs, line)
	if len(e.logs) > 500 {
		e.logs = e.logs[len(e.logs)-500:]
	}
	e.publish(Event{"log", line})
	e.mu.Unlock()
}

func (e *Engine) Logs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.logs...)
}

func (e *Engine) changed() {
	s := e.StateJSONReady()
	e.mu.Lock()
	e.publish(Event{"state", s})
	e.mu.Unlock()
}

// ---- state ----

func (e *Engine) State() State {
	sess, set := e.cfg.Snapshot()
	running := e.mgr.Running()
	e.mu.Lock()
	defer e.mu.Unlock()
	act := map[uint32]bool{}
	for _, id := range running {
		act[id] = true
	}
	st := State{
		Status: e.status, SteamRunning: e.steamRunning(),
		LoggedIn: sess.SteamID != "" && (sess.AccessToken != "" || sess.LoginSecure != ""),
		SteamID:  sess.SteamID, Strategy: string(set.Strategy), Settings: set,
		Active: running, Note: e.note,
	}
	if st.Active == nil {
		st.Active = []uint32{}
	}
	if err := e.mgr.Check(); err != nil {
		st.WorkerError = err.Error()
	}
	if !e.lastPoll.IsZero() {
		t := e.lastPoll
		st.LastPoll = &t
	}
	if e.status == Running && !e.nextPoll.IsZero() {
		t := e.nextPoll
		st.NextPoll = &t
	}
	for _, g := range e.games {
		v := GameView{Game: g, State: "queued"}
		if act[g.AppID] {
			v.State = "active"
		} else if e.failed[g.AppID] {
			v.State = "failed"
		}
		st.Games = append(st.Games, v)
	}
	for _, g := range e.done {
		g.Remaining = 0
		st.Games = append(st.Games, GameView{Game: g, State: "done"})
	}
	if st.Games == nil {
		st.Games = []GameView{}
	}
	return st
}

func (e *Engine) Start() {
	e.mu.Lock()
	if e.status == Running {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.status = Running
	e.nextPoll = time.Time{}
	e.failed = map[uint32]bool{}
	e.failCount = map[uint32]int{}
	e.mu.Unlock()
	e.Logf("Farming started")
	go e.loop(ctx)
	e.changed()
}

func (e *Engine) halt(to Status) {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.status = to
	e.mu.Unlock()
	e.mgr.StopAll()
	e.Logf("Farming %s", to)
	e.changed()
}

func (e *Engine) Pause() { e.halt(Paused) }
func (e *Engine) Stop()  { e.halt(Stopped) }

// Shutdown stops everything; used on application exit.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
	e.mgr.StopAll()
}

// RefreshNow forces a badge page refresh on the next tick (works while running).
func (e *Engine) RefreshNow() {
	e.mu.Lock()
	e.nextPoll = time.Time{}
	e.mu.Unlock()
}

// FetchOnce loads the game list without starting to farm.
func (e *Engine) FetchOnce() {
	go func() {
		e.refresh(context.Background(), false)
		e.changed()
	}()
}

// ---- main loop ----

func (e *Engine) loop(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	e.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) setNote(n string) {
	e.mu.Lock()
	e.note = n
	e.mu.Unlock()
}

func (e *Engine) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	// 1. Steam client alive?
	if !e.steamRunning() {
		if e.wasSteam {
			e.Logf("Steam client is not running - waiting (phantom games stopped)")
		}
		e.wasSteam = false
		e.mgr.StopAll()
		e.setNote("Steam client is not running. Start Steam and farming will resume automatically.")
		e.changed()
		return
	}
	if !e.wasSteam {
		e.Logf("Steam client detected again")
		e.wasSteam = true
	}
	if err := e.mgr.Check(); err != nil {
		e.setNote("Worker files are missing: " + err.Error())
		e.changed()
		return
	}

	// 2. Refresh badges if due.
	e.mu.Lock()
	due := time.Now().After(e.nextPoll)
	e.mu.Unlock()
	if due {
		if !e.refresh(ctx, true) {
			e.changed()
			return
		}
	}

	// 3. Reconcile running workers with the plan.
	e.mu.Lock()
	_, set := e.cfg.Snapshot()
	games := append([]badges.Game(nil), e.games...)
	hours := map[uint32]float64{}
	for _, g := range games {
		h := g.Hours
		if b, ok := e.hoursBase[g.AppID]; ok {
			h = b + time.Since(e.baseTime[g.AppID]).Hours()*boolF(e.mgr.IsRunning(g.AppID))
		}
		hours[g.AppID] = h
	}
	skip := map[uint32]bool{}
	for id := range e.failed {
		skip[id] = true
	}
	e.mu.Unlock()

	target := Plan(games, hours, skip, set.Strategy, set.MaxParallel, set.ParallelHours)
	want := map[uint32]bool{}
	for _, id := range target {
		want[id] = true
	}
	names := map[uint32]string{}
	for _, g := range games {
		names[g.AppID] = g.Name
	}
	for _, id := range e.mgr.Running() {
		if !want[id] {
			e.Logf("Stopping %s (%d)", names[id], id)
			e.mgr.Stop(id)
		}
	}
	for _, id := range target {
		if !e.mgr.IsRunning(id) {
			e.mu.Lock()
			g := gameByID(e.games, id)
			e.hoursBase[id], e.baseTime[id] = g.Hours, time.Now()
			e.mu.Unlock()
			if err := e.mgr.Start(id); err != nil {
				e.Logf("Failed to start %d: %v", id, err)
				e.markFail(id)
			} else {
				e.Logf("Started phantom: %s (%d), %d card(s) left, %.1f h", names[id], id, g.Remaining, g.Hours)
			}
		}
	}
	if len(target) == 0 {
		e.setNote("Nothing to farm: no games with card drops remaining.")
	} else {
		e.setNote("")
	}
	e.changed()
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func gameByID(gs []badges.Game, id uint32) badges.Game {
	for _, g := range gs {
		if g.AppID == id {
			return g
		}
	}
	return badges.Game{AppID: id}
}

func (e *Engine) markFail(id uint32) {
	e.mu.Lock()
	e.failed[id] = true
	e.mu.Unlock()
}

// refresh polls the badge pages, stops finished games and updates the queue.
// Returns false if the loop should not continue this tick.
func (e *Engine) refresh(ctx context.Context, farming bool) bool {
	games, err := e.fetch(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		e.Logf("Badge refresh failed: %v", err)
		switch {
		case errors.Is(err, badges.ErrAuth):
			e.setNote("Steam session expired - please log in again.")
			if farming {
				e.halt(Paused)
			}
		case errors.Is(err, badges.ErrRateLimit):
			e.mu.Lock()
			e.nextPoll = time.Now().Add(15 * time.Minute)
			e.note = "Steam rate limit - retrying in 15 minutes (running games keep running)."
			e.mu.Unlock()
			return true
		default:
			e.mu.Lock()
			e.nextPoll = time.Now().Add(time.Minute)
			e.note = "Could not reach Steam Community: " + err.Error()
			e.mu.Unlock()
			return true
		}
		return false
	}
	_, set := e.cfg.Snapshot()
	e.mu.Lock()
	cur := map[uint32]badges.Game{}
	for _, g := range games {
		cur[g.AppID] = g
	}
	var finished []badges.Game
	for _, old := range e.games {
		if _, ok := cur[old.AppID]; !ok {
			e.done[old.AppID] = old
			finished = append(finished, old)
		}
	}
	e.games = games
	for _, g := range games {
		delete(e.done, g.AppID)
		if e.mgr.IsRunning(g.AppID) {
			e.hoursBase[g.AppID], e.baseTime[g.AppID] = g.Hours, time.Now()
		}
	}
	e.lastPoll = time.Now()
	e.nextPoll = e.lastPoll.Add(time.Duration(set.PollMinutes) * time.Minute)
	e.note = ""
	e.mu.Unlock()

	for _, g := range finished {
		if e.mgr.IsRunning(g.AppID) {
			e.Logf("All cards dropped for %s (%d) - stopping phantom", g.Name, g.AppID)
			e.mgr.Stop(g.AppID)
		}
		e.Logf("Completed: %s (%d)", g.Name, g.AppID)
	}
	left := 0
	for _, g := range games {
		left += g.Remaining
	}
	e.Logf("Badges refreshed: %d game(s) with drops, %d card(s) remaining", len(games), left)
	return true
}

// fetchBadges loads the badge pages with the stored session, renewing the
// access token when needed.
func (e *Engine) fetchBadges(ctx context.Context) ([]badges.Game, error) {
	sess, _ := e.cfg.Snapshot()
	if sess.SteamID == "" {
		return nil, badges.ErrAuth
	}
	renewed := false
	if sess.RefreshToken != "" && (sess.AccessToken == "" || auth.TokenExpiresSoon(sess.AccessToken, 15*time.Minute)) {
		if err := auth.Renew(ctx, &sess); err != nil {
			e.Logf("Token renewal failed: %v", err)
		} else {
			renewed = true
		}
	}
	for attempt := 0; ; attempt++ {
		gs, err := badges.NewClient(sess.SteamID, auth.Cookies(sess)).FetchAll(ctx)
		if errors.Is(err, badges.ErrAuth) && attempt == 0 && sess.RefreshToken != "" && !renewed {
			if rerr := auth.Renew(ctx, &sess); rerr == nil {
				renewed = true
				continue
			}
		}
		if renewed {
			tok := sess.AccessToken
			e.cfg.Update(func(c *config.Store) { c.Session.AccessToken = tok })
		}
		return gs, err
	}
}

func (e *Engine) watchExits() {
	for ev := range e.mgr.Events {
		if ev.Stopped {
			continue
		}
		e.mu.Lock()
		running := e.status == Running
		e.mu.Unlock()
		if !running {
			continue
		}
		if ev.Lived < 15*time.Second {
			e.mu.Lock()
			e.failCount[ev.AppID]++
			n := e.failCount[ev.AppID]
			e.mu.Unlock()
			e.Logf("Phantom for %d exited right after start (%v) - is Steam running and the game owned? (attempt %d/3)", ev.AppID, ev.Err, n)
			if n >= 3 {
				e.markFail(ev.AppID)
				e.Logf("Giving up on %d for this session", ev.AppID)
			}
		} else {
			e.Logf("Phantom for %d exited unexpectedly (%v) - will restart", ev.AppID, ev.Err)
		}
		e.changed()
	}
}

// StateJSONReady returns the JSON encoding of the current state.
func (e *Engine) StateJSONReady() string {
	b, _ := jsonMarshal(e.State())
	return string(b)
}
