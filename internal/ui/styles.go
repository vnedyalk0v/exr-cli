package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// GrokNight palette (from Grok Build screenshots + binary notes).
// User panels need higher contrast than pure black (#1c1c1c vs #0a0a0a).
var (
	colorBg       = lipgloss.Color("#0a0a0a")
	colorFg       = lipgloss.Color("#e4e4e4")
	colorDim      = lipgloss.Color("#6e6e6e")
	colorMuted    = lipgloss.Color("#3a3a3a")
	colorSoft     = lipgloss.Color("#1c1c1c") // user turn panel (visible on black)
	colorBorder   = lipgloss.Color("#2e2e2e")
	colorAccent   = lipgloss.Color("#bb9af7")
	colorRail     = lipgloss.Color("#1abc9c")
	colorThinking = lipgloss.Color("#7a7a7a") // Grok: thought is quiet gray, not loud blue
	colorWarn     = lipgloss.Color("#e0af68")
	colorFail     = lipgloss.Color("#f7768e")
	colorOk       = lipgloss.Color("#9ece6a")
	colorDiffM    = lipgloss.Color("#f7768e")
	colorCursor   = lipgloss.Color("#7dcfff")
)

var (
	frameStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorFg)

	topMeta = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)

	// Grok user turn: soft gray block, padding like a chat bubble.
	userPanel = lipgloss.NewStyle().
			Background(colorSoft).
			Foreground(colorFg).
			Padding(0, 2)

	userPrefix = lipgloss.NewStyle().Foreground(colorDim).Background(colorSoft)

	toolLine = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)
	toolVerb = lipgloss.NewStyle().Foreground(colorFg).Background(colorBg)
	toolRail = lipgloss.NewStyle().Foreground(colorRail).Background(colorBg)
	toolWait = lipgloss.NewStyle().Foreground(colorWarn).Background(colorBg).Bold(true)
	toolFail = lipgloss.NewStyle().Foreground(colorFail).Background(colorBg)
	toolOk   = lipgloss.NewStyle().Foreground(colorOk).Background(colorBg)
	toolMark = lipgloss.NewStyle().Foreground(colorRail).Background(colorBg)

	thinkLabel = lipgloss.NewStyle().Foreground(colorThinking).Background(colorBg).Italic(true)
	thinkBody  = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)
	thinkRail  = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)

	resultBody = lipgloss.NewStyle().Foreground(colorFg).Background(colorBg)
	sysLine    = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)

	// Grok prompt: hairline border on pure black.
	promptBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorBg).
			Foreground(colorFg).
			Padding(0, 1)

	promptChip = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)
	promptWarn = lipgloss.NewStyle().Foreground(colorWarn).Background(colorBg)
	promptAcc  = lipgloss.NewStyle().Foreground(colorAccent).Background(colorBg)

	footerStyle = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)

	// Welcome card: hairline border on pure bg (matches Grok home card).
	welcomeCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorBg).
			Foreground(colorFg).
			Padding(1, 4)

	welcomeTitle = lipgloss.NewStyle().Foreground(colorFg).Background(colorBg).Bold(true)
	welcomeMuted = lipgloss.NewStyle().Foreground(colorDim).Background(colorBg)
	welcomeAcc   = lipgloss.NewStyle().Foreground(colorWarn).Background(colorBg)

	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorSoft).
			Foreground(colorFg).
			Padding(1, 2)

	stickyGate = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorWarn).
			Padding(0, 1)

	viewportStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorFg)
)
