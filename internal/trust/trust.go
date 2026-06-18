// Package trust marks a directory as trusted for the Claude Code and Codex
// CLIs, so their first-run "Do you trust the contents of this directory?"
// prompt never blocks an agent launched autonomously there.
//
// Codex keys trust on the exact directory path in ~/.codex/config.toml
// ([projects."<path>"] / trust_level = "trusted"). Claude Code records it in
// ~/.claude.json under projects["<path>"].hasTrustDialogAccepted. We write both
// idempotently and atomically, preserving every other byte of state we can.
package trust

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodexConfigPath / ClaudeConfigPath are the default config locations under home.
func CodexConfigPath(home string) string  { return filepath.Join(home, ".codex", "config.toml") }
func ClaudeConfigPath(home string) string { return filepath.Join(home, ".claude.json") }

// Result reports what changed for one directory.
type Result struct {
	Path        string `json:"path"`
	CodexAdded  bool   `json:"codex_added"`  // true if a new codex entry was written
	ClaudeAdded bool   `json:"claude_added"` // true if claude trust flags were flipped
}

// Dir trusts projectPath in both Codex and Claude config under home.
func Dir(home, projectPath string) (Result, error) {
	res := Result{Path: projectPath}
	codexAdded, err := Codex(CodexConfigPath(home), projectPath)
	if err != nil {
		return res, fmt.Errorf("codex: %w", err)
	}
	res.CodexAdded = codexAdded
	claudeAdded, err := Claude(ClaudeConfigPath(home), projectPath)
	if err != nil {
		return res, fmt.Errorf("claude: %w", err)
	}
	res.ClaudeAdded = claudeAdded
	return res, nil
}

// Codex ensures [projects."<projectPath>"] with trust_level = "trusted" exists
// in the TOML file at path. It edits textually (appends a table) rather than
// round-tripping TOML, so the rest of the file — model, notify arrays, other
// tables — is left byte-for-byte intact. Returns true if it added the entry.
func Codex(path, projectPath string) (bool, error) {
	header := "[projects." + tomlQuoteKey(projectPath) + "]"

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == header {
			return false, nil // already trusted
		}
	}
	if err := sc.Err(); err != nil {
		return false, err
	}

	var b bytes.Buffer
	b.Write(data)
	if len(data) > 0 {
		if !bytes.HasSuffix(data, []byte("\n")) {
			b.WriteByte('\n')
		}
		b.WriteByte('\n') // blank line before the new table
	}
	fmt.Fprintf(&b, "%s\ntrust_level = \"trusted\"\n", header)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := writeFileAtomic(path, b.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// Claude ensures projects["<projectPath>"].hasTrustDialogAccepted and
// .hasTrustDialogHooksAccepted are true in the JSON state file at path. It
// decodes with UseNumber so large integers keep their exact representation, and
// only ever flips the two trust flags — all other state is preserved. Returns
// true if it changed anything.
func Claude(path, projectPath string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	root := map[string]any{}
	if len(bytes.TrimSpace(data)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&root); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	projects, ok := root["projects"].(map[string]any)
	if !ok || projects == nil {
		projects = map[string]any{}
		root["projects"] = projects
	}
	entry, ok := projects[projectPath].(map[string]any)
	if !ok || entry == nil {
		entry = map[string]any{}
		projects[projectPath] = entry
	}

	changed := false
	for _, k := range []string{"hasTrustDialogAccepted", "hasTrustDialogHooksAccepted"} {
		if v, ok := entry[k].(bool); !ok || !v {
			entry[k] = true
			changed = true
		}
	}
	if !changed {
		return false, nil
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := writeFileAtomic(path, b.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// tomlQuoteKey renders a path as a TOML basic-string key, escaping backslash
// and double-quote (the only two characters that would break the quoting).
func tomlQuoteKey(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// writeFileAtomic writes data to a temp file in the destination directory and
// renames it over path, so a crash mid-write can never truncate the original.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dotllm-trust-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
