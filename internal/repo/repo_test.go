package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"browseros":            "browseros",
		"feat/resume-finished": "feat-resume-finished",
		"  spaced  name  ":     "spaced-name",
		"a//b\\c":              "a-b-c",
		"-.lead.trail.-":       "lead.trail",
		"weird:*?name":         "weirdname",
		"":                     "",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

// withFakeGit prepends a temp dir holding a fake `git` script to PATH so the
// Git resolver can be exercised hermetically. An empty toplevel makes
// show-toplevel fail, simulating a directory that is not a git work tree.
func withFakeGit(t *testing.T, toplevel string) {
	t.Helper()
	bin := t.TempDir()
	top := "exit 128"
	if toplevel != "" {
		top = "echo '" + toplevel + "'"
	}
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"  *show-toplevel*) " + top + " ;;\n" +
		"  *) exit 1 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGitRepoFromToplevel(t *testing.T) {
	withFakeGit(t, "/Users/x/code/browseros")
	got, err := Git{}.Repo("/Users/x/code/browseros/src")
	if err != nil {
		t.Fatal(err)
	}
	if got != "browseros" {
		t.Errorf("Repo = %q, want browseros", got)
	}
}

func TestGitRepoFallsBackToDirBasename(t *testing.T) {
	withFakeGit(t, "") // show-toplevel fails => not a work tree
	dir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Git{}.Repo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "myproject" {
		t.Errorf("Repo = %q, want myproject", got)
	}
}
