package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/vnedyalk0v/exr-cli/internal/strutil"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result is the outcome of a tool call.
type Result struct {
	OK     bool
	Output string
	Diff   string
	Target string
}

// Runner executes tools inside a workspace.
type Runner struct {
	WS     *Workspace
	HasRG  bool
	HasFD  bool
	Shell  string
	MaxOut int
}

// NewRunner detects rg/fd and configures the executor.
func NewRunner(ws *Workspace) *Runner {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return &Runner{
		WS:     ws,
		HasRG:  lookPath("rg"),
		HasFD:  lookPath("fd"),
		Shell:  shell,
		MaxOut: 200_000,
	}
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// BackendInfo describes which search binaries are active.
func (r *Runner) BackendInfo() string {
	var parts []string
	if r.HasRG {
		parts = append(parts, "rg")
	} else {
		parts = append(parts, "search:go")
	}
	if r.HasFD {
		parts = append(parts, "fd")
	} else {
		parts = append(parts, "find:go")
	}
	return strings.Join(parts, " · ")
}

// Exec runs a named tool with JSON args.
func (r *Runner) Exec(ctx context.Context, name, argsJSON string) Result {
	args, err := parseArgs(argsJSON)
	if err != nil {
		return Result{OK: false, Output: "invalid JSON arguments: " + err.Error(), Target: name}
	}
	switch name {
	case "read_file":
		return r.readFile(args)
	case "list_dir":
		return r.listDir(args)
	case "find_files":
		return r.findFiles(ctx, args)
	case "search_code":
		return r.searchCode(ctx, args)
	case "str_replace":
		return r.strReplace(args)
	case "write_file":
		return r.writeFile(args)
	case "run_shell":
		return r.runShell(ctx, args)
	default:
		return Result{OK: false, Output: "unknown tool: " + name, Target: name}
	}
}

func parseArgs(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	return args, nil
}

func strArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		i, err := strconv.Atoi(t)
		if err != nil {
			return def
		}
		return i
	default:
		return def
	}
}

func boolArg(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	default:
		return false
	}
}

func (r *Runner) readFile(args map[string]any) Result {
	path := strArg(args, "path")
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: path}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	if isBinary(data) {
		return Result{OK: false, Output: "binary file (refusing to read as text)", Target: r.WS.Rel(abs)}
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	offset := intArg(args, "offset", 1)
	limit := intArg(args, "limit", 400)
	if offset < 1 {
		offset = 1
	}
	if limit < 1 {
		limit = 400
	}
	if offset > len(lines) {
		return Result{OK: false, Output: "offset past end of file", Target: r.WS.Rel(abs)}
	}
	start := offset - 1
	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%6d|%s\n", i+1, lines[i])
	}
	if end < len(lines) {
		fmt.Fprintf(&b, "… %d more lines\n", len(lines)-end)
	}
	return Result{OK: true, Output: b.String(), Target: r.WS.Rel(abs)}
}

func (r *Runner) listDir(args map[string]any) Result {
	path := strArg(args, "path")
	if path == "" {
		path = "."
	}
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: path}
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	var b strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		fmt.Fprintln(&b, name)
	}
	return Result{OK: true, Output: b.String(), Target: r.WS.Rel(abs)}
}

func (r *Runner) findFiles(ctx context.Context, args map[string]any) Result {
	pattern := strArg(args, "pattern")
	path := strArg(args, "path")
	if path == "" {
		path = "."
	}
	limit := intArg(args, "limit", 50)
	if limit < 1 {
		limit = 50
	}
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: pattern}
	}
	target := pattern + " in " + r.WS.Rel(abs)

	if r.HasFD {
		// fd [options] [pattern] [path]
		cmdArgs := []string{"--color=never", "-H", "-t", "f", "-l", strconv.Itoa(limit)}
		// treat pattern as glob-ish
		cmdArgs = append(cmdArgs, pattern, abs)
		out, err := r.runCmd(ctx, 30*time.Second, "fd", cmdArgs...)
		if err != nil && out == "" {
			return Result{OK: false, Output: err.Error(), Target: target}
		}
		// relativize paths
		lines := strings.Split(strings.TrimSpace(out), "\n")
		var rels []string
		for _, ln := range lines {
			if ln == "" {
				continue
			}
			rels = append(rels, r.WS.Rel(ln))
		}
		backend := "fd"
		body := strings.Join(rels, "\n")
		if body != "" {
			body += "\n"
		}
		body += fmt.Sprintf("(via %s, %d hits)\n", backend, len(rels))
		return Result{OK: true, Output: body, Target: target}
	}

	// Go walk fallback
	var hits []string
	_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		rel := r.WS.Rel(p)
		matched := false
		if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
			ok, _ := filepath.Match(pattern, base)
			ok2, _ := filepath.Match(pattern, rel)
			matched = ok || ok2
		} else {
			matched = strings.Contains(base, pattern) || strings.Contains(rel, pattern)
		}
		if matched {
			hits = append(hits, rel)
			if len(hits) >= limit {
				return fmt.Errorf("limit")
			}
		}
		return nil
	})
	body := strings.Join(hits, "\n")
	if body != "" {
		body += "\n"
	}
	body += fmt.Sprintf("(via go walk, %d hits)\n", len(hits))
	return Result{OK: true, Output: body, Target: target}
}

func (r *Runner) searchCode(ctx context.Context, args map[string]any) Result {
	query := strArg(args, "query")
	path := strArg(args, "path")
	if path == "" {
		path = "."
	}
	glob := strArg(args, "glob")
	ci := boolArg(args, "case_insensitive")
	limit := intArg(args, "limit", 40)
	if limit < 1 {
		limit = 40
	}
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: query}
	}
	target := "/" + query + "/"
	if glob != "" {
		target += " " + glob
	}

	if r.HasRG {
		cmdArgs := []string{"--color=never", "--line-number", "--no-heading", "--max-count", strconv.Itoa(limit)}
		if ci {
			cmdArgs = append(cmdArgs, "-i")
		}
		if glob != "" {
			cmdArgs = append(cmdArgs, "--glob", glob)
		}
		cmdArgs = append(cmdArgs, query, abs)
		out, err := r.runCmd(ctx, 30*time.Second, "rg", cmdArgs...)
		// rg exits 1 for no matches
		if err != nil && out == "" {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				return Result{OK: true, Output: "(no matches via rg)\n", Target: target}
			}
			return Result{OK: false, Output: err.Error() + "\n" + out, Target: target}
		}
		// relativize
		var b strings.Builder
		n := 0
		for _, ln := range strings.Split(out, "\n") {
			if ln == "" {
				continue
			}
			// path:line:text
			if i := strings.Index(ln, ":"); i > 0 {
				p := ln[:i]
				rest := ln[i:]
				ln = r.WS.Rel(p) + rest
			}
			b.WriteString(ln)
			b.WriteByte('\n')
			n++
			if n >= limit {
				break
			}
		}
		b.WriteString(fmt.Sprintf("(via rg, %d lines)\n", n))
		return Result{OK: true, Output: b.String(), Target: target}
	}

	// Go regex fallback
	re, err := regexp.Compile(query)
	if err != nil {
		re, err = regexp.Compile(regexp.QuoteMeta(query))
		if err != nil {
			return Result{OK: false, Output: "bad pattern: " + err.Error(), Target: target}
		}
	}
	if ci {
		re, _ = regexp.Compile("(?i)" + re.String())
	}
	var b strings.Builder
	n := 0
	_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != "" {
			ok, _ := filepath.Match(glob, d.Name())
			if !ok {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil || isBinary(data) {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				fmt.Fprintf(&b, "%s:%d:%s\n", r.WS.Rel(p), i+1, line)
				n++
				if n >= limit {
					return fmt.Errorf("limit")
				}
			}
		}
		return nil
	})
	b.WriteString(fmt.Sprintf("(via go scan, %d lines)\n", n))
	return Result{OK: true, Output: b.String(), Target: target}
}

func (r *Runner) strReplace(args map[string]any) Result {
	path := strArg(args, "path")
	oldS := strArg(args, "old_string")
	newS := strArg(args, "new_string")
	if oldS == "" {
		return Result{OK: false, Output: "old_string is required", Target: path}
	}
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: path}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	text := string(data)
	count := strings.Count(text, oldS)
	if count == 0 {
		return Result{OK: false, Output: "old_string not found", Target: r.WS.Rel(abs)}
	}
	if count > 1 {
		return Result{OK: false, Output: fmt.Sprintf("old_string found %d times; must be unique", count), Target: r.WS.Rel(abs)}
	}
	out := strings.Replace(text, oldS, newS, 1)
	if err := os.WriteFile(abs, []byte(out), 0o644); err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	diff := unifiedSnippet(r.WS.Rel(abs), oldS, newS)
	return Result{
		OK:     true,
		Output: "replaced 1 occurrence\n" + diff,
		Diff:   diff,
		Target: fmt.Sprintf("%s  edit", r.WS.Rel(abs)),
	}
}

func (r *Runner) writeFile(args map[string]any) Result {
	path := strArg(args, "path")
	contents := strArg(args, "contents")
	abs, err := r.WS.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Target: path}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	prev, _ := os.ReadFile(abs)
	if err := os.WriteFile(abs, []byte(contents), 0o644); err != nil {
		return Result{OK: false, Output: err.Error(), Target: r.WS.Rel(abs)}
	}
	rel := r.WS.Rel(abs)
	var diff string
	if len(prev) == 0 {
		diff = fmt.Sprintf("--- /dev/null\n+++ b/%s\n@@ new file %d bytes @@\n", rel, len(contents))
	} else {
		diff = fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ write %d → %d bytes @@\n", rel, rel, len(prev), len(contents))
	}
	// show first lines of new content as evidence
	lines := strings.Split(contents, "\n")
	max := 40
	if len(lines) < max {
		max = len(lines)
	}
	var b strings.Builder
	b.WriteString(diff)
	for i := 0; i < max; i++ {
		b.WriteString("+")
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	if len(lines) > max {
		fmt.Fprintf(&b, "… %d more lines\n", len(lines)-max)
	}
	return Result{OK: true, Output: b.String(), Diff: b.String(), Target: fmt.Sprintf("%s  write", rel)}
}

func (r *Runner) runShell(ctx context.Context, args map[string]any) Result {
	command := strArg(args, "command")
	if command == "" {
		return Result{OK: false, Output: "command is required", Target: "shell"}
	}
	sec := intArg(args, "timeout_sec", 60)
	if sec < 1 {
		sec = 60
	}
	if sec > 300 {
		sec = 300
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, r.Shell, "-lc", command)
	cmd.Dir = r.WS.Root
	// minimal env
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &buf, n: r.MaxOut}
	cmd.Stderr = &limitedWriter{w: &buf, n: r.MaxOut}
	err := cmd.Run()
	out := buf.String()
	if out == "" && err != nil {
		out = err.Error()
	}
	header := "$ " + command + "\n"
	ok := err == nil
	if err != nil {
		if ee, okExit := err.(*exec.ExitError); okExit {
			header += fmt.Sprintf("(exit %d)\n", ee.ExitCode())
		} else if cctx.Err() != nil {
			header += "(timeout)\n"
			ok = false
		} else {
			header += "(error: " + err.Error() + ")\n"
		}
	}
	return Result{OK: ok, Output: header + out, Target: strutil.Truncate(command, 60)}
}

func (r *Runner) runCmd(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	cmd.Dir = r.WS.Root
	var buf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &buf, n: r.MaxOut}
	cmd.Stderr = &limitedWriter{w: &buf, n: r.MaxOut}
	err := cmd.Run()
	return buf.String(), err
}

type limitedWriter struct {
	w         io.Writer
	n         int
	written   int
	truncated bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.written >= l.n {
		l.truncated = true
		return len(p), nil
	}
	rest := l.n - l.written
	if len(p) > rest {
		_, _ = l.w.Write(p[:rest])
		_, _ = l.w.Write([]byte("\n…(output truncated)\n"))
		l.written = l.n
		l.truncated = true
		return len(p), nil
	}
	n, err := l.w.Write(p)
	l.written += n
	return n, err
}

func isBinary(data []byte) bool {
	// null byte in first 8k ⇒ binary
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "bin", ".idea", ".vscode", "target":
		return true
	default:
		return false
	}
}

func unifiedSnippet(path, oldS, newS string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", path, path)
	for _, ln := range strings.Split(oldS, "\n") {
		b.WriteString("-")
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	for _, ln := range strings.Split(newS, "\n") {
		b.WriteString("+")
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	return b.String()
}
