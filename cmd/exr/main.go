// Command exr is the exr coding agent TUI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vnedyalk0v/exr-cli/internal/agent"
	"github.com/vnedyalk0v/exr-cli/internal/config"
	"github.com/vnedyalk0v/exr-cli/internal/llm"
	"github.com/vnedyalk0v/exr-cli/internal/session"
	"github.com/vnedyalk0v/exr-cli/internal/tools"
	"github.com/vnedyalk0v/exr-cli/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "exr: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	ws, err := tools.NewWorkspace(cfg.Workspace)
	if err != nil {
		return fmt.Errorf("workspace: %w", err)
	}
	toolRunner := tools.NewRunner(ws)

	live := cfg.Live()
	modelName := cfg.Model
	if !live {
		modelName = "demo-synthetic"
		fmt.Fprintln(os.Stderr, "exr: demo mode (no OPENAI_API_KEY). Export a key for live OpenAI-compatible calls.")
		fmt.Fprintf(os.Stderr, "exr: search backends: %s\n", toolRunner.BackendInfo())
	}

	sess := session.New(modelName)
	sess.CWD = ws.Root
	sess.SetPerm(cfg.Perm)

	runner := &agent.Runner{Sess: sess, Tools: toolRunner, MaxRounds: 12}
	if live {
		runner.Client = llm.NewClient(cfg.APIKey, cfg.BaseURL, cfg.Model)
	}

	m := ui.New(sess, runner.RunTurn, live, toolRunner.BackendInfo())
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
