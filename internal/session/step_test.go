package session

import (
	"strings"
	"testing"
	"time"
)

func TestGlyphAndGate(t *testing.T) {
	s := Step{Status: StatusBlocked, RequiresApproval: true, Kind: KindEdit}
	if !s.NeedsGate() {
		t.Fatal("expected gate")
	}
	if s.Glyph("") != "■" {
		t.Fatalf("glyph: %q", s.Glyph(""))
	}
	s.Status = StatusDone
	s.Duration = 50 * time.Millisecond
	line := s.SummaryLine(80, "")
	if !strings.Contains(line, "50ms") {
		t.Fatalf("expected duration in %q", line)
	}
	// safe done uses middot
	r := Step{Status: StatusDone, Kind: KindRead}
	if r.Glyph("") != "·" {
		t.Fatalf("safe glyph %q", r.Glyph(""))
	}
}

func TestNextBlockedFromExclusive(t *testing.T) {
	s := New("m")
	s.Append(Step{Kind: KindEdit, Status: StatusBlocked, RequiresApproval: true, Target: "a"})
	s.Append(Step{Kind: KindRead, Status: StatusDone, Target: "b"})
	s.Append(Step{Kind: KindShell, Status: StatusFailed, Target: "c"})
	// from gate 0, forward should skip self → land on fail at 2
	if j := s.NextBlockedFrom(0, 1); j != 2 {
		t.Fatalf("forward from 0: got %d want 2", j)
	}
	// from fail 2, forward wraps to gate 0
	if j := s.NextBlockedFrom(2, 1); j != 0 {
		t.Fatalf("forward wrap: got %d want 0", j)
	}
	// from 0 backward → fail at 2
	if j := s.NextBlockedFrom(0, -1); j != 2 {
		t.Fatalf("backward from 0: got %d want 2", j)
	}
}

func TestInterruptDeniesGates(t *testing.T) {
	s := New("m")
	s.Append(Step{Kind: KindEdit, Status: StatusBlocked, RequiresApproval: true, Target: "a", Body: "diff"})
	s.Append(Step{Kind: KindShell, Status: StatusRunning, Target: "go test"})
	s.Interrupt()
	a, _ := s.StepAt(0)
	b, _ := s.StepAt(1)
	if a.Status != StatusDenied || a.Approval != ApprovalDenied {
		t.Fatalf("gate: %+v", a)
	}
	if b.Status != StatusFailed {
		t.Fatalf("running: %+v", b)
	}
	if s.FirstBlocked() >= 0 {
		t.Fatal("no blocked after interrupt")
	}
}
