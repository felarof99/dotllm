package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrunePreviewsByDefault(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	empty := filepath.Join(a.root, "2026-06-14", "app")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(a, "prune"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "would remove") {
		t.Errorf("preview = %q", buf.String())
	}
	if _, err := os.Stat(empty); err != nil {
		t.Errorf("preview must not delete: %v", err)
	}
}

func TestPruneYesRemovesEmptyKeepsNonEmpty(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	empty := filepath.Join(a.root, "2026-06-14", "app")
	full := filepath.Join(a.root, "2026-06-10", "web")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "note.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runCmd(a, "prune", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Errorf("empty workspace should be gone: %v", err)
	}
	// now-empty date parent removed too
	if _, err := os.Stat(filepath.Join(a.root, "2026-06-14")); !os.IsNotExist(err) {
		t.Errorf("empty date parent should be gone")
	}
	// non-empty workspace untouched
	if _, err := os.Stat(filepath.Join(full, "note.md")); err != nil {
		t.Errorf("non-empty workspace must survive: %v", err)
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestPruneKeepsSymlinkOnlyWorkspace(t *testing.T) {
	// Finding 3: a workspace whose only content is a symlink is NOT empty and
	// must not be deleted.
	a, _ := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	ws := filepath.Join(a.root, "2026-06-14", "app")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/some/target", filepath.Join(ws, "ref")); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(a, "prune", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(ws, "ref")); err != nil {
		t.Errorf("symlink-only workspace must survive prune: %v", err)
	}
}

func TestPruneYesRemovesEmptyTaskAndParents(t *testing.T) {
	a, _ := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	emptyTask := filepath.Join(a.root, "2026-06-14", "app", "fix")
	if err := os.MkdirAll(emptyTask, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runCmd(a, "prune", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(emptyTask); !os.IsNotExist(err) {
		t.Errorf("empty task workspace should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.root, "2026-06-14")); !os.IsNotExist(err) {
		t.Errorf("empty date parent should be gone")
	}
}

func TestPruneKeepsLegacyRepoFirstCleanup(t *testing.T) {
	a, _ := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	legacy := filepath.Join(a.root, "legacyapp", "2026-06-14")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runCmd(a, "prune", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("empty legacy workspace should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.root, "legacyapp")); !os.IsNotExist(err) {
		t.Errorf("empty legacy repo parent should be gone")
	}
}

func TestPruneNothing(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "prune"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "nothing to prune") {
		t.Errorf("output = %q", buf.String())
	}
}
