package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace bounds filesystem access to a root directory.
type Workspace struct {
	Root string
}

// NewWorkspace resolves root to an absolute path (symlink-canonical when possible).
func NewWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace is not a directory: %s", abs)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return &Workspace{Root: filepath.Clean(abs)}, nil
}

// Resolve maps a user path into the workspace. Rejects escapes (including symlink escapes).
func (w *Workspace) Resolve(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	// expand ~
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	var abs string
	var err error
	if filepath.IsAbs(p) {
		abs, err = filepath.Abs(p)
	} else {
		abs, err = filepath.Abs(filepath.Join(w.Root, p))
	}
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	root := filepath.Clean(w.Root)
	if err := underRoot(root, abs); err != nil {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}
	// If the path (or a parent) is a symlink chain, ensure the real path stays inside root.
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		real = filepath.Clean(real)
		if err := underRoot(root, real); err != nil {
			return "", fmt.Errorf("path escapes workspace via symlink: %s", p)
		}
		return real, nil
	}
	// Not fully existent yet (e.g. write new file): verify existing parents.
	dir := abs
	for {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			if err := underRoot(root, filepath.Clean(real)); err != nil {
				return "", fmt.Errorf("path escapes workspace via symlink: %s", p)
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return abs, nil
}

func underRoot(root, abs string) error {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)
	// Canonicalize both sides when they exist (macOS /var → /private/var, etc.).
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = filepath.Clean(r)
	}
	if a, err := filepath.EvalSymlinks(abs); err == nil {
		abs = filepath.Clean(a)
	}
	sep := string(os.PathSeparator)
	if abs == root {
		return nil
	}
	if !strings.HasPrefix(abs, root+sep) {
		return fmt.Errorf("outside root")
	}
	return nil
}

// Rel returns a path relative to the workspace for display.
func (w *Workspace) Rel(abs string) string {
	rel, err := filepath.Rel(w.Root, abs)
	if err != nil {
		return abs
	}
	return rel
}
