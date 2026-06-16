// Package repo resolves the archive's repo name for a working directory: the
// basename of the git toplevel, falling back to the directory basename.
package repo

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver discovers the repo name for a directory. It is an interface so
// commands can inject a fake instead of shelling out to git.
type Resolver interface {
	// Repo returns the repo name for dir: the git toplevel basename, or the
	// dir basename when dir is not inside a git work tree.
	Repo(dir string) (string, error)
}

// Git is the production Resolver; it runs the git binary.
type Git struct{}

// Repo returns the git toplevel basename, or the dir basename as a fallback.
func (Git) Repo(dir string) (string, error) {
	if top, ok := gitOutput(dir, "rev-parse", "--show-toplevel"); ok && top != "" {
		return filepath.Base(top), nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return filepath.Base(abs), nil
}

// gitOutput runs `git -C dir <args>` and returns trimmed stdout, ok=false on error.
func gitOutput(dir string, args ...string) (string, bool) {
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// Sanitize turns an arbitrary repo/task string into a filesystem-safe slug:
// path separators and whitespace become "-", other unsafe characters are
// dropped, runs of "-" collapse, and leading/trailing "-" and "." are trimmed.
func Sanitize(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '/' || r == '\\' || r == ' ' || r == '\t':
			b.WriteByte('-')
		case r < 0x20 || r == 0x7f:
			// drop control characters
		case r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			// drop characters that are illegal in path components on common filesystems
		default:
			b.WriteRune(r)
		}
	}
	out := collapseDashes(b.String())
	out = strings.Trim(out, "-.")
	return out
}

func collapseDashes(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if !prevDash {
				b.WriteRune(r)
			}
			prevDash = true
			continue
		}
		prevDash = false
		b.WriteRune(r)
	}
	return b.String()
}
