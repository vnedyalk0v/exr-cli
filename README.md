# exr-cli

**exr** is a terminal AI coding agent harness (Grok Build–inspired TUI).

## Run

```bash
export OPENAI_API_KEY=sk-...
go run ./cmd/exr
# or
go build -o bin/exr ./cmd/exr && ./bin/exr
```

Without an API key, **exr** starts in demo mode (synthetic tools, no disk writes).

## Config

| Env | Default | Description |
|-----|---------|-------------|
| `OPENAI_API_KEY` | — | Required for live mode |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base |
| `HARNESS_MODEL` / `OPENAI_MODEL` | `gpt-4o-mini` | Model id |
| `HARNESS_PERM` | `ask` | `plan` \| `ask` \| `allow` |
| `HARNESS_DEMO` | — | Force demo mode |
| `HARNESS_WORKSPACE` | cwd | Workspace root |

## Keys

- **Enter** send · **Shift+Tab** permission mode · **y/n** approve/deny tools · **Esc** cancel · **Ctrl+x** help · **Ctrl+c** quit

## Stack

Go + Bubble Tea · OpenAI-compatible Chat Completions · tools use `rg`/`fd` when available.
