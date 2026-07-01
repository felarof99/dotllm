package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/felarof01/dotllm/internal/repo"
)

// fakeRepo is an injectable repo.Resolver for hermetic command tests.
type fakeRepo struct {
	repo       string
	mainRoot   string
	isWorktree bool
}

func (f fakeRepo) Repo(string) (string, error)           { return f.repo, nil }
func (f fakeRepo) MainRoot(string) (string, bool, error) { return f.mainRoot, f.isWorktree, nil }

// testApp builds an app whose deps are all fakes: output to a buffer, a fixed
// clock (2026-06-14), a temp archive root, and a fixed working directory.
func testApp(t *testing.T, wd string, fr repo.Resolver) (*app, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	return &app{
		out:    buf,
		errOut: buf,
		repo:   fr,
		now:    func() time.Time { return time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC) },
		root:   t.TempDir(),
		wd:     func() (string, error) { return wd, nil },
	}, buf
}

func runCmd(a *app, args ...string) error {
	c := newRootCmdWithApp(a)
	c.SetArgs(args)
	return c.Execute()
}

func TestBareInvocationRunsStatus(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "not initialized") {
		t.Errorf("bare output = %q, want status for an uninitialized dir", buf.String())
	}
}

func TestHelpListsCommands(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "--help"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"init", "status", "list", "prune"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q\n%s", want, out)
		}
	}
}

func TestRootHelpDescribesDateFirstArchive(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "--help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "~/.llm/<yyyy-mm-dd>/<repo>[/<task>]/") {
		t.Errorf("help should describe date-first archive layout:\n%s", buf.String())
	}
}

func TestInitHelpDescribesDateFirstArchive(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "init", "--help"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"~/.llm/<yyyy-mm-dd>/<repo>[/<task>]/",
		"By default the bucket is <date>/<repo>",
		"<date>/<repo>/<task>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("init help missing %q\n%s", want, out)
		}
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	a, _ := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "bogus"); err == nil {
		t.Errorf("unknown command should error")
	}
}
