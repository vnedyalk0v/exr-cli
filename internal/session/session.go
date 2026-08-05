package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Session is the live build-log state for one agent run.
type Session struct {
	mu sync.RWMutex

	ID        string
	CWD       string
	Model     string
	Perm      PermissionMode
	TokensIn  int
	TokensOut int
	Steps     []Step
	// AgentRunning is true while the demo/real agent is producing steps.
	AgentRunning bool
	// Interrupted is set when the user hits Esc mid-run.
	Interrupted bool
	// Synthetic labels the whole session when demo content is used.
	Synthetic bool
}

// New creates a session rooted at cwd (or process cwd).
func New(model string) *Session {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cwd, _ = filepath.Abs(cwd)
	return &Session{
		ID:    shortID(),
		CWD:   cwd,
		Model: model,
		Perm:  PermAsk,
	}
}

func shortID() string {
	return fmt.Sprintf("%04x", time.Now().UnixNano()&0xffff)
}

// Snapshot returns a copy of steps and metadata for rendering.
func (s *Session) Snapshot() (meta Meta, steps []Step) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta = Meta{
		ID:           s.ID,
		CWD:          s.CWD,
		Model:        s.Model,
		Perm:         s.Perm,
		TokensIn:     s.TokensIn,
		TokensOut:    s.TokensOut,
		AgentRunning: s.AgentRunning,
		Interrupted:  s.Interrupted,
		Synthetic:    s.Synthetic,
	}
	steps = make([]Step, len(s.Steps))
	copy(steps, s.Steps)
	return meta, steps
}

// Meta is render-safe session chrome.
type Meta struct {
	ID           string
	CWD          string
	Model        string
	Perm         PermissionMode
	TokensIn     int
	TokensOut    int
	AgentRunning bool
	Interrupted  bool
	Synthetic    bool
}

// ShortCWD returns a display path, preferring ~/…
func (m Meta) ShortCWD() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return m.CWD
	}
	if len(m.CWD) >= len(home) && m.CWD[:len(home)] == home {
		return "~" + m.CWD[len(home):]
	}
	return m.CWD
}

// Append adds a step and returns its index.
func (s *Session) Append(step Step) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if step.ID == "" {
		step.ID = fmt.Sprintf("s%d", len(s.Steps)+1)
	}
	s.Steps = append(s.Steps, step)
	return len(s.Steps) - 1
}

// Update applies a function to a step by index.
func (s *Session) Update(i int, fn func(*Step)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.Steps) {
		return
	}
	fn(&s.Steps[i])
}

// ToggleExpand flips expanded state for a step.
func (s *Session) ToggleExpand(i int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.Steps) {
		return
	}
	s.Steps[i].Expanded = !s.Steps[i].Expanded
}

// SetAgentRunning toggles the running flag.
func (s *Session) SetAgentRunning(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AgentRunning = v
}

// Interrupt marks the session stopped by the user.
// Running tools fail; blocked gates are denied so the UI does not leave a zombie WAIT.
func (s *Session) Interrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Interrupted = true
	s.AgentRunning = false
	for i := range s.Steps {
		switch s.Steps[i].Status {
		case StatusRunning:
			s.Steps[i].Status = StatusFailed
			s.Steps[i].Body = appendBody(s.Steps[i].Body, "interrupted by user")
		case StatusBlocked:
			s.Steps[i].Status = StatusDenied
			s.Steps[i].Approval = ApprovalDenied
			s.Steps[i].Body = appendBody(s.Steps[i].Body, "interrupted by user — not executed")
		}
	}
}

// ClearInterrupt resets interrupt for a new turn.
func (s *Session) ClearInterrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Interrupted = false
}

// AddTokens accumulates token counters (synthetic or real).
func (s *Session) AddTokens(in, out int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TokensIn += in
	s.TokensOut += out
}

// FirstBlocked returns the index of the first approval-blocked step, or -1.
func (s *Session) FirstBlocked() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, st := range s.Steps {
		if st.NeedsGate() {
			return i
		}
	}
	return -1
}

// NextBlockedFrom returns the next blocked/failed step walking from `from` in dir (+1/-1), wrapping.
func (s *Session) NextBlockedFrom(from int, dir int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.Steps)
	if n == 0 {
		return -1
	}
	if dir == 0 {
		dir = 1
	}
	if from < 0 {
		from = 0
	}
	if from >= n {
		from = n - 1
	}
	// Start at k=1 so "from" itself is skipped (exclusive). Callers pass
	// the current cursor; they want the *next* gate in dir, not a no-op.
	for k := 1; k <= n; k++ {
		i := from + dir*k
		i %= n
		if i < 0 {
			i += n
		}
		if s.Steps[i].NeedsGate() || s.Steps[i].Status == StatusFailed {
			return i
		}
	}
	return -1
}

// SetPerm updates permission mode.
func (s *Session) SetPerm(p PermissionMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Perm = p
}

// CyclePerm rotates plan → ask → allow → plan.
func (s *Session) CyclePerm() PermissionMode {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.Perm {
	case PermPlan:
		s.Perm = PermAsk
	case PermAsk:
		s.Perm = PermAllow
	default:
		s.Perm = PermPlan
	}
	return s.Perm
}

// GetPerm returns the current permission mode.
func (s *Session) GetPerm() PermissionMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Perm
}

// StepAt returns a copy of the step at i, or false.
func (s *Session) StepAt(i int) (Step, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= len(s.Steps) {
		return Step{}, false
	}
	return s.Steps[i], true
}

// MarkSynthetic flags demo content.
func (s *Session) MarkSynthetic() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Synthetic = true
}

func appendBody(body, line string) string {
	if body == "" {
		return line
	}
	return body + "\n" + line
}
