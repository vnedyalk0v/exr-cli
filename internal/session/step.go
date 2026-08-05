// Package session holds the build-log model for an agent session.
package session

import "time"

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
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
	StatusBlocked Status = "blocked"
	StatusSkipped Status = "skipped"
	StatusDenied  Status = "denied"
)

// PermissionMode controls how tools are gated.
type PermissionMode string

const (
	PermPlan  PermissionMode = "plan"
	PermAsk   PermissionMode = "ask"
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
type Step struct {
	ID               string
	Kind             Kind
	Status           Status
	Target           string
	Body             string
	Duration         time.Duration
	Expanded         bool
	RequiresApproval bool
	Approval         Approval
	ToolName         string
	ToolArgs         string
	ToolCallID       string
	Synthetic        bool
}

// Glyph returns a single-cell status marker for the build log.
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

// NeedsGate reports whether this step is waiting on human approval.
func (s Step) NeedsGate() bool {
	return s.Status == StatusBlocked && s.RequiresApproval
}
