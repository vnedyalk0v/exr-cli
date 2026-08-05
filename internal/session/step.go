// Package session holds the build-log model for an agent session.
package session

import (
	"fmt"
	"strings"
	"time"
)

// Kind is the type of a build-log step.
type Kind string

const (
	KindUser    Kind = "user"
	KindPlan    Kind = "plan"
	KindThink   Kind = "think"
	KindRead    Kind = "read"
	KindSearch  Kind = "search"
	KindEdit    Kind = "edit"
	KindShell   Kind = "shell"
	KindApprove Kind = "approve"
	KindResult  Kind = "result"
	KindSystem  Kind = "system"
)

// Status is the lifecycle state of a step.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusBlocked  Status = "blocked"
	StatusSkipped  Status = "skipped"
	StatusDenied   Status = "denied"
)

// PermissionMode controls how tools are gated.
type PermissionMode string

const (
	PermPlan PermissionMode = "plan"
	PermAsk  PermissionMode = "ask"
	PermAllow PermissionMode = "allow"
)

// Approval is the human decision on a gated tool step.
type Approval string

const (
	ApprovalNone     Approval = ""
	ApprovalApproved Approval = "approved"
	ApprovalDenied   Approval = "denied"
)

// Step is one first-class line in the session stream.
// Tool calls and diffs live here — never hidden behind chat-only summaries.
type Step struct {
	ID       string
	Kind     Kind
	Status   Status
	Target   string // short summary: path, command, or title
	Body     string // expandable: args, stdout, diff hunks, errors
	Duration time.Duration
	Expanded bool
	// RequiresApproval marks steps that block until y/n (writes/shell in ask mode).
	RequiresApproval bool
	// Approval is set by the UI (y/n); the agent loop executes after Approved.
	Approval Approval
	// Tool metadata for the agent loop (OpenAI tool_calls).
	ToolName   string
	ToolArgs   string // JSON arguments
	ToolCallID string
	// Synthetic marks demo/fixture content so it is never mistaken for a real model run.
	Synthetic bool
}

// Glyph returns a single-cell status marker for the build log.
// Safe completed steps use · ; write/shell/result use ✓ (engineer density).
func (s Step) Glyph(spinnerFrame string) string {
	switch s.Status {
	case StatusRunning:
		if spinnerFrame != "" {
			return spinnerFrame
		}
		return "…"
	case StatusDone:
		switch s.Kind {
		case KindEdit, KindShell, KindResult, KindUser:
			return "✓"
		default:
			return "·"
		}
	case StatusFailed:
		return "✗"
	case StatusBlocked:
		return "■"
	case StatusSkipped, StatusDenied:
		return "–"
	default:
		return "·"
	}
}

// IsSafeKind reports steps that auto-collapse when done (engineer density).
func (s Step) IsSafeKind() bool {
	switch s.Kind {
	case KindThink, KindPlan, KindRead, KindSearch:
		return true
	default:
		return false
	}
}

// SummaryLine is the collapsed one-line view of the step.
func (s Step) SummaryLine(width int, spinnerFrame string) string {
	glyph := s.Glyph(spinnerFrame)
	kind := string(s.Kind)
	target := s.Target
	dur := ""
	if s.Duration > 0 && (s.Status == StatusDone || s.Status == StatusFailed || s.Status == StatusDenied) {
		dur = formatDur(s.Duration)
	}

	// kind padded to 7 for column alignment
	left := fmt.Sprintf("%s %-7s %s", glyph, kind, target)
	if dur == "" {
		return truncate(left, width)
	}
	// right-align duration when space allows
	pad := width - visibleLen(left) - len(dur) - 1
	if pad < 1 {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", pad) + dur
}

func formatDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if visibleLen(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	// naive rune truncate
	r := []rune(s)
	if len(r) > width-1 {
		return string(r[:width-1]) + "…"
	}
	return s
}

func visibleLen(s string) int {
	return len([]rune(s))
}

// NeedsGate reports whether this step is waiting on human approval.
func (s Step) NeedsGate() bool {
	return s.Status == StatusBlocked && s.RequiresApproval
}
