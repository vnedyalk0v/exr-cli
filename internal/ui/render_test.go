package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vnedyalk0v/exr-cli/internal/session"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestCountLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"a\nb\nc\n", 3},
	}
	for _, c := range cases {
		if got := countLines(c.in); got != c.want {
			t.Fatalf("countLines(%q)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestBodyOpenDensity(t *testing.T) {
	readDone := session.Step{Kind: session.KindRead, Status: session.StatusDone, Body: "x", Expanded: true}
	if bodyOpen(readDone, false) {
		t.Fatal("safe read should collapse when not focused")
	}
	if !bodyOpen(readDone, true) {
		t.Fatal("safe read should open when focused+expanded")
	}

	gate := session.Step{Kind: session.KindEdit, Status: session.StatusBlocked, RequiresApproval: true, Body: "diff"}
	if !bodyOpen(gate, false) {
		t.Fatal("gate must force open")
	}

	fail := session.Step{Kind: session.KindShell, Status: session.StatusFailed, Body: "err"}
	if !bodyOpen(fail, false) {
		t.Fatal("fail must force open")
	}
}

func TestRenderStreamOffsetsMonotonic(t *testing.T) {
	sess := session.New("test-model")
	sess.Append(session.Step{Kind: session.KindUser, Status: session.StatusDone, Target: "hello", Body: "hello"})
	sess.Append(session.Step{Kind: session.KindRead, Status: session.StatusDone, Target: "a.go", Body: "line", Expanded: true})
	sess.Append(session.Step{
		Kind: session.KindEdit, Status: session.StatusBlocked, RequiresApproval: true,
		Target: "a.go", Body: "--- a\n+++ b\n+ok\n",
	})
	m := New(sess, nil, false, "rg · fd")
	m.width = 80
	m.height = 40
	m.ready = true
	content, offsets, lines := m.renderStream()
	if content == "" {
		t.Fatal("empty stream")
	}
	if len(offsets) != 3 || len(lines) != 3 {
		t.Fatalf("offsets=%d lines=%d", len(offsets), len(lines))
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] < offsets[i-1]+lines[i-1] {
			// equal end is ok if zero, but should be monotonic non-decreasing start
			if offsets[i] < offsets[i-1] {
				t.Fatalf("offsets not monotonic: %v lines %v", offsets, lines)
			}
		}
	}
	plain := stripANSI(content)
	if !strings.Contains(plain, "hello") {
		t.Fatalf("missing user text: %q", plain)
	}
	if !strings.Contains(plain, "y/n") && !strings.Contains(plain, "Edit") {
		t.Fatalf("missing gate edit: %q", plain)
	}
}

func TestWelcomeAndNarrowPrompt(t *testing.T) {
	sess := session.New("gpt-4o-mini")
	m := New(sess, nil, true, "rg · fd")
	m.width = 40
	m.height = 24
	m.ready = true
	welcome := m.renderWelcome(session.Meta{Perm: session.PermAsk, Model: "gpt-4o-mini"})
	plain := stripANSI(welcome)
	if !strings.Contains(plain, "exr") {
		t.Fatalf("welcome: %q", plain)
	}

	// narrow prompt should not panic
	m.width = 30
	m.layout()
	box := m.renderPromptBox(session.Meta{Perm: session.PermAllow, Model: "very-long-model-name-here"})
	if stripANSI(box) == "" {
		t.Fatal("empty prompt box")
	}
}

func TestPromptBoxFitsTerminalWidth(t *testing.T) {
	sess := session.New("gpt-4o-mini")
	m := New(sess, nil, true, "rg · fd")
	for _, w := range []int{40, 60, 80, 120} {
		m.width = w
		m.height = 30
		m.ready = true
		m.layout()
		box := m.renderPromptBox(session.Meta{Perm: session.PermAllow, Model: "gpt-4o-mini-very-long"})
		// Total rendered width must not exceed terminal (borders outside Width).
		if got := lipgloss.Width(box); got > w {
			t.Fatalf("width %d: prompt box lipgloss.Width=%d > terminal", w, got)
		}
		plain := stripANSI(box)
		if !strings.Contains(plain, ">") {
			t.Fatalf("width %d: missing prompt: %q", w, plain)
		}
	}
}

func TestPromptChipsPresentOnWide(t *testing.T) {
	sess := session.New("gpt-4o-mini")
	m := New(sess, nil, true, "rg")
	m.width = 100
	m.height = 30
	m.ready = true
	m.layout()
	box := m.renderPromptBox(session.Meta{Perm: session.PermAsk, Model: "gpt-4o-mini"})
	plain := stripANSI(box)
	if !strings.Contains(plain, "ask") {
		t.Fatalf("expected perm chip: %q", plain)
	}
	if !strings.Contains(plain, "gpt-4o") {
		t.Fatalf("expected model chip: %q", plain)
	}
}

func TestRenderUserWraps(t *testing.T) {
	m := New(session.New("m"), nil, false, "")
	m.width = 40
	long := strings.Repeat("word ", 40)
	out := m.renderUser(session.Step{Kind: session.KindUser, Target: long}, 40)
	if countLines(out) < 2 {
		t.Fatalf("expected wrap, got %d lines: %q", countLines(out), stripANSI(out))
	}
}

func TestFormatDur(t *testing.T) {
	if formatDur(50*time.Millisecond) != "50ms" {
		t.Fatal(formatDur(50 * time.Millisecond))
	}
	if formatDur(1500*time.Millisecond) != "1.5s" {
		t.Fatal(formatDur(1500 * time.Millisecond))
	}
}

func TestViewDoesNotPanic(t *testing.T) {
	sess := session.New("m")
	m := New(sess, nil, false, "rg")
	// not ready
	_ = m.View()
	m.width = 100
	m.height = 30
	m.ready = true
	m.layout()
	m.rebuildViewport()
	v := m.View()
	if v == "" {
		t.Fatal("empty view")
	}
	// with steps
	sess.Append(session.Step{Kind: session.KindUser, Status: session.StatusDone, Target: "x", Body: "x"})
	m.rebuildViewport()
	_ = m.View()
	m.showHelp = true
	_ = m.View()
}
