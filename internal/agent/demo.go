// Demo synthetic agent turn for offline UX (no API key).
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vnedyalk0v/exr-cli/internal/session"
	"github.com/vnedyalk0v/exr-cli/internal/strutil"
)

// runDemo is the offline synthetic turn (no API key).
func (r *Runner) runDemo(ctx context.Context, userMsg string, events chan<- Event) {
	r.Sess.MarkSynthetic()
	r.Sess.ClearInterrupt()
	r.Sess.SetAgentRunning(true)
	defer r.Sess.SetAgentRunning(false)

	send := func(kind string) {
		ev := Event{Kind: kind}
		if kind == "done" || kind == "blocked" || kind == "err" {
			select {
			case events <- ev:
			case <-ctx.Done():
			}
			return
		}
		select {
		case events <- ev:
		case <-ctx.Done():
		default:
		}
	}

	if !sleep(ctx, 200*time.Millisecond) {
		send("done")
		return
	}

	planIdx := r.Sess.Append(session.Step{
		Kind:      session.KindPlan,
		Status:    session.StatusRunning,
		Target:    "turn plan",
		Body:      planBody(userMsg),
		Expanded:  true,
		Synthetic: true,
	})
	send("tick")
	if !sleep(ctx, 400*time.Millisecond) {
		send("done")
		return
	}
	r.Sess.Update(planIdx, func(s *session.Step) {
		s.Status = session.StatusDone
		s.Duration = 400 * time.Millisecond
		s.Expanded = false
	})
	r.Sess.AddTokens(120, 40)
	send("tick")

	thinkIdx := r.Sess.Append(session.Step{
		Kind:      session.KindThink,
		Status:    session.StatusRunning,
		Target:    "approach",
		Body:      "Locate entrypoint, inspect main, propose a minimal patch, run tests if present.",
		Synthetic: true,
	})
	send("tick")
	if !sleep(ctx, 300*time.Millisecond) {
		send("done")
		return
	}
	r.Sess.Update(thinkIdx, func(s *session.Step) {
		s.Status = session.StatusDone
		s.Duration = 300 * time.Millisecond
	})
	send("tick")

	readIdx := r.Sess.Append(session.Step{
		Kind:      session.KindRead,
		Status:    session.StatusRunning,
		Target:    "cmd/exr/main.go",
		Body:      "     1|package main\n     2|\n     3|func main() {\n     4|\trun()\n     5|}\n",
		Expanded:  true,
		Synthetic: true,
	})
	send("tick")
	if !sleep(ctx, 250*time.Millisecond) {
		send("done")
		return
	}
	r.Sess.Update(readIdx, func(s *session.Step) {
		s.Status = session.StatusDone
		s.Duration = 80 * time.Millisecond
		s.Expanded = false
	})
	send("tick")

	diff := syntheticDiff()
	editIdx := r.Sess.Append(session.Step{
		Kind:             session.KindEdit,
		Status:           session.StatusBlocked,
		Target:           "cmd/exr/main.go  +3 −1",
		Body:             diff,
		Expanded:         true,
		RequiresApproval: true,
		ToolName:         "str_replace",
		Synthetic:        true,
	})
	send("blocked")

	if !waitApproval(ctx, r.Sess, editIdx) {
		send("done")
		return
	}
	st, _ := r.Sess.StepAt(editIdx)
	if st.Approval == session.ApprovalDenied || st.Status == session.StatusDenied {
		r.Sess.Update(editIdx, func(s *session.Step) {
			s.Status = session.StatusDenied
			s.Body = strutil.AppendLine(s.Body, "# denied by user — not executed")
		})
		r.Sess.Append(session.Step{
			Kind:      session.KindResult,
			Status:    session.StatusDone,
			Target:    "stopped — edit denied",
			Body:      "User denied the proposed edit. (synthetic)",
			Synthetic: true,
		})
		send("done")
		return
	}
	// synthetic apply (do not clobber interrupt)
	r.Sess.Update(editIdx, func(s *session.Step) {
		if s.Status == session.StatusDenied || s.Status == session.StatusFailed {
			return
		}
		s.Status = session.StatusDone
		s.Duration = 40 * time.Millisecond
		s.Body = strutil.AppendLine(s.Body, "\n# approved — patch applied (synthetic, no disk write)")
	})
	if meta, _ := r.Sess.Snapshot(); meta.Interrupted || ctx.Err() != nil {
		send("done")
		return
	}
	send("tick")

	shellIdx := r.Sess.Append(session.Step{
		Kind:             session.KindShell,
		Status:           session.StatusBlocked,
		Target:           "go test ./...",
		Body:             "$ go test ./...\n(synthetic command — not executed until approved)",
		Expanded:         true,
		RequiresApproval: true,
		ToolName:         "run_shell",
		Synthetic:        true,
	})
	send("blocked")

	if !waitApproval(ctx, r.Sess, shellIdx) {
		send("done")
		return
	}
	st, _ = r.Sess.StepAt(shellIdx)
	if st.Approval == session.ApprovalDenied || st.Status == session.StatusDenied {
		r.Sess.Update(shellIdx, func(s *session.Step) {
			s.Status = session.StatusDenied
			s.Body = strutil.AppendLine(s.Body, "# denied by user — not executed")
		})
		r.Sess.Append(session.Step{
			Kind:      session.KindResult,
			Status:    session.StatusDone,
			Target:    "edit applied; tests skipped",
			Body:      "Edit approved; shell denied. (synthetic)",
			Synthetic: true,
		})
		send("done")
		return
	}
	r.Sess.Update(shellIdx, func(s *session.Step) {
		if s.Status == session.StatusDenied || s.Status == session.StatusFailed {
			return
		}
		s.Status = session.StatusDone
		s.Duration = 180 * time.Millisecond
		s.Body = "$ go test ./...\nok  \tgithub.com/vnedyalk0v/exr-cli/internal/session\t0.12s\n(synthetic — not really executed)\n"
	})
	if meta, _ := r.Sess.Snapshot(); meta.Interrupted || ctx.Err() != nil {
		send("done")
		return
	}
	send("tick")

	if !sleep(ctx, 150*time.Millisecond) {
		send("done")
		return
	}

	r.Sess.Append(session.Step{
		Kind:      session.KindResult,
		Status:    session.StatusDone,
		Target:    "turn complete (synthetic)",
		Body:      fmt.Sprintf("Responded to: %q\nNo real model or filesystem writes were performed.\nSet OPENAI_API_KEY for live mode.", strutil.Truncate(userMsg, 80)),
		Expanded:  true,
		Synthetic: true,
	})
	r.Sess.AddTokens(80, 120)
	send("done")
}

func planBody(userMsg string) string {
	return strings.Join([]string{
		"1. Inspect relevant source for the request",
		"2. Propose a minimal edit (gated)",
		"3. Run tests if applicable (gated)",
		"4. Summarize result",
		"",
		"user: " + strutil.Truncate(userMsg, 120),
		"(synthetic plan — not a real model)",
	}, "\n")
}

func syntheticDiff() string {
	return strings.Join([]string{
		"--- a/cmd/exr/main.go",
		"+++ b/cmd/exr/main.go",
		"@@ -1,6 +1,8 @@",
		" package main",
		" ",
		"+// exr — Build Log Console",
		" func main() {",
		"-	run()",
		"+	// synthetic patch for demo approval",
		"+	run()",
		" }",
		"",
		"(synthetic diff — not applied until approved; demo still does not write disk)",
	}, "\n")
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
