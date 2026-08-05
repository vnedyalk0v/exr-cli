# Grok Build TUI — visual notes (from real screenshots)

Sources (local copies in this folder):
- `midudev-tui.jpg` — welcome / home screen
- `cn-tui.png` — active session (user turn, tools, thinking, prompt)
- `review-help.jpeg` — shortcuts overlay + help turn
- Binary palette: GrokNight bg `#0a0a0a`, accent magenta `#bb9af7`, body text ~`#e1e1e1` / `#eeeeee`

## Layout grammar

```
[optional dim branch + cwd top-left]              [context / tokens top-right]

  (scrollback — lots of air)

  > user message in soft gray panel                    time

  ◆ Thought for Ns   (dim italic secondary)
  assistant prose

  │ ◆ Run …
  │ ◆ Searched …
  │ ◆ Read …          ← green/cyan left rail groups tools
  │ ◆ Edit …

  ┌─ prompt box (thin border, full width) ─────────────────────┐
  │ > █                                         model · mode   │
  └────────────────────────────────────────────────────────────┘
  footer keys dim: Shift+Tab:mode | Esc:… | Ctrl+x:shortcuts
```

## What it is NOT

- No full-width loud brand bar (`◆ harness`)
- No make/cargo column table as the primary chrome
- No heavy amber full-row WAIT wash (approval is calmer, still clear)
- No double horizontal rules painting the whole frame
- Not “CI log first” — **conversation first, tools as quiet blocks**

## Steal list for our harness

1. Near-black flat field (`#0a0a0a`), soft grays for secondary text
2. Magenta `#bb9af7` as *sparse* accent (prompt caret, focus), not every label
3. User turns = soft panel + `>`
4. Tools = `◆` + short verb line + optional left rail
5. Thinking = left border + dim “Thought for …”
6. Prompt = bordered box; **status chips inside bottom-right of the box**
7. Footer hint row outside the box
8. Welcome = centered soft card, not a marketing dump
