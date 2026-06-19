// Package repo resolves the archive's repo name for a working directory: the
// basename of the git toplevel, falling back to the directory basename.
package repo

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver discovers the repo name and worktree layout for a directory. It is
// an interface so commands can inject a fake instead of shelling out to git.
type Resolver interface {
	// Repo returns the repo name for dir: the main worktree's directory
	// basename (shared by every linked worktree of the repo), or the dir
	// basename when dir is not inside a git work tree.
	Repo(dir string) (string, error)
	// MainRoot returns the repo's main worktree path and whether dir is a
	// *linked* worktree (not the main checkout). For a non-git dir it returns
	// ("", false, nil).
	MainRoot(dir string) (string, bool, error)
}

// Git is the production Resolver; it runs the git binary.
type Git struct{}

// Repo returns the main worktree's directory basename — shared across all of a
// repo's linked worktrees — falling back to the git toplevel basename, then the
// dir basename. Deriving the name from the common git dir (rather than the
// per-worktree toplevel) keeps every worktree of a repo under one archive name.
func (Git) Repo(dir string) (string, error) {
	if common, ok := gitOutput(dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); ok && common != "" {
		return filepath.Base(filepath.Dir(filepath.Clean(common))), nil
	}
	if top, ok := gitOutput(dir, "rev-parse", "--show-toplevel"); ok && top != "" {
		return filepath.Base(top), nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return filepath.Base(abs), nil
}

// MainRoot reports the repo's main worktree path and whether dir is a linked
// worktree. Git's common dir is shared by every worktree of a repo and lives at
// <main-worktree>/.git, so its parent is the main worktree; dir is a *linked*
// worktree when its own toplevel differs from that main worktree.
func (Git) MainRoot(dir string) (string, bool, error) {
	common, ok := gitOutput(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok || common == "" {
		return "", false, nil
	}
	mainRoot := filepath.Dir(filepath.Clean(common))
	top, ok := gitOutput(dir, "rev-parse", "--show-toplevel")
	if !ok || top == "" {
		return mainRoot, false, nil
	}
	return mainRoot, filepath.Clean(top) != mainRoot, nil
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
