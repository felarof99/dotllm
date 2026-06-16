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
	force bool
	quiet bool
	json  bool
}

func newInitCmd(a *app) *cobra.Command {
	var name, repoOverride string
	var force, quiet, jsonOut bool

	cmd := &cobra.Command{
		Use:   "init [task]",
		Short: "Create or re-link this directory's .llm into the home archive",
		Long: `init ensures ~/.llm/<repo>/<yyyy-mm-dd>[_<task>]/ exists and makes ./.llm a
symlink to it. It is idempotent, adopts an existing real ./.llm (moving its
files into the archive without overwriting), and re-attaches if the archive dir
exists but the local link is gone.

By default the bucket is just <repo>/<date>. Pass a task label (via --name or
the positional argument) to get a separate <date>_<task> bucket.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := name
			if task == "" && len(args) > 0 {
				task = args[0]
			}
			return runInit(a, initArgs{
				name:  task,
				repo:  repoOverride,
				force: force,
				quiet: quiet,
				json:  jsonOut,
			})
		},
	}

	f := cmd.Flags()
	f.StringVarP(&name, "name", "n", "", "task label (default: none — a plain <date> bucket)")
	f.StringVar(&repoOverride, "repo", "", "override the auto-detected repo name")
	f.BoolVarP(&force, "force", "f", false, "re-point a .llm symlink that points elsewhere")
	f.BoolVarP(&quiet, "quiet", "q", false, "print nothing on success (for hooks)")
	f.BoolVar(&jsonOut, "json", false, "print the result as JSON")
	return cmd
}

func runInit(a *app, in initArgs) error {
	dir, err := a.wd()
	if err != nil {
		return err
	}
	r, err := a.resolve(dir, in.repo, in.name)
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
