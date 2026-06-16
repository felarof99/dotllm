package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setup returns a project dir and the canonical archive path under a temp root.
func setup(t *testing.T) (dir, canonical string) {
	t.Helper()
	root := t.TempDir()
	dir = t.TempDir()
	canonical = filepath.Join(root, "app", "2026-06-14")
	return dir, canonical
}

func readThrough(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestInitFreshCreatesLink(t *testing.T) {
	dir, canonical := setup(t)
	res, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Linked {
		t.Errorf("Linked = false, want true")
	}
	// .llm is a symlink to canonical
	target, err := os.Readlink(filepath.Join(dir, ".llm"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(target) != filepath.Clean(canonical) {
		t.Errorf("link target = %q, want %q", target, canonical)
	}
	// writing through the link lands in the archive
	if err := os.WriteFile(filepath.Join(dir, ".llm", "x.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readThrough(t, filepath.Join(canonical, "x.md")); got != "hi" {
		t.Errorf("archive file = %q, want hi", got)
	}
}

func TestInitIdempotent(t *testing.T) {
	dir, canonical := setup(t)
	if _, err := Init(InitOptions{Dir: dir, Canonical: canonical}); err != nil {
		t.Fatal(err)
	}
	res, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyOK || res.Linked {
		t.Errorf("second init = %+v, want AlreadyOK", res)
	}
}

func TestInitAdoptsRealDir(t *testing.T) {
	dir, canonical := setup(t)
	llm := filepath.Join(dir, ".llm")
	if err := os.MkdirAll(filepath.Join(llm, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(llm, "a.md"), "alpha")
	mustWrite(t, filepath.Join(llm, "sub", "b.md"), "beta")

	res, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Linked || len(res.Adopted) != 2 {
		t.Errorf("res = %+v, want Linked + 2 adopted", res)
	}
	// .llm is now a symlink
	fi, _ := os.Lstat(llm)
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf(".llm is not a symlink after adopt")
	}
	// files readable through both paths
	if got := readThrough(t, filepath.Join(canonical, "a.md")); got != "alpha" {
		t.Errorf("archive a.md = %q", got)
	}
	if got := readThrough(t, filepath.Join(llm, "sub", "b.md")); got != "beta" {
		t.Errorf("through-link sub/b.md = %q", got)
	}
}

func TestInitAdoptConflictAbortsNonDestructively(t *testing.T) {
	dir, canonical := setup(t)
	// archive already holds a.md
	mustWrite(t, filepath.Join(canonical, "a.md"), "ARCHIVE")
	// local real .llm has conflicting a.md and a fresh c.md
	mustWrite(t, filepath.Join(dir, ".llm", "a.md"), "LOCAL")
	mustWrite(t, filepath.Join(dir, ".llm", "c.md"), "fresh")

	_, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err == nil || !strings.Contains(err.Error(), "a.md") {
		t.Fatalf("err = %v, want conflict mentioning a.md", err)
	}
	// archive copy untouched, nothing moved (all-or-nothing)
	if got := readThrough(t, filepath.Join(canonical, "a.md")); got != "ARCHIVE" {
		t.Errorf("archive a.md = %q, want ARCHIVE", got)
	}
	if _, err := os.Stat(filepath.Join(canonical, "c.md")); !os.IsNotExist(err) {
		t.Errorf("c.md should not have moved on conflict")
	}
	// local .llm still a real dir with both files
	fi, _ := os.Lstat(filepath.Join(dir, ".llm"))
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Errorf("local .llm should remain a real dir on conflict")
	}
	if got := readThrough(t, filepath.Join(dir, ".llm", "c.md")); got != "fresh" {
		t.Errorf("local c.md = %q, want fresh", got)
	}
}

func TestInitReattaches(t *testing.T) {
	dir, canonical := setup(t)
	// archive exists with a file, but local .llm is missing
	mustWrite(t, filepath.Join(canonical, "keep.md"), "k")
	res, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Linked {
		t.Errorf("re-attach should create the link")
	}
	if got := readThrough(t, filepath.Join(dir, ".llm", "keep.md")); got != "k" {
		t.Errorf("re-attached file = %q, want k", got)
	}
}

func TestInitForeignSymlinkNeedsForce(t *testing.T) {
	dir, canonical := setup(t)
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(dir, ".llm")); err != nil {
		t.Fatal(err)
	}
	// without force: refuse
	if _, err := Init(InitOptions{Dir: dir, Canonical: canonical}); err == nil {
		t.Fatalf("expected refusal for foreign symlink")
	}
	// with force: re-point
	res, err := Init(InitOptions{Dir: dir, Canonical: canonical, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Linked {
		t.Errorf("force should re-point")
	}
	target, _ := os.Readlink(filepath.Join(dir, ".llm"))
	if filepath.Clean(target) != filepath.Clean(canonical) {
		t.Errorf("after force, target = %q, want %q", target, canonical)
	}
}

func TestStatClassification(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "app", "2026-06-14")

	// Absent
	d1 := t.TempDir()
	if st, _ := Stat(d1, root); st.Kind != Absent {
		t.Errorf("absent: kind = %s", st.Kind)
	}

	// Managed
	d2 := t.TempDir()
	if _, err := Init(InitOptions{Dir: d2, Canonical: canonical}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(canonical, "f.md"), "x")
	if st, _ := Stat(d2, root); st.Kind != Managed || st.Files != 1 {
		t.Errorf("managed: %+v", st)
	}

	// Dangling
	d3 := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "gone", "missing"), filepath.Join(d3, ".llm")); err != nil {
		t.Fatal(err)
	}
	if st, _ := Stat(d3, root); st.Kind != Dangling {
		t.Errorf("dangling: kind = %s", st.Kind)
	}

	// Foreign (real dir)
	d4 := t.TempDir()
	if err := os.Mkdir(filepath.Join(d4, ".llm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if st, _ := Stat(d4, root); st.Kind != Foreign {
		t.Errorf("foreign dir: kind = %s", st.Kind)
	}

	// Foreign (symlink outside archive)
	d5 := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(d5, ".llm")); err != nil {
		t.Fatal(err)
	}
	if st, _ := Stat(d5, root); st.Kind != Foreign {
		t.Errorf("foreign link: kind = %s", st.Kind)
	}
}

func TestInitCrossDeviceFallback(t *testing.T) {
	// Force the copy-then-remove path by making rename always report EXDEV.
	orig := renameFunc
	renameFunc = func(_, _ string) error { return errCrossDevice }
	defer func() { renameFunc = orig }()

	dir, canonical := setup(t)
	mustWrite(t, filepath.Join(dir, ".llm", "a.md"), "viacopy")
	mustWrite(t, filepath.Join(dir, ".llm", "d", "e.md"), "nested")

	res, err := Init(InitOptions{Dir: dir, Canonical: canonical})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Linked {
		t.Fatalf("res = %+v", res)
	}
	if got := readThrough(t, filepath.Join(canonical, "a.md")); got != "viacopy" {
		t.Errorf("copied a.md = %q", got)
	}
	if got := readThrough(t, filepath.Join(canonical, "d", "e.md")); got != "nested" {
		t.Errorf("copied nested = %q", got)
	}
	// source consumed
	if _, err := os.Stat(filepath.Join(dir, ".llm")); err == nil {
		fi, _ := os.Lstat(filepath.Join(dir, ".llm"))
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf(".llm should be a symlink after cross-device adopt")
		}
	}
}

func TestInitCrossDevicePreservesSymlink(t *testing.T) {
	// Finding 1: the cross-device fallback must not flatten a symlink into a
	// regular file copy of its target.
	orig := renameFunc
	renameFunc = func(_, _ string) error { return errCrossDevice }
	defer func() { renameFunc = orig }()

	dir, canonical := setup(t)
	llm := filepath.Join(dir, ".llm")
	if err := os.MkdirAll(llm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/somewhere/else", filepath.Join(llm, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{Dir: dir, Canonical: canonical}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(filepath.Join(canonical, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("adopted entry is a %v, want a symlink", fi.Mode())
	}
	if got, _ := os.Readlink(filepath.Join(canonical, "link")); got != "/somewhere/else" {
		t.Errorf("symlink target = %q, want /somewhere/else", got)
	}
}

func TestInitCrossDevicePartialFailureRollsBackAndRecovers(t *testing.T) {
	// Finding 2: a mid-copy failure must leave nothing partial in the archive
	// (so the conflict pre-check doesn't wedge a later retry) and must not
	// remove the still-uncopied source.
	origR, origC := renameFunc, copyFileFunc
	renameFunc = func(_, _ string) error { return errCrossDevice }
	failOn := "bad.md"
	copyFileFunc = func(src, dst string, perm os.FileMode) error {
		if filepath.Base(src) == failOn {
			return os.ErrPermission
		}
		return origC(src, dst, perm)
	}
	defer func() { renameFunc, copyFileFunc = origR, origC }()

	dir, canonical := setup(t)
	mustWrite(t, filepath.Join(dir, ".llm", "bad.md"), "boom")

	if _, err := Init(InitOptions{Dir: dir, Canonical: canonical}); err == nil {
		t.Fatal("expected the copy failure to surface")
	}
	// no partial dst left behind
	if _, err := os.Stat(filepath.Join(canonical, "bad.md")); !os.IsNotExist(err) {
		t.Errorf("partial dst should have been rolled back: %v", err)
	}
	// source still intact
	if got := readThrough(t, filepath.Join(dir, ".llm", "bad.md")); got != "boom" {
		t.Errorf("source should survive a failed move, got %q", got)
	}

	// retry succeeds once the copy works
	copyFileFunc = origC
	if _, err := Init(InitOptions{Dir: dir, Canonical: canonical}); err != nil {
		t.Fatalf("retry should recover: %v", err)
	}
	if got := readThrough(t, filepath.Join(canonical, "bad.md")); got != "boom" {
		t.Errorf("archive bad.md = %q after recovery", got)
	}
}

func TestIsUnder(t *testing.T) {
	cases := []struct {
		path, root string
		want       bool
	}{
		{"/a/b/c", "/a/b", true},
		{"/a/b", "/a/b", true},
		{"/a/bc", "/a/b", false}, // sibling, not nested
		{"/x", "/", true},        // Finding 4: filesystem root
		{"/a/b", "/c", false},
	}
	for _, c := range cases {
		if got := isUnder(c.path, c.root); got != c.want {
			t.Errorf("isUnder(%q,%q) = %v, want %v", c.path, c.root, got, c.want)
		}
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
