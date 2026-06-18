package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexAppendsPreservingExisting(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	seed := "model = \"gpt-5.5\"\n\n[projects.\"/already\"]\ntrust_level = \"trusted\"\n\n[features]\napps = true\n"
	if err := os.WriteFile(cfg, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	added, err := Codex(cfg, "/Users/x/proj")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added=true")
	}

	got, _ := os.ReadFile(cfg)
	s := string(got)
	for _, want := range []string{
		"model = \"gpt-5.5\"",
		"[projects.\"/already\"]",
		"[features]",
		"[projects.\"/Users/x/proj\"]\ntrust_level = \"trusted\"",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}

	// idempotent: second call is a no-op
	added2, err := Codex(cfg, "/Users/x/proj")
	if err != nil {
		t.Fatal(err)
	}
	if added2 {
		t.Error("expected idempotent second call (added=false)")
	}
}

func TestCodexCreatesFileWhenMissing(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".codex", "config.toml")
	added, err := Codex(cfg, "/p")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added")
	}
	got, _ := os.ReadFile(cfg)
	if !strings.Contains(string(got), "[projects.\"/p\"]") {
		t.Errorf("missing entry:\n%s", got)
	}
}

func TestClaudeSetsTrustPreservingData(t *testing.T) {
	js := filepath.Join(t.TempDir(), ".claude.json")
	// A large integer + a float + a sibling project: all must survive intact.
	seed := `{"userID":"abc","numStartups":1718900000000,` +
		`"projects":{"/other":{"hasTrustDialogAccepted":true,"lastCost":1.5}}}`
	if err := os.WriteFile(js, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	added, err := Claude(js, "/Users/x/proj")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added")
	}

	root := decodeNumberJSON(t, js)
	if root["userID"] != "abc" {
		t.Errorf("lost userID: %v", root["userID"])
	}
	if n, ok := root["numStartups"].(json.Number); !ok || n.String() != "1718900000000" {
		t.Errorf("integer precision lost: %v (%T)", root["numStartups"], root["numStartups"])
	}
	projects := root["projects"].(map[string]any)
	if _, ok := projects["/other"]; !ok {
		t.Error("lost sibling project /other")
	}
	entry := projects["/Users/x/proj"].(map[string]any)
	if entry["hasTrustDialogAccepted"] != true || entry["hasTrustDialogHooksAccepted"] != true {
		t.Errorf("trust flags not set: %v", entry)
	}

	// idempotent
	added2, err := Claude(js, "/Users/x/proj")
	if err != nil {
		t.Fatal(err)
	}
	if added2 {
		t.Error("expected idempotent second call")
	}
}

func TestClaudeCreatesWhenMissing(t *testing.T) {
	js := filepath.Join(t.TempDir(), ".claude.json")
	added, err := Claude(js, "/p")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added")
	}
	root := decodeNumberJSON(t, js)
	entry := root["projects"].(map[string]any)["/p"].(map[string]any)
	if entry["hasTrustDialogAccepted"] != true {
		t.Errorf("trust not set: %v", entry)
	}
}

func decodeNumberJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		t.Fatal(err)
	}
	return root
}
