package cmd

import (
	"fmt"

	"github.com/felarof01/dotllm/internal/store"
	"github.com/felarof01/dotllm/internal/workspace"
	"github.com/spf13/cobra"
)

type initArgs struct {
	name  string // explicit task label (flag or positional)
	repo  string // repo override
	date  string // explicit date bucket
	force bool
	quiet bool
	json  bool
}

func newInitCmd(a *app) *cobra.Command {
	var name, repoOverride, projectOverride, dateOverride string
	var force, quiet, jsonOut bool

	cmd := &cobra.Command{
		Use:   "init [task]",
		Short: "Create or re-link this directory's .llm into the home archive",
		Long: `init ensures ~/.llm/<yyyy-mm-dd>/<repo>[/<task>]/ exists and makes ./.llm a
symlink to it. It is idempotent, adopts an existing real ./.llm (moving its
files into the archive without overwriting), and re-attaches if the archive dir
exists but the local link is gone.

By default the bucket is <date>/<repo>. Pass a task label (via --name or the
positional argument) to get a separate <date>/<repo>/<task> bucket.

Pass --project <label> and optional --date <YYYY-MM-DD> from multiple agents to
force the same ~/.llm/<date>/<project> root even when their working directories
or git repos differ. --project is a clearer alias for --repo.

In a linked git worktree, init instead mirrors the main checkout's .llm — it
links to wherever the primary worktree's .llm points (or to its real .llm dir) —
so every worktree of a repo shares one scratch+status tree and tools rooted at
the main .llm (e.g. a daemon) see work done in any worktree. Pass any explicit
root selector (--repo, --project, --date, or --name) to force a distinct bucket.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := name
			if task == "" && len(args) > 0 {
				task = args[0]
			}
			repo, err := chooseRepoOverride(repoOverride, projectOverride)
			if err != nil {
				return err
			}
			return runInit(a, initArgs{
				name:  task,
				repo:  repo,
				date:  dateOverride,
				force: force,
				quiet: quiet,
				json:  jsonOut,
			})
		},
	}

	f := cmd.Flags()
	f.StringVarP(&name, "name", "n", "", "task label (default: none — a plain <date> bucket)")
	f.StringVar(&repoOverride, "repo", "", "override the auto-detected repo name")
	f.StringVar(&projectOverride, "project", "", "shared high-level project label (alias for --repo)")
	f.StringVar(&dateOverride, "date", "", "archive date bucket (YYYY-MM-DD; default: today)")
	f.BoolVarP(&force, "force", "f", false, "re-point a .llm symlink that points elsewhere")
	f.BoolVarP(&quiet, "quiet", "q", false, "print nothing on success (for hooks)")
	f.BoolVar(&jsonOut, "json", false, "print the result as JSON")
	return cmd
}

func chooseRepoOverride(repoOverride, projectOverride string) (string, error) {
	if repoOverride != "" && projectOverride != "" && repoOverride != projectOverride {
		return "", fmt.Errorf("--repo and --project both set; pass only one shared project label")
	}
	if projectOverride != "" {
		return projectOverride, nil
	}
	return repoOverride, nil
}

func runInit(a *app, in initArgs) error {
	dir, err := a.wd()
	if err != nil {
		return err
	}

	// A linked worktree shares its main checkout's .llm, so every worktree of a
	// repo — and a daemon/tool rooted at the main .llm — see one scratch+status
	// tree. (Skip when explicit root selectors ask for a distinct bucket.)
	if in.repo == "" && in.name == "" && in.date == "" {
		mainRoot, isWorktree, err := a.repo.MainRoot(dir)
		if err != nil {
			return err
		}
		if isWorktree {
			if canonical, ok := mainLLMTarget(a.root, mainRoot); ok {
				res, err := workspace.Init(workspace.InitOptions{Dir: dir, Canonical: canonical, Force: in.force})
				if err != nil {
					return err
				}
				return reportInit(a, in, res)
			}
			// main has no .llm yet: fall through to the normal bucket. With the
			// shared repo name, that's the same dir main adopts on its own init.
		}
	}

	r, err := a.resolve(dir, in.repo, in.name, in.date)
	if err != nil {
		return err
	}
	if r.Repo == "" {
		return fmt.Errorf("could not determine a repo name for %s (pass --repo)", dir)
	}

	canonical := store.WorkspacePath(a.root, r.Repo, r.Date, r.Task)
	res, err := workspace.Init(workspace.InitOptions{Dir: dir, Canonical: canonical, Force: in.force})
	if err != nil {
		return err
	}
	if r.Task != "" {
		if err := store.EnsureTaskMarker(canonical); err != nil {
			return err
		}
	}
	return reportInit(a, in, res)
}

// mainLLMTarget resolves where a worktree should point its .llm to mirror the
// main checkout's: the link's target if main's .llm is a symlink, else the real
// .llm directory itself (e.g. a tool's root that isn't dotllm-managed). ok is
// false when main has no .llm yet.
func mainLLMTarget(root, mainRoot string) (string, bool) {
	st, err := workspace.Stat(mainRoot, root)
	if err != nil || st.Kind == workspace.Absent {
		return "", false
	}
	if st.IsSymlink && st.Target != "" {
		return st.Target, true
	}
	return st.Local, true
}

func reportInit(a *app, in initArgs, res workspace.InitResult) error {
	if in.json {
		return printJSON(a.out, res)
	}
	if in.quiet {
		return nil
	}
	switch {
	case res.AlreadyOK:
		fmt.Fprintf(a.out, "already linked: %s -> %s\n", res.Local, res.Canonical)
	case len(res.Adopted) > 0:
		fmt.Fprintf(a.out, "adopted %d item(s), linked: %s -> %s\n", len(res.Adopted), res.Local, res.Canonical)
	default:
		fmt.Fprintf(a.out, "linked: %s -> %s\n", res.Local, res.Canonical)
	}
	return nil
}
