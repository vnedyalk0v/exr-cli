package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vnedyalk0v/exr-cli/internal/llm"
	"github.com/vnedyalk0v/exr-cli/internal/session"
	"github.com/vnedyalk0v/exr-cli/internal/tools"
)

// Event is pushed to the UI while a turn runs.
type Event struct {
	Kind string // "tick" | "done" | "blocked" | "err"
}

// Runner is the live OpenAI-compatible agent loop.
type Runner struct {
	Sess   *session.Session
	Client *llm.Client
	Tools  *tools.Runner
	// MaxRounds caps tool loops per user turn.
	MaxRounds int
}

// systemPrompt is the agent contract (Operate / Build Log Console).
func systemPrompt(wsRoot, backends string, perm session.PermissionMode) string {
	return fmt.Sprintf(`You are an AI coding agent inside a local terminal harness (Build Log Console).
Workspace root: %s
Search backends: %s
Permission mode: %s

Rules:
- Prefer tools over guessing. Read before edit.
- Prefer read_file, search_code (ripgrep), find_files (fd) over shell for inspection.
- Use str_replace for small precise edits; write_file only for new/full files.
- run_shell for tests, builds, git — never for reading files when read_file works.
- Keep changes minimal and explain briefly after tools finish.
- Do not invent file contents. Paths are relative to the workspace.
- When done, give a short final summary with what changed.

Plan mode means you must NOT call tools that write or run shell; only inspect and propose.
`, wsRoot, backends, perm)
}

// RunTurn calls the model with tools until a final answer or cancel.
func (r *Runner) RunTurn(ctx context.Context, userMsg string, events chan<- Event) {
	r.Sess.ClearInterrupt()
	r.Sess.SetAgentRunning(true)
	defer r.Sess.SetAgentRunning(false)

	send := func(kind string) {
		// Prefer delivery for terminal states; drop only idle ticks if buffer is full.
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

	if r.Client == nil {
		r.Sess.Append(session.Step{
			Kind:   session.KindSystem,
			Status: session.StatusFailed,
			Target: "no LLM client",
			Body:   "OPENAI_API_KEY is not set. Export a key or run with HARNESS_DEMO=1.",
		})
		send("err")
		send("done")
		return
	}
	if r.MaxRounds <= 0 {
		r.MaxRounds = 12
	}

	meta, _ := r.Sess.Snapshot()
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt(r.Tools.WS.Root, r.Tools.BackendInfo(), meta.Perm)},
		{Role: llm.RoleUser, Content: userMsg},
	}
	toolDefs := tools.Definitions()

	// Plan step placeholder
	planIdx := r.Sess.Append(session.Step{
		Kind:     session.KindPlan,
		Status:   session.StatusRunning,
		Target:   "agent turn",
		Body:     "Calling model…",
		Expanded: true,
	})
	send("tick")

	for round := 0; round < r.MaxRounds; round++ {
		if ctx.Err() != nil {
			r.Sess.Update(planIdx, func(s *session.Step) {
				s.Status = session.StatusFailed
				s.Target = "interrupted"
			})
			send("done")
			return
		}

		start := time.Now()
		resp, err := r.Client.Chat(ctx, messages, toolDefs)
		if err != nil {
			interrupted := ctx.Err() != nil
			if meta, _ := r.Sess.Snapshot(); meta.Interrupted {
				interrupted = true
			}
			r.Sess.Update(planIdx, func(s *session.Step) {
				s.Status = session.StatusFailed
				s.Duration = time.Since(start)
				s.Expanded = true
				if interrupted {
					s.Target = "interrupted"
					if !strings.Contains(s.Body, "interrupted by user") {
						s.Body = appendLine(s.Body, "interrupted by user")
					}
				} else {
					s.Target = "model error"
					s.Body = err.Error()
				}
			})
			send("err")
			send("done")
			return
		}
		if resp.Usage != nil {
			r.Sess.AddTokens(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		}

		msg := resp.Choices[0].Message
		// Normalize content
		content := strings.TrimSpace(msg.Content)

		if len(msg.ToolCalls) == 0 {
			// Final answer
			r.Sess.Update(planIdx, func(s *session.Step) {
				if s.Status == session.StatusRunning {
					s.Status = session.StatusDone
					s.Duration = time.Since(start)
					s.Target = "turn plan"
					if content != "" && s.Body == "Calling model…" {
						s.Body = content
					}
					s.Expanded = false
				}
			})
			if content != "" {
				r.Sess.Append(session.Step{
					Kind:     session.KindResult,
					Status:   session.StatusDone,
					Target:   "assistant",
					Body:     content,
					Expanded: true,
				})
			} else {
				r.Sess.Append(session.Step{
					Kind:   session.KindResult,
					Status: session.StatusDone,
					Target: "assistant (empty)",
					Body:   "(model returned no text)",
				})
			}
			send("tick")
			send("done")
			return
		}

		// Record assistant message with tool calls for the API
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   content,
			ToolCalls: msg.ToolCalls,
		})

		if content != "" {
			r.Sess.Append(session.Step{
				Kind:   session.KindThink,
				Status: session.StatusDone,
				Target: "note",
				Body:   content,
			})
			send("tick")
		}

		// Update plan line once we have tool activity
		r.Sess.Update(planIdx, func(s *session.Step) {
			s.Status = session.StatusDone
			s.Duration = time.Since(start)
			s.Target = fmt.Sprintf("round %d · %d tools", round+1, len(msg.ToolCalls))
			s.Body = fmt.Sprintf("Model requested %d tool call(s).", len(msg.ToolCalls))
			s.Expanded = false
		})

		for _, tc := range msg.ToolCalls {
			if ctx.Err() != nil {
				send("done")
				return
			}
			name := tc.Function.Name
			args := tc.Function.Arguments
			target := toolTarget(name, args)
			kind := session.Kind(tools.KindFromTool(name))
			risk := tools.ToolRisk(name)
			perm := r.Sess.GetPerm()

			// Plan mode: block writes/shell entirely (auto-deny with explanation)
			if perm == session.PermPlan && risk != tools.RiskSafe {
				body := fmt.Sprintf("tool: %s\nargs: %s\n\n(blocked: permission mode is plan — switch to ask/allow to execute)", name, prettyJSON(args))
				idx := r.Sess.Append(session.Step{
					Kind:             kind,
					Status:           session.StatusDenied,
					Target:           target,
					Body:             body,
					Expanded:         true,
					RequiresApproval: true,
					ToolName:         name,
					ToolArgs:         args,
					ToolCallID:       tc.ID,
				})
				_ = idx
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Name:       name,
					Content:    "DENIED: plan mode — tool not executed. Propose the change in text or ask user to switch mode.",
				})
				send("tick")
				continue
			}

			needGate := perm == session.PermAsk && risk != tools.RiskSafe
			status := session.StatusRunning
			if needGate {
				status = session.StatusBlocked
			}
			preview := fmt.Sprintf("tool: %s\nargs:\n%s", name, prettyJSON(args))
			idx := r.Sess.Append(session.Step{
				Kind:             kind,
				Status:           status,
				Target:           target,
				Body:             preview,
				Expanded:         needGate || risk != tools.RiskSafe,
				RequiresApproval: needGate,
				ToolName:         name,
				ToolArgs:         args,
				ToolCallID:       tc.ID,
			})
			if needGate {
				send("blocked")
				if !waitApproval(ctx, r.Sess, idx) {
					// interrupted
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						ToolCallID: tc.ID,
						Name:       name,
						Content:    "INTERRUPTED: user cancelled the turn.",
					})
					send("done")
					return
				}
				st, _ := r.Sess.StepAt(idx)
				if st.Approval == session.ApprovalDenied || st.Status == session.StatusDenied {
					r.Sess.Update(idx, func(s *session.Step) {
						s.Status = session.StatusDenied
						s.Body = appendLine(s.Body, "# denied by user — not executed")
					})
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						ToolCallID: tc.ID,
						Name:       name,
						Content:    "DENIED: user rejected this tool call.",
					})
					send("tick")
					continue
				}
			}

			// Re-check cancel after approval before any side effect.
			if ctx.Err() != nil {
				send("done")
				return
			}
			if meta, _ := r.Sess.Snapshot(); meta.Interrupted {
				send("done")
				return
			}

			// Execute
			r.Sess.Update(idx, func(s *session.Step) {
				if s.Status != session.StatusDenied && s.Status != session.StatusFailed {
					s.Status = session.StatusRunning
				}
			})
			send("tick")
			t0 := time.Now()
			res := r.Tools.Exec(ctx, name, args)
			dur := time.Since(t0)
			body := res.Output
			if res.Diff != "" && !strings.Contains(body, res.Diff) {
				body = res.Diff + "\n" + body
			}

			// If user interrupted mid-exec, do not paint a false success.
			metaAfter, _ := r.Sess.Snapshot()
			if ctx.Err() != nil || metaAfter.Interrupted {
				r.Sess.Update(idx, func(s *session.Step) {
					if s.Status == session.StatusRunning || s.Status == session.StatusBlocked {
						s.Status = session.StatusFailed
						s.Duration = dur
						s.Body = appendLine(s.Body, "interrupted by user")
						s.Expanded = true
					}
				})
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Name:       name,
					Content:    "INTERRUPTED: user cancelled the turn.",
				})
				send("done")
				return
			}

			if !res.OK {
				r.Sess.Update(idx, func(s *session.Step) {
					// Do not clobber interrupt paint.
					if s.Status == session.StatusDenied {
						return
					}
					s.Status = session.StatusFailed
					s.Duration = dur
					s.Target = res.Target
					if res.Target == "" {
						s.Target = target
					}
					s.Body = body
					s.Expanded = true
				})
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Name:       name,
					Content:    "ERROR:\n" + body,
				})
			} else {
				r.Sess.Update(idx, func(s *session.Step) {
					if s.Status == session.StatusDenied || s.Status == session.StatusFailed {
						// Interrupted or already failed — keep that truth.
						return
					}
					s.Status = session.StatusDone
					s.Duration = dur
					s.Target = res.Target
					if res.Target == "" {
						s.Target = target
					}
					s.Body = body
					// keep edits expanded; collapse huge successful shell dumps
					if risk == tools.RiskShell && strings.Count(body, "\n") > 40 {
						s.Expanded = false
					} else {
						s.Expanded = risk != tools.RiskSafe || len(body) < 2000
					}
				})
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Name:       name,
					Content:    body,
				})
			}
			send("tick")
		}

		// Next model round with tool results
		// Reset plan line for next iteration visual
		planIdx = r.Sess.Append(session.Step{
			Kind:   session.KindPlan,
			Status: session.StatusRunning,
			Target: "continue",
			Body:   "Model continuing with tool results…",
		})
		send("tick")
	}

	r.Sess.Append(session.Step{
		Kind:   session.KindSystem,
		Status: session.StatusFailed,
		Target: "max rounds",
		Body:   fmt.Sprintf("Stopped after %d tool rounds. Send another message to continue.", r.MaxRounds),
		Expanded: true,
	})
	send("done")
}

func waitApproval(ctx context.Context, sess *session.Session, idx int) bool {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		// Hard stop on cancel / session interrupt before treating deny as a normal answer.
		if ctx.Err() != nil {
			sess.Update(idx, func(s *session.Step) {
				if s.Status == session.StatusBlocked {
					s.Status = session.StatusDenied
					s.Approval = session.ApprovalDenied
					s.Body = appendLine(s.Body, "interrupted by user — not executed")
				}
			})
			return false
		}
		meta, _ := sess.Snapshot()
		if meta.Interrupted {
			return false
		}
		st, ok := sess.StepAt(idx)
		if ok && st.Approval == session.ApprovalApproved {
			return true
		}
		if ok && (st.Approval == session.ApprovalDenied || st.Status == session.StatusDenied) {
			// User pressed n (not Esc): ApprovalDenied without Interrupted.
			return true
		}
		if ok && st.Status == session.StatusFailed {
			return false
		}
		select {
		case <-ctx.Done():
			sess.Update(idx, func(s *session.Step) {
				if s.Status == session.StatusBlocked {
					s.Status = session.StatusDenied
					s.Approval = session.ApprovalDenied
					s.Body = appendLine(s.Body, "interrupted by user — not executed")
				}
			})
			return false
		case <-tick.C:
		}
	}
}

// ApproveStep records human approval; the agent loop performs execution.
func ApproveStep(sess *session.Session, idx int) {
	sess.Update(idx, func(s *session.Step) {
		if s.Status != session.StatusBlocked {
			return
		}
		s.Approval = session.ApprovalApproved
		s.Status = session.StatusRunning
	})
}

// DenyStep records human denial.
func DenyStep(sess *session.Session, idx int) {
	sess.Update(idx, func(s *session.Step) {
		if s.Status != session.StatusBlocked {
			return
		}
		s.Approval = session.ApprovalDenied
		s.Status = session.StatusDenied
	})
}

func toolTarget(name, argsJSON string) string {
	var m map[string]any
	_ = json.Unmarshal([]byte(argsJSON), &m)
	switch name {
	case "read_file", "write_file", "str_replace", "list_dir":
		if p, ok := m["path"].(string); ok {
			return p
		}
	case "run_shell":
		if c, ok := m["command"].(string); ok {
			return truncate(c, 72)
		}
	case "search_code":
		if q, ok := m["query"].(string); ok {
			return "/" + truncate(q, 40) + "/"
		}
	case "find_files":
		if p, ok := m["pattern"].(string); ok {
			return p
		}
	}
	return name
}

func prettyJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(b)
}

func appendLine(body, line string) string {
	if body == "" {
		return line
	}
	return body + "\n" + line
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
