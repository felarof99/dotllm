// Package store owns the central home archive: where its root lives, how a
// date/repo/task maps to a canonical workspace path, and how to scan it.
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

// WorkspacePath returns the canonical absolute path for a date/repo/task under root.
func WorkspacePath(root, repo, date, task string) string {
	if task == "" {
		return filepath.Join(root, date, repo)
	}
	return filepath.Join(root, date, repo, task)
}

// Workspace is one scratch directory inside the archive.
type Workspace struct {
	Repo   string `json:"repo"`
	Name   string `json:"name"`           // "<repo>", "<repo>/<task>", or legacy "<repo>/<date>[_<task>]"
	Date   string `json:"date"`           // YYYY-MM-DD
	Task   string `json:"task,omitempty"` // "" when there is no task label
	Path   string `json:"path"`
	Files  int    `json:"files"` // non-directory entries (files + symlinks), counted recursively
	Legacy bool   `json:"legacy,omitempty"`
}

// DateGroup is the set of workspaces for one date. Legacy repo-first archives
// are returned in a trailing group with Legacy=true.
type DateGroup struct {
	Date       string      `json:"date,omitempty"`
	Legacy     bool        `json:"legacy,omitempty"`
	Workspaces []Workspace `json:"workspaces"`
}

// Scan reads the archive under root and returns date-first workspaces grouped by
// date descending. Legacy repo-first archives are still surfaced in a trailing
// group so old data remains visible and prunable. A missing root yields no
// groups (no error).
func Scan(root string) ([]DateGroup, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var groups []DateGroup
	var legacy []Workspace
	for _, re := range entries {
		if !re.IsDir() {
			continue
		}
		path := filepath.Join(root, re.Name())
		if looksLikeDate(re.Name()) {
			wss, err := scanDateFirstGroup(path, re.Name())
			if err != nil {
				return nil, err
			}
			if len(wss) > 0 {
				groups = append(groups, DateGroup{Date: re.Name(), Workspaces: wss})
			}
			continue
		}
		wss, err := scanLegacyRepo(path, re.Name())
		if err != nil {
			return nil, err
		}
		legacy = append(legacy, wss...)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Date > groups[j].Date })
	sortLegacy(legacy)
	if len(legacy) > 0 {
		groups = append(groups, DateGroup{Legacy: true, Workspaces: legacy})
	}
	return groups, nil
}

func scanDateFirstGroup(dateDir, date string) ([]Workspace, error) {
	repos, err := os.ReadDir(dateDir)
	if err != nil {
		return nil, err
	}
	var wss []Workspace
	for _, re := range repos {
		if !re.IsDir() {
			continue
		}
		got, err := scanDateRepo(filepath.Join(dateDir, re.Name()), date, re.Name())
		if err != nil {
			return nil, err
		}
		wss = append(wss, got...)
	}
	sortWorkspaces(wss)
	return wss, nil
}

func scanDateRepo(repoDir, date, repo string) ([]Workspace, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}
	var dirs []os.DirEntry
	hasDirectFile := false
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
			continue
		}
		hasDirectFile = true
	}

	var wss []Workspace
	if len(entries) == 0 || hasDirectFile {
		n, err := countFiles(repoDir)
		if err != nil {
			return nil, err
		}
		wss = append(wss, Workspace{
			Repo:  repo,
			Name:  repo,
			Date:  date,
			Path:  repoDir,
			Files: n,
		})
	}
	for _, d := range dirs {
		path := filepath.Join(repoDir, d.Name())
		n, err := countFiles(path)
		if err != nil {
			return nil, err
		}
		wss = append(wss, Workspace{
			Repo:  repo,
			Name:  repo + "/" + d.Name(),
			Date:  date,
			Task:  d.Name(),
			Path:  path,
			Files: n,
		})
	}
	return wss, nil
}

func scanLegacyRepo(repoDir, repo string) ([]Workspace, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}
	var wss []Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		date, task := splitName(e.Name())
		if date == "" {
			continue
		}
		path := filepath.Join(repoDir, e.Name())
		n, err := countFiles(path)
		if err != nil {
			return nil, err
		}
		wss = append(wss, Workspace{
			Repo:   repo,
			Name:   repo + "/" + e.Name(),
			Date:   date,
			Task:   task,
			Path:   path,
			Files:  n,
			Legacy: true,
		})
	}
	return wss, nil
}

func sortWorkspaces(wss []Workspace) {
	sort.Slice(wss, func(i, j int) bool {
		if wss[i].Repo != wss[j].Repo {
			return wss[i].Repo < wss[j].Repo
		}
		return wss[i].Task < wss[j].Task
	})
}

func sortLegacy(wss []Workspace) {
	sort.Slice(wss, func(i, j int) bool {
		if wss[i].Date != wss[j].Date {
			return wss[i].Date > wss[j].Date
		}
		if wss[i].Repo != wss[j].Repo {
			return wss[i].Repo < wss[j].Repo
		}
		return wss[i].Task < wss[j].Task
	})
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
