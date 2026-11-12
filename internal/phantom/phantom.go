// Package phantom starts and stops the native phantom worker processes that
// make the local Steam client believe a game is running.
package phantom

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type proc struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	err   error
}

// ExitEvent is reported when a worker exits.
type ExitEvent struct {
	AppID   uint32
	Err     error
	Stopped bool // true if we asked it to stop
	Lived   time.Duration
}

type Manager struct {
	dir    string
	mu     sync.Mutex
	procs  map[uint32]*proc
	stops  map[uint32]bool
	Log    func(format string, args ...any)
	Events chan ExitEvent
}

func NewManager(dir string, logf func(string, ...any)) *Manager {
	if dir == "" {
		dir = defaultDir()
	}
	return &Manager{dir: dir, procs: map[uint32]*proc{}, stops: map[uint32]bool{}, Log: logf, Events: make(chan ExitEvent, 64)}
}

func defaultDir() string {
	if d := os.Getenv("STEAM_CARD_XMK_WORKER_DIR"); d != "" {
		return d
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Dir(exe)
}

// Check verifies that worker binary and Steamworks library are present.
func (m *Manager) Check() error {
	for _, f := range []string{workerName, libName} {
		if _, err := os.Stat(filepath.Join(m.dir, f)); err != nil {
			return fmt.Errorf("missing %s in %s", f, m.dir)
		}
	}
	return nil
}

func (m *Manager) Start(appID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.procs[appID]; ok {
		return nil
	}
	if err := m.Check(); err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(m.dir, workerName), fmt.Sprint(appID))
	cmd.Dir = m.dir
	prepare(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := afterStart(cmd); err != nil {
		go m.Log("warning: %v", err)
	}
	p := &proc{cmd: cmd, stdin: stdin, done: make(chan struct{})}
	m.procs[appID] = p
	started := time.Now()
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			m.Log("[worker %d] %s", appID, sc.Text())
		}
	}()
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		delete(m.procs, appID)
		stopped := m.stops[appID]
		delete(m.stops, appID)
		m.mu.Unlock()
		p.err = err
		close(p.done)
		if stopped {
			err = nil
		}
		m.Events <- ExitEvent{AppID: appID, Err: err, Stopped: stopped, Lived: time.Since(started)}
	}()
	return nil
}

// Stop asks the worker to shut down gracefully (SteamAPI_Shutdown) and waits.
func (m *Manager) Stop(appID uint32) {
	m.mu.Lock()
	p := m.procs[appID]
	if p != nil {
		m.stops[appID] = true
	}
	m.mu.Unlock()
	if p == nil {
		return
	}
	terminate(p.cmd, p.stdin)
	select {
	case <-p.done:
	case <-time.After(8 * time.Second):
		p.cmd.Process.Kill()
		<-p.done
	}
}

func (m *Manager) StopAll() {
	for _, id := range m.Running() {
		m.Stop(id)
	}
}

func (m *Manager) Running() []uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]uint32, 0, len(m.procs))
	for id := range m.procs {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) IsRunning(appID uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[appID]
	return ok
}
