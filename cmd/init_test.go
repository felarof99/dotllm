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
