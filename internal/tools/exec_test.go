package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndStrReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(ws)

	res := r.Exec(context.Background(), "read_file", `{"path":"hello.txt"}`)
	if !res.OK || !strings.Contains(res.Output, "hello world") {
		t.Fatalf("read: %+v", res)
	}

	res = r.Exec(context.Background(), "str_replace", `{"path":"hello.txt","old_string":"hello world","new_string":"hello exr"}`)
	if !res.OK {
		t.Fatalf("replace: %+v", res)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello exr\n" {
		t.Fatalf("got %q", data)
	}
}

func TestWorkspaceEscape(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ws.Resolve("../outside")
	if err == nil {
		t.Fatal("expected escape error")
	}
}

func TestWorkspaceSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "leak")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink not supported:", err)
	}
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ws.Resolve("leak/secret.txt")
	if err == nil {
		t.Fatal("expected symlink escape error")
	}
}

func TestSearchBackendsReported(t *testing.T) {
	dir := t.TempDir()
	ws, _ := NewWorkspace(dir)
	r := NewRunner(ws)
	info := r.BackendInfo()
	if info == "" {
		t.Fatal("empty backend info")
	}
	t.Log(info)
}
