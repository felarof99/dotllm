package cmd

import (
	"encoding/json"
	"io"

	"github.com/felarof01/dotllm/internal/repo"
)

// resolved is the repo/date/task that a working directory maps to.
type resolved struct {
	Repo string
	Task string
	Date string
}

// resolve computes the repo name, task label, and date for dir.
//
// Repo: repoOverride if set, else the resolver's repo name (sanitized).
// Task: nameOverride (the explicit --name/positional arg) if set, else empty.
// The default is a plain <date> bucket; a task subfolder is only created when
// you pass one.
// Date: today, formatted yyyy-mm-dd.
func (a *app) resolve(dir, repoOverride, nameOverride string) (resolved, error) {
	r := repoOverride
	if r == "" {
		got, err := a.repo.Repo(dir)
		if err != nil {
			return resolved{}, err
		}
		r = got
	}
	return resolved{
		Repo: repo.Sanitize(r),
		Task: repo.Sanitize(nameOverride),
		Date: a.now().Format("2006-01-02"),
	}, nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
