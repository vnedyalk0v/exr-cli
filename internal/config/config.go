// Package config loads harness settings from the environment.
package config

import (
	"os"
	"strings"

	"github.com/vnedyalk0v/exr-cli/internal/session"
)

// Config is runtime configuration for the harness.
type Config struct {
	// APIKey for OpenAI-compatible providers (OPENAI_API_KEY).
	APIKey string
	// BaseURL defaults to https://api.openai.com/v1.
	// Point at SpaceXAI or other compatible gateways when ready:
	//   OPENAI_BASE_URL=https://api.spacexai.example/v1
	BaseURL string
	// Model is the chat model id (HARNESS_MODEL or OPENAI_MODEL).
	Model string
	// Perm is the default permission mode: plan | ask | allow.
	Perm session.PermissionMode
	// Demo forces the synthetic agent even when an API key is set.
	Demo bool
	// Workspace is the project root the agent may touch (defaults to cwd).
	Workspace string
}

// Load reads environment variables.
//
//	OPENAI_API_KEY      required for live model calls
//	OPENAI_BASE_URL     default https://api.openai.com/v1
//	HARNESS_MODEL       default gpt-4o-mini (or OPENAI_MODEL)
//	HARNESS_PERM        plan | ask | allow  (default ask)
//	HARNESS_DEMO        1/true to force synthetic demo
//	HARNESS_WORKSPACE   absolute or relative root (default cwd)
func Load() Config {
	base := strings.TrimRight(envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/")
	model := envOr("HARNESS_MODEL", "")
	if model == "" {
		model = envOr("OPENAI_MODEL", "gpt-4o-mini")
	}
	perm := session.PermAsk
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARNESS_PERM"))) {
	case "plan":
		perm = session.PermPlan
	case "allow", "yolo":
		perm = session.PermAllow
	case "ask", "":
		perm = session.PermAsk
	default:
		perm = session.PermAsk
	}
	demo := truthy(os.Getenv("HARNESS_DEMO"))
	ws, _ := os.Getwd()
	if v := os.Getenv("HARNESS_WORKSPACE"); v != "" {
		ws = v
	}
	return Config{
		APIKey:    strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL:   base,
		Model:     model,
		Perm:      perm,
		Demo:      demo,
		Workspace: ws,
	}
}

// Live reports whether a real model backend should be used.
func (c Config) Live() bool {
	return c.APIKey != "" && !c.Demo
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
