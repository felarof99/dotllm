// Package store owns the central home archive: where its root lives, how a
// repo/date/task maps to a canonical workspace path, and how to scan it.
package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Root resolves the archive root as an absolute path: $DOTLLM_HOME (with a
// leading ~ expanded) if set, otherwise <home>/.llm. An absolute result is
// what lets Stat reliably tell a managed link (into the archive) from a foreign
// one.
func Root() (string, error) {
	if v := strings.TrimSpace(os.Getenv("DOTLLM_HOME")); v != "" {
		p, err := expandTilde(v)
		if err != nil {
			return "", err
		}
		return filepath.Abs(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".llm"), nil
}

func expandTilde(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// DirName builds the workspace directory name: "<date>" or "<date>_<task>".
func DirName(date, task string) string {
	if task == "" {
		return date
	}
	return date + "_" + task
}

// WorkspacePath returns the canonical absolute path for a repo/date/task under root.
func WorkspacePath(root, repo, date, task string) string {
	return filepath.Join(root, repo, DirName(date, task))
}

// Workspace is one dated scratch directory inside the archive.
type Workspace struct {
	Repo  string `json:"repo"`
	Name  string `json:"name"`           // "<date>" or "<date>_<task>"
	Date  string `json:"date"`           // "" if Name is not date-prefixed
	Task  string `json:"task,omitempty"` // "" when there is no task label
	Path  string `json:"path"`
	Files int    `json:"files"` // non-directory entries (files + symlinks), counted recursively
}

// RepoGroup is the set of workspaces for one repo.
type RepoGroup struct {
	Repo       string      `json:"repo"`
	Workspaces []Workspace `json:"workspaces"`
}

// Scan reads the archive under root and returns workspaces grouped by repo,
// sorted by repo then workspace name. A missing root yields no groups (no error).
func Scan(root string) ([]RepoGroup, error) {
	repos, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var groups []RepoGroup
	for _, re := range repos {
		if !re.IsDir() {
			continue
		}
		repoDir := filepath.Join(root, re.Name())
		entries, err := os.ReadDir(repoDir)
		if err != nil {
			return nil, err
		}
		var wss []Workspace
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			path := filepath.Join(repoDir, e.Name())
			n, err := countFiles(path)
			if err != nil {
				return nil, err
			}
			date, task := splitName(e.Name())
			wss = append(wss, Workspace{
				Repo:  re.Name(),
				Name:  e.Name(),
				Date:  date,
				Task:  task,
				Path:  path,
				Files: n,
			})
		}
		if len(wss) == 0 {
			continue
		}
		sort.Slice(wss, func(i, j int) bool { return wss[i].Name < wss[j].Name })
		groups = append(groups, RepoGroup{Repo: re.Name(), Workspaces: wss})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Repo < groups[j].Repo })
	return groups, nil
}

// countFiles counts non-directory entries (regular files and symlinks) anywhere
// under dir. Symlinks count so prune never treats a workspace holding curated
// links as "empty" and deletes them.
func countFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

// splitName parses "<date>" or "<date>_<task>" where date is YYYY-MM-DD. If the
// name is not date-prefixed, date is "" and task is "".
func splitName(name string) (date, task string) {
	const dateLen = len("2006-01-02")
	if len(name) < dateLen || !looksLikeDate(name[:dateLen]) {
		return "", ""
	}
	date = name[:dateLen]
	if len(name) > dateLen+1 && name[dateLen] == '_' {
		task = name[dateLen+1:]
	}
	return date, task
}

func looksLikeDate(s string) bool {
	// YYYY-MM-DD shape check; cheap and dependency-free.
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
