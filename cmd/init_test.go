package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func linkTarget(t *testing.T, wd string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(wd, ".llm"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	return filepath.Clean(target)
}

func TestInitCreatesLinkAndArchive(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "app", "2026-06-14")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
	if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
		t.Errorf("archive dir missing: %v", err)
	}
	if !strings.Contains(buf.String(), "linked") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestInitDefaultsToPlainDateBucket(t *testing.T) {
	// No task is created unless one is passed.
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "app", "2026-06-14")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("link target = %q, want %q (plain date bucket)", got, want)
	}
}

func TestInitTaskFromNameOrPositional(t *testing.T) {
	for _, args := range [][]string{{"init", "--name", "fix"}, {"init", "fix"}} {
		wd := t.TempDir()
		a, _ := testApp(t, wd, fakeRepo{repo: "app"})
		if err := runCmd(a, args...); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(a.root, "app", "2026-06-14_fix")
		if got := linkTarget(t, wd); got != want {
			t.Errorf("%v: link target = %q, want %q", args, got, want)
		}
	}
}

func TestInitRepoOverride(t *testing.T) {
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "autodetected"})
	if err := runCmd(a, "init", "--repo", "chosen"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "chosen", "2026-06-14")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
}

func TestInitProjectAndDateOverride(t *testing.T) {
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "autodetected"})
	if err := runCmd(a, "init", "--project", "BrowserOS", "--date", "2026-06-23"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "BrowserOS", "2026-06-23")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
}

func TestInitProjectDefaultsToToday(t *testing.T) {
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "autodetected"})
	if err := runCmd(a, "init", "--project", "BrowserOS"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "BrowserOS", "2026-06-14")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
}

func TestInitRejectsConflictingRepoAndProject(t *testing.T) {
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "autodetected"})
	if err := runCmd(a, "init", "--repo", "one", "--project", "two"); err == nil {
		t.Fatal("conflicting --repo and --project should error")
	}
	if _, err := os.Lstat(filepath.Join(wd, ".llm")); !os.IsNotExist(err) {
		t.Fatalf(".llm should not be created after conflict, err = %v", err)
	}
}

func TestInitRejectsInvalidDateWithoutLink(t *testing.T) {
	wd := t.TempDir()
	a, _ := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init", "--date", "2026-6-23"); err == nil {
		t.Fatal("invalid --date should error")
	}
	if _, err := os.Lstat(filepath.Join(wd, ".llm")); !os.IsNotExist(err) {
		t.Fatalf(".llm should not be created after invalid date, err = %v", err)
	}
}

func TestInitJSON(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init", "--json"); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Canonical string `json:"canonical"`
		Linked    bool   `json:"linked"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if !res.Linked || res.Canonical != filepath.Join(a.root, "app", "2026-06-14") {
		t.Errorf("res = %+v", res)
	}
}

func TestInitQuietIsSilent(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init", "--quiet"); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("quiet output = %q, want empty", buf.String())
	}
	if _, err := os.Lstat(filepath.Join(wd, ".llm")); err != nil {
		t.Errorf(".llm should still be created: %v", err)
	}
}

func TestInitIdempotentReportsAlready(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "already linked") {
		t.Errorf("second init = %q", buf.String())
	}
}

func TestInitWorktreeMirrorsMainRealLLM(t *testing.T) {
	// main's .llm is a real directory (e.g. a tool's root that isn't managed).
	// A worktree init must link straight to it, not mint a fresh dated bucket.
	mainRoot := t.TempDir()
	mainLLM := filepath.Join(mainRoot, ".llm")
	if err := os.MkdirAll(mainLLM, 0o755); err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	a, _ := testApp(t, wt, fakeRepo{repo: "skl", mainRoot: mainRoot, isWorktree: true})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	if got, want := linkTarget(t, wt), filepath.Clean(mainLLM); got != want {
		t.Errorf("worktree .llm -> %q, want main's real .llm %q", got, want)
	}
}

func TestInitWorktreeMirrorsManagedMain(t *testing.T) {
	// main's .llm is a managed symlink into the archive; the worktree should
	// point at the same archive dir (the link's target), not a chained link.
	mainRoot := t.TempDir()
	wt := t.TempDir()
	a, _ := testApp(t, wt, fakeRepo{repo: "skl", mainRoot: mainRoot, isWorktree: true})
	managed := filepath.Join(a.root, "skl", "2026-06-14")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(mainRoot, ".llm")); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	if got := linkTarget(t, wt); got != managed {
		t.Errorf("worktree .llm -> %q, want shared archive dir %q", got, managed)
	}
}

func TestInitWorktreeFallsBackWhenMainAbsent(t *testing.T) {
	// main has no .llm yet: the worktree falls back to the normal bucket, which
	// (sharing the main repo's name) is what main adopts on its own init.
	mainRoot := t.TempDir() // no .llm
	wt := t.TempDir()
	a, _ := testApp(t, wt, fakeRepo{repo: "skl", mainRoot: mainRoot, isWorktree: true})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "skl", "2026-06-14")
	if got := linkTarget(t, wt); got != want {
		t.Errorf("worktree .llm -> %q, want fallback bucket %q", got, want)
	}
}

func TestInitForeignSymlinkNeedsForce(t *testing.T) {
	wd := t.TempDir()
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(wd, ".llm")); err != nil {
		t.Fatal(err)
	}
	a, _ := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init"); err == nil {
		t.Fatalf("foreign symlink should error without --force")
	}
	if err := runCmd(a, "init", "--force"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(a.root, "app", "2026-06-14")
	if got := linkTarget(t, wd); got != want {
		t.Errorf("after force, target = %q, want %q", got, want)
	}
}
