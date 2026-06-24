package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

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
// Date: dateOverride if set, else today, formatted yyyy-mm-dd.
func (a *app) resolve(dir, repoOverride, nameOverride, dateOverride string) (resolved, error) {
	date, err := resolveDate(a.now, dateOverride)
	if err != nil {
		return resolved{}, err
	}

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
		Date: date,
	}, nil
}

func resolveDate(now func() time.Time, dateOverride string) (string, error) {
	if dateOverride == "" {
		return now().Format("2006-01-02"), nil
	}
	parsed, err := time.Parse("2006-01-02", dateOverride)
	if err != nil || parsed.Format("2006-01-02") != dateOverride {
		return "", fmt.Errorf("invalid --date %q: use YYYY-MM-DD", dateOverride)
	}
	return dateOverride, nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
