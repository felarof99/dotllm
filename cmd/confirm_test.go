package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felarof01/dotllm/internal/repo"
)

// llmDir makes a working dir that already has a real .llm/ in it.
func llmDir(t *testing.T) string {
	t.Helper()
	wd := t.TempDir()
	if err := os.Mkdir(filepath.Join(wd, ".llm"), 0o755); err != nil {
		t.Fatal(err)
	}
	return wd
}

func sentinelPath(wd string) string {
	return filepath.Join(wd, ".llm", SentinelName)
}

// policyOf runs `dotllm confirm` with the given args and returns the word.
func policyOf(t *testing.T, wd string, args ...string) string {
	t.Helper()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, append([]string{"confirm"}, args...)...); err != nil {
		t.Fatalf("confirm %v: %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func setPolicy(t *testing.T, wd string, args ...string) error {
	t.Helper()
	a, _ := testApp(t, wd, fakeRepo{repo: "app"})
	return runCmd(a, append([]string{"confirm"}, args...)...)
}

func TestConfirmDefaultsToAsk(t *testing.T) {
	if got := policyOf(t, llmDir(t), "--run", "anything"); got != PolicyAsk {
		t.Errorf("policy = %q, want %q", got, PolicyAsk)
	}
}

// Without a .llm at all, the answer is still "ask". A missing workspace must
// never read as consent.
func TestConfirmWithoutLLMDirSaysAsk(t *testing.T) {
	if got := policyOf(t, t.TempDir(), "--run", "x"); got != PolicyAsk {
		t.Errorf("policy = %q, want %q", got, PolicyAsk)
	}
}

// THE finding this scoping exists for: every worktree of a repo shares one
// .llm, so run A's opt-out must not silently cover unrelated run B.
func TestGrantDoesNotLeakToAnotherRun(t *testing.T) {
	wd := llmDir(t)
	if err := setPolicy(t, wd, "auto", "--run", "run-a"); err != nil {
		t.Fatal(err)
	}
	if got := policyOf(t, wd, "--run", "run-a"); got != PolicyAuto {
		t.Errorf("run-a = %q, want %q", got, PolicyAuto)
	}
	if got := policyOf(t, wd, "--run", "run-b"); got != PolicyAsk {
		t.Errorf("run-b = %q, want %q -- a grant leaked across runs", got, PolicyAsk)
	}
	// An unscoped read must not pick up a run-scoped grant either.
	if got := policyOf(t, wd); got != PolicyAsk {
		t.Errorf("unscoped = %q, want %q", got, PolicyAsk)
	}
}

// The blanket grant is available, but only when asked for explicitly.
func TestAutoAllCoversEveryRun(t *testing.T) {
	wd := llmDir(t)
	if err := setPolicy(t, wd, "auto", "--all"); err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{"run-a", "run-b", ""} {
		var got string
		if run == "" {
			got = policyOf(t, wd)
		} else {
			got = policyOf(t, wd, "--run", run)
		}
		if got != PolicyAuto {
			t.Errorf("run %q = %q, want %q", run, got, PolicyAuto)
		}
	}
}

// An unscoped `auto` is refused. Making the blanket case explicit is what
// stops one agent from disabling everyone's prompt by accident.
func TestUnscopedAutoIsRefused(t *testing.T) {
	wd := llmDir(t)
	err := setPolicy(t, wd, "auto")
	if err == nil {
		t.Fatal("expected `confirm auto` with no scope to be refused")
	}
	if !strings.Contains(err.Error(), "--run") || !strings.Contains(err.Error(), "--all") {
		t.Errorf("error should name both scopes, got %v", err)
	}
	if _, statErr := os.Stat(sentinelPath(wd)); !os.IsNotExist(statErr) {
		t.Error("a refused grant must not create a sentinel")
	}
}

func TestBothScopesAtOnceIsRefused(t *testing.T) {
	if err := setPolicy(t, llmDir(t), "auto", "--all", "--run", "x"); err == nil {
		t.Fatal("expected --all with --run to be refused")
	}
}

func TestMultipleRunGrantsCoexist(t *testing.T) {
	wd := llmDir(t)
	for _, r := range []string{"one", "two", "three"} {
		if err := setPolicy(t, wd, "auto", "--run", r); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []string{"one", "two", "three"} {
		if got := policyOf(t, wd, "--run", r); got != PolicyAuto {
			t.Errorf("run %q = %q, want %q", r, got, PolicyAuto)
		}
	}
	if got := policyOf(t, wd, "--run", "four"); got != PolicyAsk {
		t.Errorf("ungranted run = %q, want %q", got, PolicyAsk)
	}
}

func TestGrantIsIdempotent(t *testing.T) {
	wd := llmDir(t)
	for i := 0; i < 3; i++ {
		if err := setPolicy(t, wd, "auto", "--run", "same"); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, l := range readLines(sentinelPath(wd)) {
		if strings.Contains(l, "same") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("grant written %d times, want 1", n)
	}
}

func TestAskClearsEverything(t *testing.T) {
	wd := llmDir(t)
	if err := setPolicy(t, wd, "auto", "--all"); err != nil {
		t.Fatal(err)
	}
	if err := setPolicy(t, wd, "auto", "--run", "keep-me"); err != nil {
		t.Fatal(err)
	}
	if err := setPolicy(t, wd, "ask"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinelPath(wd)); !os.IsNotExist(err) {
		t.Errorf("`confirm ask` should remove the sentinel, stat err = %v", err)
	}
	if got := policyOf(t, wd, "--run", "keep-me"); got != PolicyAsk {
		t.Errorf("after clear: %q, want %q", got, PolicyAsk)
	}
}

func TestAskWithScopeClearsOnlyThatRun(t *testing.T) {
	wd := llmDir(t)
	if err := setPolicy(t, wd, "auto", "--run", "a"); err != nil {
		t.Fatal(err)
	}
	if err := setPolicy(t, wd, "auto", "--run", "b"); err != nil {
		t.Fatal(err)
	}
	if err := setPolicy(t, wd, "ask", "--run", "a"); err != nil {
		t.Fatal(err)
	}
	if got := policyOf(t, wd, "--run", "a"); got != PolicyAsk {
		t.Errorf("cleared run = %q, want %q", got, PolicyAsk)
	}
	if got := policyOf(t, wd, "--run", "b"); got != PolicyAuto {
		t.Errorf("other run = %q, want %q", got, PolicyAuto)
	}
}

func TestConfirmAskIsIdempotent(t *testing.T) {
	if err := setPolicy(t, llmDir(t), "ask"); err != nil {
		t.Fatalf("`confirm ask` on a clean dir should succeed, got %v", err)
	}
}

// Everything that is not an exact grant line must read as "ask". These are the
// realistic ways a sentinel goes bad.
func TestUnrecognizedSentinelContentReadsAsAsk(t *testing.T) {
	for _, content := range []string{
		"", " ", "\n", "auto", "true", "yes", "aut", "auto-ish", "ask",
		"auto al", "auto allx", "autoall", "auto run", "auto run ",
		"# auto all", "auto extra words", "\x00\x01binary",
		"auto run other-run",
	} {
		wd := llmDir(t)
		if err := os.WriteFile(sentinelPath(wd), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := policyOf(t, wd, "--run", "my-run"); got != PolicyAsk {
			t.Errorf("content %q -> %q, want %q", content, got, PolicyAsk)
		}
	}
}

// The accepted spellings: surrounding whitespace and case are tolerated,
// because a human may well write the line by hand.
func TestGrantSpellings(t *testing.T) {
	cases := map[string]string{
		"auto all":                           "any-run",
		"auto all\n":                         "any-run",
		"  AUTO ALL  \n":                     "any-run",
		"auto run my-run":                    "my-run",
		"AUTO RUN My-Run\n":                  "my-run",
		"  auto run my-run  ":                "MY-RUN",
		sentinelHeader + "auto run my-run\n": "my-run",
	}
	for content, run := range cases {
		wd := llmDir(t)
		if err := os.WriteFile(sentinelPath(wd), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := policyOf(t, wd, "--run", run); got != PolicyAuto {
			t.Errorf("content %q run %q -> %q, want %q", content, run, got, PolicyAuto)
		}
	}
}

// An unreadable sentinel must not crash and must not grant consent.
func TestUnreadableSentinelReadsAsAsk(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 is not enforced")
	}
	wd := llmDir(t)
	p := sentinelPath(wd)
	if err := os.WriteFile(p, []byte("auto all\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if got := ReadPolicy(p, "x"); got != PolicyAsk {
		t.Errorf("unreadable sentinel -> %q, want %q", got, PolicyAsk)
	}
}

// A sentinel that is a directory must not crash or grant.
func TestSentinelAsDirectoryReadsAsAsk(t *testing.T) {
	wd := llmDir(t)
	if err := os.Mkdir(sentinelPath(wd), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ReadPolicy(sentinelPath(wd), "x"); got != PolicyAsk {
		t.Errorf("directory sentinel -> %q, want %q", got, PolicyAsk)
	}
}

func TestConfirmRejectsUnknownPolicy(t *testing.T) {
	err := setPolicy(t, llmDir(t), "maybe")
	if err == nil {
		t.Fatal("expected an error for an unknown policy")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Errorf("error should name the bad value, got %v", err)
	}
}

func TestConfirmAutoWithoutLLMDirErrors(t *testing.T) {
	err := setPolicy(t, t.TempDir(), "auto", "--all")
	if err == nil {
		t.Fatal("expected an error when .llm is missing")
	}
	if !strings.Contains(err.Error(), "dotllm init") {
		t.Errorf("error should point at `dotllm init`, got %v", err)
	}
}

func TestConfirmJSON(t *testing.T) {
	a, buf := testApp(t, llmDir(t), fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm", "--run", "abc", "--json"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"policy": "ask"`, `"run": "abc"`, SentinelName} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %s: %s", want, out)
		}
	}
}

// A crashed write must not leave a temp file that a later read could pick up.
func TestWriteIsAtomic(t *testing.T) {
	wd := llmDir(t)
	if err := setPolicy(t, wd, "auto", "--run", "r"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(wd, ".llm"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".confirm-") {
			t.Errorf("temp file %q left behind after write", e.Name())
		}
	}
	if got := ReadPolicy(sentinelPath(wd), "r"); got != PolicyAuto {
		t.Errorf("policy = %q, want %q", got, PolicyAuto)
	}
}

// A run name with a newline could otherwise forge a second grant line.
func TestRunNameCannotInjectALine(t *testing.T) {
	wd := llmDir(t)
	err := setPolicy(t, wd, "auto", "--run", "safe\nauto all")
	if err == nil {
		if got := ReadPolicy(sentinelPath(wd), "anything-else"); got == PolicyAuto {
			t.Fatal("a newline in --run forged a blanket grant")
		}
	}
}

var _ repo.Resolver = fakeRepo{}
