package tools

import "github.com/vnedyalk0v/exr-cli/internal/llm"

// Definitions returns OpenAI tool schemas for the coding agent.
func Definitions() []llm.ToolDef {
	return []llm.ToolDef{
		fn("read_file", "Read a UTF-8 text file under the workspace. Prefer this over shell cat.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path relative to workspace root (or absolute inside workspace)",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-based start line (optional)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max lines to return (optional, default 400)",
				},
			},
			"required": []string{"path"},
		}),
		fn("list_dir", "List files and directories at a path under the workspace.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path relative to workspace (default .)",
				},
			},
		}),
		fn("find_files", "Find files by glob/name. Uses fd when available (faster than find), else Go walk.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob or substring, e.g. '*.go' or 'main'",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Subdirectory to search (default .)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max results (default 50)",
				},
			},
			"required": []string{"pattern"},
		}),
		fn("search_code", "Search file contents with regex. Uses ripgrep (rg) when available, else a Go scanner.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Regex or fixed string to search for",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory to search (default .)",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "Optional file glob filter, e.g. '*.go'",
				},
				"case_insensitive": map[string]any{
					"type": "boolean",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max matching lines (default 40)",
				},
			},
			"required": []string{"query"},
		}),
		fn("str_replace", "Replace exactly one occurrence of old_string with new_string in a file. Fails if not unique or missing.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type": "string",
				},
				"old_string": map[string]any{
					"type": "string",
				},
				"new_string": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		}),
		fn("write_file", "Create or overwrite a whole file with contents. Prefer str_replace for small edits.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type": "string",
				},
				"contents": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"path", "contents"},
		}),
		fn("run_shell", "Run a shell command in the workspace. Prefer read_file/search_code/find_files when they fit. Captures stdout/stderr.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command string passed to the user shell",
				},
				"timeout_sec": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (default 60, max 300)",
				},
			},
			"required": []string{"command"},
		}),
	}
}

func fn(name, desc string, params map[string]any) llm.ToolDef {
	return llm.ToolDef{
		Type: "function",
		Function: llm.ToolDefFunction{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}
