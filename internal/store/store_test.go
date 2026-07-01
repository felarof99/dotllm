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
	if got := WorkspacePath(root, "app", "2026-06-14", ""); got != "/r/2026-06-14/app" {
		t.Errorf("WorkspacePath no task = %q", got)
	}
	if got := WorkspacePath(root, "app", "2026-06-14", "fix"); got != "/r/2026-06-14/app/fix" {
		t.Errorf("WorkspacePath with task = %q", got)
	}
}

func TestScanDateFirstGroupsAndCounts(t *testing.T) {
	root := t.TempDir()
	// 2026-06-14/app with 2 files
	mustWrite(t, filepath.Join(root, "2026-06-14", "app", "a.md"), "a")
	mustWrite(t, filepath.Join(root, "2026-06-14", "app", "b.md"), "b")
	// 2026-06-14/web/fix with 1 file
	mustWrite(t, filepath.Join(root, "2026-06-14", "web", "fix", "c.md"), "c")
	mustWrite(t, filepath.Join(root, "2026-06-14", "web", "fix", ".dotllm-task"), "")
	// 2026-06-13/web with 0 files (empty dir)
	if err := os.MkdirAll(filepath.Join(root, "2026-06-13", "web"), 0o755); err != nil {
		t.Fatal(err)
	}

	groups, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Date != "2026-06-14" || groups[1].Date != "2026-06-13" {
		t.Fatalf("groups = %+v, want recent date groups first", groups)
	}

	recent := groups[0].Workspaces
	if len(recent) != 2 {
		t.Fatalf("recent workspaces = %d, want 2", len(recent))
	}
	if recent[0].Repo != "app" || recent[0].Name != "app" || recent[0].Task != "" || recent[0].Files != 2 {
		t.Errorf("recent[0] = %+v", recent[0])
	}
	if recent[1].Repo != "web" || recent[1].Name != "web/fix" || recent[1].Task != "fix" || recent[1].Files != 1 {
		t.Errorf("recent[1] = %+v", recent[1])
	}
	if groups[1].Workspaces[0].Files != 0 {
		t.Errorf("web workspace files = %d, want 0", groups[1].Workspaces[0].Files)
	}
}

func TestScanDoesNotTreatUnmarkedChildDirsAsTasks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "2026-06-14", "app", "note.md"), "a")
	if err := os.MkdirAll(filepath.Join(root, "2026-06-14", "app", "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}

	groups, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want one date group", groups)
	}
	wss := groups[0].Workspaces
	if len(wss) != 1 {
		t.Fatalf("workspaces = %+v, want only the plain app workspace", wss)
	}
	if wss[0].Name != "app" || wss[0].Task != "" || wss[0].Files != 1 {
		t.Errorf("workspace = %+v, want plain app with one file", wss[0])
	}
}

func TestScanKeepsLegacyRepoFirstArchivesVisible(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "2026-06-14", "app", "a.md"), "a")
	mustWrite(t, filepath.Join(root, "legacyapp", "2026-06-13_fix", "c.md"), "c")

	groups, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want current plus legacy groups", groups)
	}
	if groups[0].Legacy {
		t.Fatalf("legacy group should sort after date-first groups: %+v", groups)
	}
	if !groups[1].Legacy {
		t.Fatalf("second group should be legacy: %+v", groups)
	}
	legacy := groups[1].Workspaces
	if len(legacy) != 1 {
		t.Fatalf("legacy workspaces = %+v, want 1", legacy)
	}
	if legacy[0].Repo != "legacyapp" || legacy[0].Date != "2026-06-13" || legacy[0].Task != "fix" || !legacy[0].Legacy {
		t.Errorf("legacy workspace = %+v", legacy[0])
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
