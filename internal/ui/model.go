package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vnedyalk0v/exr-cli/internal/agent"
	"github.com/vnedyalk0v/exr-cli/internal/session"
)

const (
	topMetaHeight = 1
	promptBoxRows = 3 // rounded border box ≈ 3 rows
	footerHeight  = 1
	stickyHeight  = 1
)

// TurnFunc runs one agent turn (demo or live).
type TurnFunc func(ctx context.Context, userMsg string, events chan<- agent.Event)

// Model is the Grok Build–inspired session TUI.
type Model struct {
	sess       *session.Session
	runTurn    TurnFunc
	live       bool
	backends   string
	input      textinput.Model
	vp         viewport.Model
	spin       spinner.Model
	width      int
	height     int
	ready      bool
	showHelp   bool
	cursor     int
	followTail bool
	stepOffset []int
	stepLines  []int
	cancel     context.CancelFunc
	evCh       chan agent.Event
	// turning is set synchronously in submit so Esc→re-submit cannot overlap turns.
	turning bool
	// spinFrame cached so non-running renders stay stable
	spinFrame string
}

// Options configure the UI.
type Options struct {
	RunTurn  TurnFunc
	Live     bool
	Backends string
}

// New constructs the session UI.
func New(sess *session.Session, opt Options) Model {
	ti := textinput.New()
	// Grok Build: "> " in prompt; pure bg (no soft fill — that caused the gray bar).
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorFg).Background(colorBg).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg).Background(colorBg)
	ti.Placeholder = ""
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorCursor).Background(colorBg)
	ti.Cursor.TextStyle = lipgloss.NewStyle().Foreground(colorBg).Background(colorCursor)
	ti.CharLimit = 0
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return Model{
		sess:       sess,
		runTurn:    opt.RunTurn,
		live:       opt.Live,
		backends:   opt.Backends,
		input:      ti,
		spin:       sp,
		cursor:     -1,
		followTail: true,
		evCh:       make(chan agent.Event, 32),
		spinFrame:  "•",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

type agentEventMsg agent.Event
type agentDoneMsg struct{}

func waitEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return agentEventMsg(ev)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.ready && !m.showHelp {
			ev := tea.MouseEvent(msg)
			// Ignore pure motion (WithMouseCellMotion floods these).
			if !ev.IsWheel() && ev.Action == tea.MouseActionMotion {
				return m, nil
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			newY := m.vp.YOffset
			m.followTail = m.vp.AtBottom()
			// Wheel scrolls viewport only — no full content rebuild.
			if ev.IsWheel() {
				return m, cmd
			}
			m.rebuildViewportPreserving(newY)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "q", "enter", "ctrl+x":
				m.showHelp = false
			}
			return m, nil
		}

		meta, steps := m.sess.Snapshot()
		blocked := m.sess.FirstBlocked()
		inputEmpty := strings.TrimSpace(m.input.Value()) == ""

		switch msg.String() {
		case "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit

		case "esc":
			if meta.AgentRunning || m.turning {
				if m.cancel != nil {
					m.cancel()
				}
				m.sess.Interrupt()
				m.sess.Append(session.Step{
					Kind:   session.KindSystem,
					Status: session.StatusDone,
					Target: "interrupted",
					Body:   "Agent stop requested (Esc).",
				})
				m.rebuildViewport()
				return m, nil
			}
			m.input.SetValue("")
			return m, nil

		case "?", "ctrl+x":
			if inputEmpty {
				m.showHelp = true
				return m, nil
			}

		case "shift+tab", "ctrl+t":
			// Mode cycle always available (Grok Build Shift+Tab); not a printable conflict.
			p := m.sess.CyclePerm()
			m.sess.Append(session.Step{
				Kind:   session.KindSystem,
				Status: session.StatusDone,
				Target: "mode → " + string(p),
				Body:   permHelp(p),
			})
			m.followTail = true
			m.rebuildViewport()
			return m, nil

		case "y":
			// Steal y only when gated and input empty (composing "yes…" still needs a non-empty buffer first).
			if inputEmpty && blocked >= 0 {
				agent.ApproveStep(m.sess, blocked)
				m.cursor = blocked
				m.rebuildViewport()
				return m, nil
			}

		case "n":
			if inputEmpty && blocked >= 0 {
				agent.DenyStep(m.sess, blocked)
				m.cursor = blocked
				m.rebuildViewport()
				return m, nil
			}

		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val != "" {
				return m.submit(val)
			}
			// Empty enter: expand only in nav mode (cursor focused).
			if m.cursor >= 0 && m.cursor < len(steps) {
				m.sess.ToggleExpand(m.cursor)
				m.rebuildViewport()
				return m, nil
			}

		case "o":
			// Navigation-mode only so "ok …" can be typed with empty→o conflict avoided
			// when cursor is on a step (user chose nav). When cursor < 0, pass to input.
			if inputEmpty && m.cursor >= 0 && m.cursor < len(steps) {
				m.sess.ToggleExpand(m.cursor)
				m.rebuildViewport()
				return m, nil
			}

		case "[":
			// Only steal when a prev gate/fail exists — otherwise type '[' into the prompt.
			if inputEmpty {
				from := m.cursor
				if from < 0 {
					from = 0
				}
				if ni := m.sess.NextBlockedFrom(from, -1); ni >= 0 {
					m.cursor = ni
					m.followTail = false
					m.rebuildViewport()
					m.scrollToCursor()
					return m, nil
				}
			}

		case "]":
			if inputEmpty {
				from := m.cursor
				if from < 0 {
					from = len(steps) - 1
					if from < 0 {
						from = 0
					}
				}
				if ni := m.sess.NextBlockedFrom(from, 1); ni >= 0 {
					m.cursor = ni
					m.followTail = false
					m.rebuildViewport()
					m.scrollToCursor()
					return m, nil
				}
			}

		case "g":
			// Only when navigating steps — free "go …" prompts when cursor is tail.
			if inputEmpty && m.cursor >= 0 {
				m.followTail = false
				m.cursor = 0
				m.vp.GotoTop()
				return m, nil
			}

		case "G":
			if inputEmpty && m.cursor >= 0 {
				m.followTail = true
				m.cursor = -1
				m.vp.GotoBottom()
				return m, nil
			}

		case "up", "ctrl+p":
			if inputEmpty && len(steps) > 0 {
				if m.cursor < 0 {
					m.cursor = len(steps) - 1
				} else if m.cursor > 0 {
					m.cursor--
				}
				m.followTail = false
				m.rebuildViewport()
				m.scrollToCursor()
				return m, nil
			}

		case "down", "ctrl+n":
			if inputEmpty && len(steps) > 0 {
				if m.cursor >= 0 && m.cursor < len(steps)-1 {
					m.cursor++
					m.followTail = false
				} else if m.cursor == len(steps)-1 {
					m.cursor = -1
					m.followTail = true
				}
				m.rebuildViewport()
				if m.followTail {
					m.vp.GotoBottom()
				} else {
					m.scrollToCursor()
				}
				return m, nil
			}

		case "pgup":
			m.vp.LineUp(max(1, m.vp.Height/2))
			m.followTail = false
			return m, nil

		case "pgdown":
			m.vp.LineDown(max(1, m.vp.Height/2))
			if m.vp.AtBottom() {
				m.followTail = true
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.layout()
		m.rebuildViewport()
		if m.followTail {
			m.vp.GotoBottom()
		}
		m.ready = true
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		m.spinFrame = m.spin.View()
		if meta, _ := m.sess.Snapshot(); meta.AgentRunning {
			m.rebuildViewport()
			if m.followTail {
				m.vp.GotoBottom()
			}
		}
		return m, cmd

	case agentEventMsg:
		m.rebuildViewport()
		if m.followTail {
			m.vp.GotoBottom()
		}
		return m, waitEvent(m.evCh)

	case agentDoneMsg:
		m.turning = false
		m.rebuildViewport()
		if m.followTail {
			m.vp.GotoBottom()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) submit(val string) (tea.Model, tea.Cmd) {
	meta, _ := m.sess.Snapshot()
	if meta.AgentRunning || m.turning || m.runTurn == nil {
		return m, nil
	}
	m.input.SetValue("")
	m.sess.Append(session.Step{
		Kind:      session.KindUser,
		Status:    session.StatusDone,
		Target:    val,
		Body:      val,
		Synthetic: !m.live,
	})
	m.followTail = true
	m.cursor = -1
	m.turning = true
	m.rebuildViewport()
	m.vp.GotoBottom()

	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// Capture channel locally so a later submit cannot steal close().
	ch := make(chan agent.Event, 32)
	m.evCh = ch
	run := m.runTurn
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Surface as a session step on next snapshot path via system line is ideal;
				// at minimum never double-close or panic the process.
				_ = r
			}
			close(ch)
		}()
		run(ctx, val, ch)
	}()
	return m, tea.Batch(m.spin.Tick, waitEvent(ch))
}

func (m *Model) layout() {
	w := max(20, m.width)
	// Prompt geometry (must fit terminal width w):
	//   total = border(2) + padding(2) + content
	//   content = fieldView + pad + chips
	//   fieldView ≈ Prompt(2) + cursor(1) + Width  (when focused)
	// chipReserve covers "model(16) · always-approve(14)" + spaces ≈ 36
	chipReserve := 0
	if w >= 52 {
		chipReserve = 36
	}
	// contentW = w - 4; field budget = contentW - chipReserve - minPad(1)
	// Width = fieldBudget - prompt(2) - cursor(1)
	contentW := max(8, w-4)
	fieldBudget := contentW - chipReserve - 1
	if fieldBudget < 6 {
		fieldBudget = max(4, contentW/2)
	}
	m.input.Width = max(2, fieldBudget-3)

	chrome := topMetaHeight + promptBoxRows + footerHeight
	// Stable chrome: reserve sticky row whenever any gate exists (avoids height thrash).
	if m.sess.FirstBlocked() >= 0 {
		chrome += stickyHeight
	}
	vh := m.height - chrome
	if vh < 3 {
		vh = 3
	}
	if !m.ready {
		m.vp = viewport.New(w, vh)
		m.vp.MouseWheelEnabled = true
	}
	m.vp.Width = w
	m.vp.Height = vh
	m.vp.Style = viewportStyle
}

func (m *Model) rebuildViewport() {
	if m.width == 0 {
		return
	}
	m.layout()
	content, offsets, lines := m.renderStream()
	m.stepOffset = offsets
	m.stepLines = lines
	y := m.vp.YOffset
	m.vp.SetContent(content)
	if m.followTail {
		m.vp.GotoBottom()
	} else {
		m.vp.SetYOffset(y)
	}
}

func (m *Model) rebuildViewportPreserving(y int) {
	if m.width == 0 {
		return
	}
	m.layout()
	content, offsets, lines := m.renderStream()
	m.stepOffset = offsets
	m.stepLines = lines
	m.vp.SetContent(content)
	if m.followTail {
		m.vp.GotoBottom()
	} else {
		m.vp.SetYOffset(y)
	}
}

func (m *Model) scrollToCursor() {
	if m.cursor < 0 || m.cursor >= len(m.stepOffset) {
		return
	}
	m.vp.SetYOffset(max(0, m.stepOffset[m.cursor]))
}

func (m Model) showStickyGate() bool {
	bi := m.sess.FirstBlocked()
	if bi < 0 || !m.ready {
		return false
	}
	if bi >= len(m.stepOffset) {
		// Offsets not built yet; don't reserve sticky (WAIT is usually near tail).
		return false
	}
	return m.stepOffScreen(bi)
}

func (m Model) stepOffScreen(i int) bool {
	if i < 0 || i >= len(m.stepOffset) {
		return true
	}
	start := m.stepOffset[i]
	h := 1
	if i < len(m.stepLines) && m.stepLines[i] > 0 {
		h = m.stepLines[i]
	}
	end := start + h
	top := m.vp.YOffset
	bottom := top + m.vp.Height
	return end <= top || start >= bottom
}

func (m Model) View() string {
	w := max(20, m.width)
	h := max(8, m.height)
	if !m.ready {
		return frameStyle.Width(w).Height(h).Render("\n  …")
	}
	if m.showHelp {
		return frameStyle.Width(w).Height(h).Render(m.renderHelp())
	}

	meta, _ := m.sess.Snapshot()
	top := m.renderTop(meta)
	stream := m.vp.View()

	// Always reserve a sticky row when gated: show WAIT text if off-screen, else blank spacer.
	var sticky string
	if bi := m.sess.FirstBlocked(); bi >= 0 {
		_, steps := m.sess.Snapshot()
		if bi < len(steps) && m.showStickyGate() {
			st := steps[bi]
			sticky = stickyGate.Width(w).Render(
				fmt.Sprintf(" WAIT  %s  %s   y/n · ] jump ", st.Kind, truncateRunes(st.Target, max(6, w-40))),
			)
		} else {
			sticky = frameStyle.Width(w).Render("")
		}
	}

	prompt := m.renderPromptBox(meta)
	footer := m.renderFooter(meta)

	parts := []string{top, stream}
	if sticky != "" {
		parts = append(parts, sticky)
	}
	parts = append(parts, prompt, footer)
	ui := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return frameStyle.Width(w).Height(h).Render(ui)
}

func (m Model) renderTop(meta session.Meta) string {
	w := max(20, m.width)
	cwd := meta.ShortCWD()
	if lipgloss.Width(cwd) > w/2 {
		cwd = filepath.Base(meta.CWD)
	}
	// Grok: dim path top-left (branch omitted until we wire git).
	left := topMeta.Render("  " + cwd)

	var rightBits []string
	if meta.AgentRunning {
		rightBits = append(rightBits, m.spinFrame+" running")
	}
	if meta.TokensIn+meta.TokensOut > 0 {
		rightBits = append(rightBits, fmt.Sprintf("%d↑%d↓", meta.TokensIn, meta.TokensOut))
	}
	right := ""
	if len(rightBits) > 0 {
		right = topMeta.Render(strings.Join(rightBits, " · ") + "  ")
	}
	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return frameStyle.Width(w).Render(left + strings.Repeat(" ", pad) + right)
}

func (m Model) promptChips(meta session.Meta) string {
	w := max(20, m.width)
	if w < 52 {
		return ""
	}
	perm := string(meta.Perm)
	permStyle := promptChip
	switch meta.Perm {
	case session.PermAllow:
		perm = "always-approve"
		permStyle = promptWarn
	case session.PermPlan:
		permStyle = promptAcc
	case session.PermAsk:
		perm = "ask"
		permStyle = promptWarn // amber like Grok mode chip
	}
	model := truncateRunes(meta.Model, 16)
	if !m.live {
		if model == "" || model == "demo-synthetic" {
			model = "demo"
		}
	}
	// Prefer shorter perm label if still tight for very long models.
	chips := promptChip.Render(model) + promptChip.Render(" · ") + permStyle.Render(perm)
	if lipgloss.Width(chips) > 38 && meta.Perm == session.PermAllow {
		perm = "allow"
		chips = promptChip.Render(model) + promptChip.Render(" · ") + permStyle.Render(perm)
	}
	return chips
}

func (m Model) renderPromptBox(meta session.Meta) string {
	w := max(20, m.width)
	// lipgloss Width = content+padding; borders add 2 outside → use w-2 for total = w.
	boxInner := max(8, w-2)
	// padding (0,1) eats 2 of boxInner; text content budget:
	contentW := max(6, boxInner-2)

	field := m.input.View()
	chips := m.promptChips(meta)
	fieldW := lipgloss.Width(field)
	chipW := lipgloss.Width(chips)
	pad := contentW - fieldW - chipW
	if pad < 1 {
		// Shrink chips first, then pad.
		if chipW > 0 && fieldW+1 < contentW {
			// keep chips; force pad 1 by accepting slight overflow only if needed
			pad = 1
			if fieldW+chipW+1 > contentW {
				// drop model, keep perm only
				permOnly := m.promptPermOnly(meta)
				if lipgloss.Width(permOnly)+fieldW+1 <= contentW {
					chips = permOnly
					chipW = lipgloss.Width(chips)
					pad = contentW - fieldW - chipW
				} else {
					chips = ""
					pad = max(1, contentW-fieldW)
				}
			}
		} else {
			chips = ""
			pad = max(1, contentW-fieldW)
		}
	}
	if pad < 1 {
		pad = 1
	}
	line := field + strings.Repeat(" ", pad) + chips
	if lw := lipgloss.Width(line); lw < contentW {
		line += strings.Repeat(" ", contentW-lw)
	} else if lw > contentW {
		// hard clip visual overflow (safety)
		line = truncateRunes(stripANSI(line), contentW)
	}
	return promptBorder.Width(boxInner).Render(line)
}

func (m Model) promptPermOnly(meta session.Meta) string {
	perm := string(meta.Perm)
	st := promptChip
	switch meta.Perm {
	case session.PermAllow:
		perm = "allow"
		st = promptWarn
	case session.PermPlan:
		st = promptAcc
	case session.PermAsk:
		perm = "ask"
		st = promptWarn
	}
	return st.Render(perm)
}

func (m Model) renderFooter(meta session.Meta) string {
	w := max(20, m.width)
	// Grok footer: pipe separators, sparse key list under the prompt.
	var text string
	switch {
	case m.sess.FirstBlocked() >= 0 && strings.TrimSpace(m.input.Value()) == "":
		text = "  y:approve  |  n:deny  |  Esc:cancel  |  Shift+Tab:mode"
	case meta.AgentRunning || m.turning:
		text = "  Esc:cancel  |  Ctrl+c:quit"
	default:
		text = "  Enter:send  |  Shift+Tab:mode  |  Ctrl+x:help"
	}
	return footerStyle.Width(w).Render(truncateRunes(text, w))
}

func (m *Model) renderStream() (content string, offsets []int, lines []int) {
	meta, steps := m.sess.Snapshot()
	if len(steps) == 0 {
		return m.renderWelcome(meta), nil, nil
	}

	var b strings.Builder
	offsets = make([]int, len(steps))
	lines = make([]int, len(steps))
	w := max(20, m.width)
	line := 0

	for i, st := range steps {
		offsets[i] = line
		focused := i == m.cursor
		block := ensureNL(m.renderStep(st, focused, w))
		// Grok scrollback air: blank line after every block (not only user/result).
		block += "\n"
		b.WriteString(block)
		n := countLines(block)
		if n < 1 {
			n = 1
		}
		lines[i] = n
		line += n
	}
	return b.String(), offsets, lines
}

func (m Model) renderWelcome(meta session.Meta) string {
	// Match Grok Build home: centered soft card, amber callout, tip under card, open air.
	w := max(20, m.width)
	cardW := min(72, max(40, w*2/3))
	if cardW > w-4 {
		cardW = max(28, w-4)
	}

	backend := "demo (no API key)"
	if m.live {
		backend = "live · OpenAI-compatible"
	}
	search := m.backends
	if search == "" {
		search = "rg · fd"
	}

	// Menu rows with right-aligned hints (Grok home list energy).
	row := func(label, hint string) string {
		lw := lipgloss.Width(label)
		hw := lipgloss.Width(hint)
		pad := cardW - 10 - lw - hw // padding/border slack
		if pad < 2 {
			pad = 2
		}
		return welcomeTitle.Render(label) + strings.Repeat(" ", pad) + welcomeMuted.Render(hint)
	}

	body := strings.Join([]string{
		welcomeTitle.Render("exr") + welcomeMuted.Render("  agent"),
		"",
		welcomeAcc.Render("Ready to build"),
		welcomeMuted.Render("Type a task below. Tools stream into the scrollback."),
		"",
		row("Mode  "+string(meta.Perm), "Shift+Tab"),
		row("Search  "+search, ""),
		row("Backend  "+backend, ""),
		"",
		row("Approve / deny tools", "y / n"),
		row("Keyboard help", "Ctrl+x"),
	}, "\n")

	card := welcomeCard.Width(cardW).Render(body)
	tip := welcomeMuted.Render("Tip: Press Shift+Tab to cycle plan · ask · always-approve.")

	// Vertical placement: card in upper-middle, tip under it (like Grok), rest empty → prompt.
	inner := lipgloss.JoinVertical(lipgloss.Left, card, "", tip)
	placed := lipgloss.Place(w, max(12, m.height/2), lipgloss.Center, lipgloss.Center, inner)
	return "\n" + placed
}

func (m Model) renderStep(st session.Step, focused bool, width int) string {
	switch st.Kind {
	case session.KindUser:
		return m.renderUser(st, width)
	case session.KindThink, session.KindPlan:
		return m.renderThink(st, focused, width)
	case session.KindResult:
		return m.renderResult(st, width)
	case session.KindSystem:
		return sysLine.Render("  · "+st.Target) + "\n"
	default:
		return m.renderTool(st, focused, width)
	}
}

func (m Model) renderUser(st session.Step, width int) string {
	text := st.Target
	if text == "" {
		text = st.Body
	}
	// Grok: soft gray full-width turn panel with "> " prefix.
	innerW := max(10, width-6)
	var lines []string
	for i, ln := range wrapLine(text, innerW) {
		if i == 0 {
			lines = append(lines, userPrefix.Render("> ")+lipgloss.NewStyle().Foreground(colorFg).Background(colorSoft).Render(ln))
		} else {
			lines = append(lines, userPrefix.Render("  ")+lipgloss.NewStyle().Foreground(colorFg).Background(colorSoft).Render(ln))
		}
	}
	if len(lines) == 0 {
		lines = []string{userPrefix.Render("> ")}
	}
	block := strings.Join(lines, "\n")
	// Leave 1-col margin so panel doesn't glue to terminal edge.
	panel := userPanel.Width(max(10, width-2)).Render(block)
	return " " + panel + "\n"
}

func (m Model) renderThink(st session.Step, focused bool, width int) string {
	open := bodyOpen(st, focused)
	// Grok: quiet "Thought for 5.8s" — not a bright ◆ Plan badge.
	var label string
	name := "Thought"
	if st.Kind == session.KindPlan {
		name = "Plan"
	}
	switch {
	case st.Status == session.StatusRunning:
		label = m.spinFrame + " " + name + "…"
	case st.Duration > 0:
		label = fmt.Sprintf("◆ %s for %s", name, formatDur(st.Duration))
	default:
		label = "◆ " + name
	}
	var b strings.Builder
	b.WriteString(thinkLabel.Render("  "+label) + "\n")
	if open && st.Body != "" {
		for _, ln := range strings.Split(st.Body, "\n") {
			for _, wln := range wrapLine(ln, max(12, width-6)) {
				// Soft left rule, not a loud colored pipe.
				b.WriteString(thinkRail.Render("  │ ") + thinkBody.Render(wln) + "\n")
			}
		}
	}
	return b.String()
}

func (m Model) renderTool(st session.Step, focused bool, width int) string {
	open := bodyOpen(st, focused)
	verb := toolVerbLabel(st)
	mark := "◆"
	if st.Status == session.StatusRunning {
		mark = m.spinFrame
	}

	target := truncateRunes(st.Target, max(8, width-32))
	// Grok tool lines: thin teal rail + diamond + quiet label (not rail on every body line).
	rail := toolRail.Render(" │")
	var head string
	switch {
	case st.NeedsGate():
		head = rail + " " + toolWait.Render(mark+" "+verb+"  ") +
			toolWait.Render(target) + toolWait.Render("  y/n")
	case st.Status == session.StatusFailed:
		head = rail + " " + toolFail.Render(mark+" "+verb+"  ") + toolFail.Render(target)
	case st.Status == session.StatusDenied:
		head = rail + " " + toolLine.Render("– "+verb+"  "+target+"  (denied)")
	case st.Status == session.StatusDone:
		// Duration dim on the right of the summary, Grok-compact.
		head = rail + " " + toolMark.Render(mark) + toolLine.Render(" "+verb+"  ") + toolLine.Render(target)
		if st.Duration > 0 {
			head += toolLine.Render("  " + formatDur(st.Duration))
		}
	default:
		head = rail + " " + toolVerb.Render(mark+" "+verb+"  ") + toolLine.Render(target)
	}
	if focused {
		head = lipgloss.NewStyle().Background(colorSoft).Width(width).Render(head)
	}

	var b strings.Builder
	b.WriteString(head + "\n")
	if open && st.Body != "" {
		body := renderBody(st, max(12, width-6))
		for _, ln := range strings.Split(body, "\n") {
			// Indent body only — continuous green rail on every line was the main "not Grok" tell.
			b.WriteString("    " + ln + "\n")
		}
	}
	return b.String()
}

func (m Model) renderResult(st session.Step, width int) string {
	text := st.Body
	if text == "" {
		text = st.Target
	}
	if text == "" {
		return resultBody.Render("  (empty result)") + "\n"
	}
	// Assistant prose: plain body with a little left margin (Grok final text, not a tool).
	var b strings.Builder
	for _, ln := range strings.Split(text, "\n") {
		for _, wln := range wrapLine(ln, max(12, width-4)) {
			b.WriteString(resultBody.Render("  "+wln) + "\n")
		}
	}
	return b.String()
}

func (m Model) renderHelp() string {
	mode := "demo"
	if m.live {
		mode = "live"
	}
	body := fmt.Sprintf(`
exr — keys

  Mode: %s · search: %s

  Enter           send
  Esc             cancel agent / clear
  y / n           approve / deny tool
  Shift+Tab       cycle plan → ask → allow
  o               expand / fold step
  ↑ ↓             focus steps
  [ ]             prev / next gate
  Ctrl+x / ?      this help
  Ctrl+c          quit

Scrollback · bordered prompt · chips · footer
`, mode, m.backends)
	box := helpStyle.Width(min(72, max(24, m.width-4))).Render(strings.TrimSpace(body))
	return lipgloss.Place(max(20, m.width), max(8, m.height), lipgloss.Center, lipgloss.Center, box)
}
