package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedArchive creates <root>/<repo>/<name>/file.md entries.
func seedArchive(t *testing.T, root string, items map[string][]string) {
	t.Helper()
	for repoName, names := range items {
		for _, name := range names {
			p := filepath.Join(root, repoName, name, "note.md")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestListGroupsByRepo(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	seedArchive(t, a.root, map[string][]string{
		"app": {"2026-06-14", "2026-06-13_fix"},
		"web": {"2026-06-10"},
	})
	if err := runCmd(a, "list"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "app") || !strings.Contains(out, "web") ||
		!strings.Contains(out, "2026-06-13_fix") {
		t.Errorf("list = %q", out)
	}
}

func TestListFilterSubstring(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	seedArchive(t, a.root, map[string][]string{
		"browseros": {"2026-06-14"},
		"web":       {"2026-06-10"},
	})
	if err := runCmd(a, "list", "BROWSER"); err != nil { // case-insensitive
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "browseros") || strings.Contains(out, "web") {
		t.Errorf("filtered list = %q", out)
	}
}

func TestListEmpty(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "nothing tracked") {
		t.Errorf("empty list = %q", buf.String())
	}
}

func TestListJSONEmptyIsArray(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	if err := runCmd(a, "list", "--json"); err != nil {
		t.Fatal(err)
	}
	var groups []any
	if err := json.Unmarshal(buf.Bytes(), &groups); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if len(groups) != 0 {
		t.Errorf("groups = %v, want []", groups)
	}
}
