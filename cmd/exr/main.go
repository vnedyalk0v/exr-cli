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

	modelName := cfg.Model
	if !cfg.Live() {
		modelName = "demo-synthetic"
	}

	sess := session.New(modelName)
	sess.CWD = ws.Root
	sess.SetPerm(cfg.Perm)

	var turn ui.TurnFunc
	live := cfg.Live()
	if live {
		client := llm.NewClient(cfg.APIKey, cfg.BaseURL, cfg.Model)
		runner := &agent.Runner{
			Sess:      sess,
			Client:    client,
			Tools:     toolRunner,
			MaxRounds: 12,
		}
		turn = runner.RunTurn
	} else {
		demo := &agent.DemoRunner{Sess: sess}
		turn = demo.RunTurn
		fmt.Fprintln(os.Stderr, "exr: demo mode (no OPENAI_API_KEY). Export a key for live OpenAI-compatible calls.")
		fmt.Fprintf(os.Stderr, "exr: search backends: %s\n", toolRunner.BackendInfo())
	}

	m := ui.New(sess, ui.Options{
		RunTurn:  turn,
		Live:     live,
		Backends: toolRunner.BackendInfo(),
	})

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
