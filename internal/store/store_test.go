package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootHonorsEnvAndExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("DOTLLM_HOME", "/tmp/explicit")
	if got, _ := Root(); got != "/tmp/explicit" {
		t.Errorf("Root with env = %q, want /tmp/explicit", got)
	}

	t.Setenv("DOTLLM_HOME", "~/archive")
	if got, _ := Root(); got != filepath.Join(home, "archive") {
		t.Errorf("Root with ~ = %q, want %q", got, filepath.Join(home, "archive"))
	}

	os.Unsetenv("DOTLLM_HOME")
	if got, _ := Root(); got != filepath.Join(home, ".llm") {
		t.Errorf("Root default = %q, want %q", got, filepath.Join(home, ".llm"))
	}
}

func TestWorkspacePath(t *testing.T) {
	root := "/r"
	if got := WorkspacePath(root, "app", "2026-06-14", ""); got != "/r/app/2026-06-14" {
		t.Errorf("WorkspacePath no task = %q", got)
	}
	if got := WorkspacePath(root, "app", "2026-06-14", "fix"); got != "/r/app/2026-06-14_fix" {
		t.Errorf("WorkspacePath with task = %q", got)
	}
}

func TestScanGroupsAndCounts(t *testing.T) {
	root := t.TempDir()
	// app/2026-06-14 with 2 files (one nested)
	mustWrite(t, filepath.Join(root, "app", "2026-06-14", "a.md"), "a")
	mustWrite(t, filepath.Join(root, "app", "2026-06-14", "sub", "b.md"), "b")
	// app/2026-06-13_fix with 1 file
	mustWrite(t, filepath.Join(root, "app", "2026-06-13_fix", "c.md"), "c")
	// web/2026-06-14 with 0 files (empty dir)
	if err := os.MkdirAll(filepath.Join(root, "web", "2026-06-14"), 0o755); err != nil {
		t.Fatal(err)
	}

	groups, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Repo != "app" || groups[1].Repo != "web" {
		t.Fatalf("groups = %+v, want app then web", groups)
	}

	app := groups[0].Workspaces
	if len(app) != 2 {
		t.Fatalf("app workspaces = %d, want 2", len(app))
	}
	// sorted by name: 2026-06-13_fix before 2026-06-14
	if app[0].Name != "2026-06-13_fix" || app[0].Date != "2026-06-13" || app[0].Task != "fix" || app[0].Files != 1 {
		t.Errorf("app[0] = %+v", app[0])
	}
	if app[1].Name != "2026-06-14" || app[1].Task != "" || app[1].Files != 2 {
		t.Errorf("app[1] = %+v", app[1])
	}
	if groups[1].Workspaces[0].Files != 0 {
		t.Errorf("web workspace files = %d, want 0", groups[1].Workspaces[0].Files)
	}
}

func TestScanMissingRoot(t *testing.T) {
	groups, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("Scan missing root err = %v, want nil", err)
	}
	if len(groups) != 0 {
		t.Errorf("groups = %d, want 0", len(groups))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
