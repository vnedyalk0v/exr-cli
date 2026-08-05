# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

**Go + Bubble Tea** (Charm: bubbletea, bubbles, lipgloss). Real terminal TUI — not a browser app. Platform recorded as `web` only because Impeccable’s platform enum has no terminal value; design and delivery assume a native terminal UI (monospace grid, ANSI color, keyboard-first).

## Users

Primary users are solo or power-user software engineers running a local AI coding agent in their terminal. They are mid-task, often deep in a repo, switching between planning, watching the agent work, intervening, and resuming flow. State of mind: focused, impatient with chrome, high tolerance for density, low tolerance for ambiguity about what the agent is doing or about to do.

## Product Purpose

A local agent CLI harness (in the family of Claude Code / Aider): an AI agent that edits the user’s repository, runs shell commands, and streams tool use inside a terminal interface. Success means the engineer can trust, supervise, and steer the agent without leaving the keyboard—understanding plan, actions, diffs, and blockers at a glance.

## Positioning

Not a chat box pasted into a terminal. The product’s mechanism is a live, inspectable agent session: streaming thought and tool use, concrete file/command effects, and clear human gates—so the engineer can stay in control of an autonomous coding loop without abandoning terminal workflow.

## Operating Context

- Local machine, project directory as CWD / workspace root
- Terminal emulator (iTerm, Ghostty, Alacritty, VS Code integrated terminal, etc.)
- Repo as primary material: files, git status, shell, linters/tests
- Typical session: user prompt → agent plan/tools → file edits & commands → optional approval → continue or correct
- Adjacent tools: git, editors, language servers (outside the harness but part of the ritual)

## Capabilities and Constraints

**Confirmed intent**
- Local agent CLI surface analogous to Claude Code / Aider
- Real terminal TUI (Go + Bubble Tea), not a web mockup
- OpenAI-compatible Chat Completions (`OPENAI_API_KEY`, `OPENAI_BASE_URL`) — OpenAI subscription first; SpaceXAI later if the same protocol works with their subscription
- Real tools: read/list, find (`fd` when present), search (`rg` when present), str_replace/write, shell
- Permission modes: `plan` | `ask` | `allow` (yolo), cycled in UI with ctrl+t

**Capabilities**
- Conversational prompt and agent response stream (build-log steps)
- Visible tool calls with expandable bodies (never hide tools/diffs)
- Human gates on writes/shell in ask mode
- Session status: cwd, model, permission, tokens, live/demo

**Constraints**
- Terminal grid; keyboard-primary; mouse wheel scroll supported
- Workspace-sandboxed paths; shell still powerful in allow mode
- No fabricated customers, benchmarks, pricing, or model capabilities

**Open decisions**
- Multi-agent / worktree views (v1 is single-agent)
- Provider-specific SpaceXAI auth quirks beyond OpenAI-compatible base URL
- Exact vim-mode keybindings
- Session log persistence / replay

## Brand Commitments

Product name: **exr** (repo: exr-cli). No logo, palette, or brand voice locked yet. Terminal craft and agent-session clarity outrank decorative identity.

## Evidence on Hand

No real product screenshots, transcripts, or brand assets in the repo. Future work must not invent commercial claims. Demo content for design may be synthetic and must be labeled as such.

## Product Principles

1. **Session truth first** — Always make plain what the agent is doing, did, and wants next.
2. **Keyboard flow** — Every primary action has a key path; no dead-ends that require a mouse.
3. **Density with hierarchy** — Experts get signal-dense panes; urgency and approval must still pop.
4. **Intervention is first-class** — Stop, approve, reject, and re-steer are not afterthoughts.
5. **Terminal honesty** — Design for monospaced grids, resize, and scroll—not for browser chrome fantasies.
