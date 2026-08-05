# Shape brief — AI Coding Harness TUI (v2)

<!-- design pass 2026-08-05 · no code in this pass -->
<!-- world: Build Log Console × Grok Build hybrid · seed 07026972 still owns “log not chat” -->

## Status

| Field | Value |
|--------|--------|
| **Audience** | Solo / power-user engineers in-flow |
| **Mode** | Operate |
| **Stack** | Go + Bubble Tea |
| **Model** | OpenAI-compatible (`OPENAI_API_KEY` / `OPENAI_BASE_URL`) |
| **This deliverable** | Design brief + ASCII wireframes only (user confirmed) |

---

## 1. Job and audience

Engineers run a local coding agent in the terminal: prompt → watch tools → approve risk → re-steer.  
Success: at a glance know **what is running, what changed, what is blocked, how to stop**.

**Hard anti-goal:** never hide tool calls or diffs behind pretty chat.

---

## 2. Reference hybrid (what we steal / refuse)

User reference set: **Grok Build** (primary craft bar), Claude Code, Pi, Codex CLI.

| Steal from Grok Build (and peers) | Refuse / keep ours |
|-----------------------------------|--------------------|
| Full-screen scrollback + docked prompt | Not a pure chat bubble app |
| Neutral dark + **one** accent (GrokNight energy: quiet base, magenta/violet signal) | Not rainbow kind colors on every row |
| Tool blocks as first-class scrollback entries | Not “assistant said it edited a file” without body |
| Expand/collapse, mouse scroll, turn navigation feel | Not IDE chrome / sidebars in v1 |
| Permission modes that are visible and cheap to switch | Not yolo by default |
| Calm density; thinking secondary to tools | Not CI-only quiet (still need recovery trail) |

**Ours:** Build Log Console grammar — steps read as **build steps** (`run` / `ok` / `FAIL` / `WAIT`), not only as chat turns. Diffs and shell stay in the stream.

**Thesis (one line):**  
*Match Grok Build’s real TUI grammar (from screenshots): conversation scrollback, soft blocks, diamond tools + left rail, bordered bottom prompt with mode chips — tools stay visible; gates are calm amber, not a CI dashboard.*

**Visual reference pack:** `.impeccable/refs/grok-build/` (`midudev-tui.jpg`, `cn-tui.png`, `NOTES.md`).

---

## 3. Selected design decisions (this pass)

| Decision | Choice |
|----------|--------|
| Visual hybrid | **Neutral stream + single accent**; color only for run / gate / fail / allow-danger |
| Approve UX | **Stronger inline step only** (no docked gate panel, no full-screen modal) |
| Density | **Engineer default** — auto-collapse think + successful reads; keep edit/shell/error/active open |
| Layout | Vertical stack (Claude Code / Grok Build family): status → scrollback → prompt |
| Sticky gate | Thin one-line reminder **only if** the blocked step is off-screen |
| Status chrome | Minimal primary row; secondary detail only while running or on peek |

---

## 4. Visual system (intent — not final hex yet)

### Palette strategy: Restrained + one signal

| Role | Intent |
|------|--------|
| Background | Near-black / terminal default (no painted full-bleed chrome) |
| Primary text | Soft light gray |
| Dim / done | Muted gray (completed safe steps recede) |
| Accent | Single hue (magenta/violet family, GrokNight-like) for focus, links, prompt caret, running |
| Gate | Warm amber/yellow reserved **only** for WAIT/approve |
| Fail | Red reserved **only** for failed tools / errors |
| Diff | Green `+` / red `-` lines inside expanded edit bodies only |
| Allow mode | Accent flips to danger red in status so yolo is unmistakable |

**No** per-kind rainbow (`read` blue, `edit` orange, `shell` yellow as competing equals). Kind is a **label column**; status is the color.

### Type / columns

```
[g] kind   target…                                    duration
     └ body (dim, indented 2)
```

- `g` = glyph (1 cell): spinner | ✓ | ✗ | ■ | –
- `kind` = fixed width 6–7, dim caps or lowercase monospaced
- `target` = path/command, truncates with …
- `duration` = right-aligned when done

### Glyph vocabulary (build log)

| State | Glyph | Notes |
|-------|-------|--------|
| running | spinner | accent color |
| done | `✓` or `·` | dim for safe; normal for write/shell |
| failed | `✗` | red |
| blocked (WAIT) | `■` or `▶` | amber + bold row |
| denied/skip | `–` | dim |

---

## 5. Density rules (engineer default)

| Step kind | Default expand | Auto-collapse when |
|-----------|----------------|--------------------|
| user | one-line target; body = full text on expand | — |
| plan | collapsed after done | always after complete |
| think | collapsed | always when done |
| read / search / list | collapsed when OK | success |
| edit | **expanded** (diff visible) | never auto while gate/fail; after OK may collapse on next turn |
| shell | **expanded** while run/fail; summary line when OK | success + short output |
| result | expanded once; collapsible | — |
| system | one-line | — |

**Overflow:** oldest safe steps force-collapse; keep last gate/fail/edit open; `G` jumps to tail.

---

## 6. Approve moment (inline only)

When a tool needs approval:

1. Step status = **WAIT** (blocked).
2. Row is **high contrast**: amber glyph, bold target, trailing ` y/n `.
3. Body **force-expanded**: full command or diff (not a summary-only line).
4. Optional left rule / full-width underline under the step (1 row) so it scans as a build failure style halt.
5. If user scrolls away → thin sticky: `WAIT edit path · y/n · ] jump` (not a second copy of the diff).
6. `y` / `n` with empty input always apply to **first blocked** (or focused blocked if focused).

No docked multi-row gate panel. No modal overlay.

---

## 7. Topology & chrome

```
┌─ status (1 row, minimal) ─────────────────────────────────────────┐
│ ~/proj · gpt-4o-mini · ask · live                    #a1b2       │
├─ scrollback (flex) ───────────────────────────────────────────────┤
│  … steps …                                                        │
│  [sticky only if WAIT off-screen]                                 │
├─ prompt (2–3 rows) ───────────────────────────────────────────────┤
│ > _                                                               │
│   enter send · esc stop · ctrl+t perm · ?                         │
└───────────────────────────────────────────────────────────────────┘
```

**Status minimal:** `cwd · model · perm · live|demo`  
**While running add:** spinner + `run` + tokens  
**Hide by default:** long key cheat-sheets (keep one short hint under prompt; full list on `?`)

**Permission cycle:** `ctrl+t` → plan → ask → allow (match mental model of Grok Build’s cheap mode switch).

---

## 8. ASCII wireframes

### 8.1 Empty / ready

```
 ~/code/harness · gpt-4o-mini · ask · live                    #3f2a
────────────────────────────────────────────────────────────────
 Build Log · waiting for target

 Type a task. Tools and diffs show up as steps.
 Search: rg · fd    Perm: ask (ctrl+t)

────────────────────────────────────────────────────────────────
 > fix the flaky test in session_
   enter send · ctrl+t perm · ? help
```

### 8.2 Running turn (engineer density)

```
 ~/code/harness · gpt-4o-mini · ask · live  ⠋ run  1.2k↑800↓   #3f2a
────────────────────────────────────────────────────────────────
 ✓ user    fix the flaky test in session
 · plan    3 steps                                              0.4s
 · think   approach                                             0.2s
 · read    internal/session/session.go                          12ms
 ⠋ shell   go test ./internal/session/...
     $ go test ./internal/session/...
     --- FAIL: TestDemo (0.01s)
         session_test.go:12: …
────────────────────────────────────────────────────────────────
 >
   esc interrupt
```

Notes: think/plan/read collapsed (·); shell expanded with live output; status shows run + tokens.

### 8.3 APPROVE — inline hero (focal)

```
 ~/code/harness · gpt-4o-mini · ask · live                    #3f2a
────────────────────────────────────────────────────────────────
 ✓ user    fix the flaky test in session
 · plan    3 steps                                              0.4s
 · read    internal/session/session.go                          12ms
 ■ edit    internal/session/session.go  +8 −2            [y/n]
     ─────────────────────────────────────────────────────────
     --- a/internal/session/session.go
     +++ b/internal/session/session.go
     @@ -40,6 +40,10 @@
     -       return err
     +       if err != nil {
     +               return fmt.Errorf("snapshot: %w", err)
     +       }
     +       return nil
     ─────────────────────────────────────────────────────────
 · shell   go test ./internal/session/...                 pending
────────────────────────────────────────────────────────────────
 >
   y approve · n deny · ] next gate · o fold
```

Blocked edit owns the eye: glyph ■, `[y/n]`, forced diff, light rules. Next shell stays pending dim.

### 8.4 Sticky only (scrolled away from WAIT)

```
 ~/code/harness · gpt-4o-mini · ask · live                    #3f2a
────────────────────────────────────────────────────────────────
 ✓ user    …earlier steps scrolled…
 · read    …
 · read    …
 · search  TestDemo
────────────────────────────────────────────────────────────────
 WAIT edit session.go · y/n · ] jump
────────────────────────────────────────────────────────────────
 >
   y approve · n deny
```

Sticky is **one row**, not a second diff.

### 8.5 Allow (yolo) — danger visible

```
 ~/code/harness · gpt-4o-mini · ALLOW · live  ⠋ run            #3f2a
────────────────────────────────────────────────────────────────
 …
 ⠋ shell   rm -rf /tmp/harness-scratch && go test ./...
     $ rm -rf /tmp/harness-scratch && go test ./...
     ok  	…	0.3s
────────────────────────────────────────────────────────────────
 >
   esc interrupt · ctrl+t → ask
```

`ALLOW` in status uses danger styling; no y/n gates.

### 8.6 Plan mode

```
 ~/code/harness · gpt-4o-mini · plan · live                   #3f2a
────────────────────────────────────────────────────────────────
 ✓ user    redesign the approval UX
 · plan    inspect-only turn
 · read    internal/ui/model.go
 – edit    internal/ui/model.go                         denied
     (plan mode — switch to ask/allow to execute)
 ✓ result  Proposed steps: …
────────────────────────────────────────────────────────────────
 >
   ctrl+t → ask to apply
```

### 8.7 Error

```
 …
 ✗ shell   go test ./...                                        1.1s
     $ go test ./...
     FAIL	…	[exit 1]
 ✓ result  Tests failed; see shell step. Suggested fix…
────────────────────────────────────────────────────────────────
 >
```

---

## 9. Interaction map (v2 keys)

| Key | Action |
|-----|--------|
| type + enter | Send (or expand if input empty + step focused) |
| esc | Interrupt agent / clear input |
| y / n | Approve / deny blocked tool (empty input) |
| o | Toggle expand focused step |
| ↑ ↓ | Focus steps |
| [ ] | Prev/next gate or error |
| g / G | Top / bottom |
| pgup/pgdn · wheel | Scroll (follow-tail releases when scrolling up) |
| ctrl+t | Cycle plan → ask → allow |
| ? | Help overlay |
| ctrl+c | Quit |

**Later (Grok Build–inspired, not v2-required):** turn jumps, expand-all, `/theme`, vim mode.

---

## 10. States checklist

| State | Design |
|-------|--------|
| Empty | Cold build waiting for target — 3 lines max copy |
| Running | Accent spinner on active step only; tail follow |
| Gate | Inline WAIT hero + optional sticky |
| Denied | Dim `–` + one-line reason in body |
| Fail | Red `✗`, body open |
| Interrupt | System step + prompt reclaim |
| Demo vs live | Status badge only; same layout |

---

## 11. Implementation consequence (when we code)

Refactor **view layer first** (no new features):

1. Restyle palette → restrained + one accent + gate/fail  
2. Step renderer: columns, dim completed safe steps, force-expand WAIT  
3. Auto-collapse policy in session or view model  
4. Minimal status; shorter hints  
5. Diff line coloring in edit bodies  
6. Sticky only when blocked step off-screen (viewport intersection)

Keep agent/tools/LLM as-is unless view needs a field (e.g. `CollapsedPreferred`).

---

## 12. Out of scope (still)

- Multi-agent / worktree panes  
- Docked gate panel / modal gate  
- Full Grok Build clone (slash command OS, theme marketplace)  
- Streaming token paint polish (can follow later)  
- Invented product claims  

---

## 13. Confirmation

- [x] Hybrid craft: Grok Build calm + build-log spine  
- [x] Inline-only approve (stronger step)  
- [x] Engineer density  
- [x] Wireframes above are the build target  
- [x] User confirmed 2026-08-05 — implement view redesign (no feature creep)

**Open for a later pass:** theme pack / `/theme`, vim mode, turn jumps. Glyphs: `■` gate, `·` safe-done, `✓` write/shell done.
