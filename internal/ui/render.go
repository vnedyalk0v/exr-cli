package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vnedyalk0v/exr-cli/internal/session"
)

// Pure rendering helpers (testable without a Bubble Tea program).

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if strings.HasSuffix(s, "\n") {
		return n
	}
	return n + 1
}

func ensureNL(s string) string {
	if s == "" {
		return "\n"
	}
	if !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}

func formatDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func toolVerbLabel(st session.Step) string {
	// Grok-ish past tense when finished.
	done := st.Status == session.StatusDone || st.Status == session.StatusDenied
	switch st.Kind {
	case session.KindRead:
		return "Read"
	case session.KindSearch:
		if done {
			return "Searched"
		}
		return "Search"
	case session.KindEdit:
		if done {
			return "Edited"
		}
		return "Edit"
	case session.KindShell:
		if done {
			return "Ran"
		}
		return "Run"
	default:
		if st.Kind == "" {
			return "Tool"
		}
		return string(st.Kind)
	}
}

func bodyOpen(st session.Step, focused bool) bool {
	if st.NeedsGate() || st.Status == session.StatusFailed {
		return true
	}
	if st.Status == session.StatusRunning && st.Body != "" {
		return true
	}
	if st.IsSafeKind() && (st.Status == session.StatusDone || st.Status == session.StatusDenied || st.Status == session.StatusSkipped) {
		return focused && st.Expanded
	}
	if st.Kind == session.KindResult {
		return true
	}
	return st.Expanded && st.Body != ""
}

func wrapLine(ln string, width int) []string {
	if width < 8 {
		width = 8
	}
	if lipgloss.Width(ln) <= width {
		return []string{ln}
	}
	var parts []string
	var cur []rune
	curW := 0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		parts = append(parts, string(cur))
		cur = cur[:0]
		curW = 0
	}
	for _, r := range ln {
		rw := lipgloss.Width(string(r))
		if rw < 1 {
			rw = 1
		}
		if curW+rw > width && len(cur) > 0 {
			// prefer break at last space in current line
			if i := lastSpace(cur); i > len(cur)/4 {
				parts = append(parts, string(cur[:i+1]))
				cur = cur[i+1:]
				curW = lipgloss.Width(string(cur))
			} else {
				flush()
			}
		}
		cur = append(cur, r)
		curW += rw
	}
	flush()
	if len(parts) == 0 {
		return []string{ln}
	}
	return parts
}

func lastSpace(r []rune) int {
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] == ' ' || r[i] == '\t' {
			return i
		}
	}
	return -1
}

func renderBody(st session.Step, width int) string {
	var out []string
	for _, ln := range strings.Split(st.Body, "\n") {
		for _, wln := range wrapLine(ln, width) {
			out = append(out, styleBodyLine(st, wln))
		}
	}
	return strings.Join(out, "\n")
}

func styleBodyLine(st session.Step, ln string) string {
	base := lipgloss.NewStyle().Background(colorBg)
	if st.Kind == session.KindEdit || looksLikeDiff(ln) {
		trim := strings.TrimLeft(ln, " ")
		switch {
		case strings.HasPrefix(trim, "+") && !strings.HasPrefix(trim, "+++"):
			return base.Foreground(colorOk).Render(ln)
		case strings.HasPrefix(trim, "-") && !strings.HasPrefix(trim, "---"):
			return base.Foreground(colorDiffM).Render(ln)
		case strings.HasPrefix(trim, "@@"):
			return base.Foreground(colorAccent).Render(ln)
		case strings.HasPrefix(trim, "---") || strings.HasPrefix(trim, "+++"):
			return base.Foreground(colorDim).Render(ln)
		}
	}
	if st.Status == session.StatusFailed {
		return base.Foreground(colorFail).Render(ln)
	}
	return base.Foreground(colorDim).Render(ln)
}

func looksLikeDiff(ln string) bool {
	trim := strings.TrimLeft(ln, " ")
	return strings.HasPrefix(trim, "+") || strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "@@") ||
		strings.HasPrefix(trim, "---") || strings.HasPrefix(trim, "+++")
}

func permHelp(p session.PermissionMode) string {
	switch p {
	case session.PermPlan:
		return "plan: inspect only; writes/shell denied."
	case session.PermAllow:
		return "allow (always-approve): tools run without asking."
	default:
		return "ask: reads auto; edits and shell need y/n."
	}
}
