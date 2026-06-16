package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusManaged(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "init"); err != nil {
		t.Fatal(err)
	}
	// write a file through the link so the count is non-zero
	if err := os.WriteFile(filepath.Join(wd, ".llm", "x.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := runCmd(a, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "managed") || !strings.Contains(buf.String(), "1 item") {
		t.Errorf("status = %q", buf.String())
	}
}

func TestStatusAbsentDanglingForeign(t *testing.T) {
	// absent
	a1, b1 := testApp(t, t.TempDir(), fakeRepo{repo: "app"})
	_ = runCmd(a1, "status")
	if !strings.Contains(b1.String(), "not initialized") {
		t.Errorf("absent = %q", b1.String())
	}

	// dangling
	wd2 := t.TempDir()
	a2, b2 := testApp(t, wd2, fakeRepo{repo: "app"})
	if err := os.Symlink(filepath.Join(a2.root, "gone"), filepath.Join(wd2, ".llm")); err != nil {
		t.Fatal(err)
	}
	_ = runCmd(a2, "status")
	if !strings.Contains(b2.String(), "dangling") {
		t.Errorf("dangling = %q", b2.String())
	}

	// foreign (real dir)
	wd3 := t.TempDir()
	a3, b3 := testApp(t, wd3, fakeRepo{repo: "app"})
	if err := os.Mkdir(filepath.Join(wd3, ".llm"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = runCmd(a3, "status")
	if !strings.Contains(b3.String(), "foreign") {
		t.Errorf("foreign = %q", b3.String())
	}
}

func TestStatusJSON(t *testing.T) {
	wd := t.TempDir()
	a, buf := testApp(t, wd, fakeRepo{repo: "app"})
	if err := runCmd(a, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	var st struct {
		Kind string `json:"kind"`
		Repo string `json:"repo"`
		Date string `json:"date"`
		Task string `json:"task"`
	}
	if err := json.Unmarshal(buf.Bytes(), &st); err != nil {
		t.Fatalf("json: %v\n%s", err, buf.String())
	}
	if st.Kind != "absent" || st.Repo != "app" || st.Date != "2026-06-14" || st.Task != "" {
		t.Errorf("status json = %+v", st)
	}
}
