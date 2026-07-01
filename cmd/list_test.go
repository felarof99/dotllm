package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedArchive creates <root>/<date>/<repo>[/<task>]/note.md entries.
func seedArchive(t *testing.T, root string, items map[string][]string) {
	t.Helper()
	for date, names := range items {
		for _, name := range names {
			p := filepath.Join(root, date, filepath.FromSlash(name), "note.md")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(name, "/") {
				if err := os.WriteFile(filepath.Join(root, date, filepath.FromSlash(name), ".dotllm-task"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func seedLegacyArchive(t *testing.T, root string, items map[string][]string) {
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

func TestListGroupsByRecentDate(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	seedArchive(t, a.root, map[string][]string{
		"2026-06-14": {"app", "web/fix"},
		"2026-06-10": {"web"},
	})
	if err := runCmd(a, "list"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "2026-06-14") || !strings.Contains(out, "app") ||
		!strings.Contains(out, "web/fix") {
		t.Errorf("list = %q", out)
	}
	if strings.Index(out, "2026-06-14") > strings.Index(out, "2026-06-10") {
		t.Errorf("recent date should print first: %q", out)
	}
}

func TestListFilterSubstring(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	seedArchive(t, a.root, map[string][]string{
		"2026-06-14": {"browseros"},
		"2026-06-10": {"web"},
	})
	if err := runCmd(a, "list", "BROWSER"); err != nil { // case-insensitive
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "browseros") || strings.Contains(out, "web") {
		t.Errorf("filtered list = %q", out)
	}
}

func TestListShowsLegacyAfterDateFirst(t *testing.T) {
	a, buf := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	seedArchive(t, a.root, map[string][]string{
		"2026-06-14": {"app"},
	})
	seedLegacyArchive(t, a.root, map[string][]string{
		"legacyapp": {"2026-06-13_fix"},
	})
	if err := runCmd(a, "list"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "legacy repo-first archives") || !strings.Contains(out, "legacyapp/2026-06-13_fix") {
		t.Errorf("legacy list = %q", out)
	}
	if strings.Index(out, "legacy repo-first archives") < strings.Index(out, "2026-06-14") {
		t.Errorf("legacy section should print after date-first archives: %q", out)
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
