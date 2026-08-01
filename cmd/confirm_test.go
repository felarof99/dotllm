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

func TestConfirmDefaultsToAsk(t *testing.T) {
	a, buf := testApp(t, llmDir(t), fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != PolicyAsk {
		t.Errorf("policy = %q, want %q", got, PolicyAsk)
	}
}

// The safety property, stated as a test: without a .llm at all, the answer is
// still "ask". A missing workspace must never read as consent.
func TestConfirmWithoutLLMDirSaysAsk(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != PolicyAsk {
		t.Errorf("policy = %q, want %q", got, PolicyAsk)
	}
}

func TestConfirmAutoThenAsk(t *testing.T) {
	wd := llmDir(t)

	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm", "auto"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != PolicyAuto {
		t.Fatalf("after set auto: policy = %q, want %q", got, PolicyAuto)
	}
	if _, err := os.Stat(sentinelPath(wd)); err != nil {
		t.Fatalf("sentinel should exist after `confirm auto`: %v", err)
	}

	a2, buf2 := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a2, "confirm", "ask"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf2.String()); got != PolicyAsk {
		t.Fatalf("after set ask: policy = %q, want %q", got, PolicyAsk)
	}
	if _, err := os.Stat(sentinelPath(wd)); !os.IsNotExist(err) {
		t.Errorf("`confirm ask` should remove the sentinel, stat err = %v", err)
	}
}

// Setting "ask" when there is no sentinel is a normal no-op, not an error.
func TestConfirmAskIsIdempotent(t *testing.T) {
	a, _ := testApp(t, llmDir(t), fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm", "ask"); err != nil {
		t.Fatalf("`confirm ask` on a clean dir should succeed, got %v", err)
	}
}

// Everything that is not exactly "auto" must read as "ask". These are the
// realistic ways a sentinel goes bad: junk, a truncated write, an empty file,
// a word from some future version.
func TestUnrecognizedSentinelContentReadsAsAsk(t *testing.T) {
	for _, content := range []string{
		"", " ", "\n", "true", "yes", "aut", "auto-ish", "ask", "AUTOMATIC",
		"auto extra", "# comment\nauto", "\x00\x01binary",
	} {
		wd := llmDir(t)
		if err := os.WriteFile(sentinelPath(wd), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		a, buf := testApp(t, wd, fakeRepo{repo: "app"})
		if err := runCmd(a, "confirm"); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(buf.String()); got != PolicyAsk {
			t.Errorf("content %q -> policy %q, want %q", content, got, PolicyAsk)
		}
	}
}

// The accepted spellings of "auto": surrounding whitespace and case are
// tolerated, because a human may well echo it by hand.
func TestAutoSpellings(t *testing.T) {
	for _, content := range []string{"auto", "auto\n", " auto \n", "AUTO", "Auto\r\n"} {
		wd := llmDir(t)
		if err := os.WriteFile(sentinelPath(wd), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		a, buf := testApp(t, wd, fakeRepo{repo: "app"})
		if err := runCmd(a, "confirm"); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(buf.String()); got != PolicyAuto {
			t.Errorf("content %q -> policy %q, want %q", content, got, PolicyAuto)
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
	if err := os.WriteFile(p, []byte("auto\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if got := readPolicy(p); got != PolicyAsk {
		t.Errorf("unreadable sentinel -> %q, want %q", got, PolicyAsk)
	}
}

func TestConfirmRejectsUnknownPolicy(t *testing.T) {
	a, _ := testApp(t, llmDir(t), fakeRepo{repo: "app"})
	err := runCmd(a, "confirm", "maybe")
	if err == nil {
		t.Fatal("expected an error for an unknown policy")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Errorf("error should name the bad value, got %v", err)
	}
}

// `confirm auto` needs a real .llm to write into; it should say so plainly
// instead of silently creating one somewhere unexpected.
func TestConfirmAutoWithoutLLMDirErrors(t *testing.T) {
	a, _ := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	err := runCmd(a, "confirm", "auto")
	if err == nil {
		t.Fatal("expected an error when .llm is missing")
	}
	if !strings.Contains(err.Error(), "dotllm init") {
		t.Errorf("error should point at `dotllm init`, got %v", err)
	}
}

func TestConfirmJSON(t *testing.T) {
	a, buf := testApp(t, llmDir(t), fakeRepo{repo: "app"})
	if err := runCmd(a, "confirm", "--json"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"policy": "ask"`) {
		t.Errorf("json output missing policy: %s", out)
	}
	if !strings.Contains(out, SentinelName) {
		t.Errorf("json output missing sentinel path: %s", out)
	}
}

// A leftover temp file from a crashed write must not be mistaken for the
// sentinel, and must not stop a later write from succeeding.
func TestWritePolicyIsAtomic(t *testing.T) {
	wd := llmDir(t)
	p := sentinelPath(wd)
	if err := writePolicy(p, PolicyAuto); err != nil {
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
	if got := readPolicy(p); got != PolicyAuto {
		t.Errorf("policy = %q, want %q", got, PolicyAuto)
	}
}

var _ repo.Resolver = fakeRepo{}
