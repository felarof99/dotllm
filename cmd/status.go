package cmd

import (
	"fmt"

	"github.com/felarof01/dotllm/internal/store"
	"github.com/felarof01/dotllm/internal/workspace"
	"github.com/spf13/cobra"
)

type statusArgs struct {
	json bool
}

func newStatusCmd(a *app) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show where this directory's .llm points",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(a, statusArgs{json: jsonOut})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print status as JSON")
	return cmd
}

func runStatus(a *app, in statusArgs) error {
	dir, err := a.wd()
	if err != nil {
		return err
	}
	st, err := workspace.Stat(dir, a.root)
	if err != nil {
		return err
	}
	r, err := a.resolve(dir, "", "", "")
	if err != nil {
		return err
	}
	canonical := store.WorkspacePath(a.root, r.Repo, r.Date, r.Task)

	if in.json {
		return printJSON(a.out, map[string]any{
			"kind":      st.Kind,
			"repo":      r.Repo,
			"date":      r.Date,
			"task":      r.Task,
			"local":     st.Local,
			"target":    st.Target,
			"canonical": canonical,
			"files":     st.Files,
		})
	}

	switch st.Kind {
	case workspace.Managed:
		fmt.Fprintf(a.out, "managed: %s -> %s (%d item(s))\n", st.Local, st.Target, st.Files)
	case workspace.Absent:
		fmt.Fprintf(a.out, "not initialized: run `dotllm init` to link %s -> %s\n", st.Local, canonical)
	case workspace.Dangling:
		fmt.Fprintf(a.out, "dangling: %s -> %s (target missing; run `dotllm init` to re-create)\n", st.Local, st.Target)
	case workspace.Foreign:
		if st.IsSymlink {
			fmt.Fprintf(a.out, "foreign: %s -> %s (outside the archive; `dotllm init --force` to re-point)\n", st.Local, st.Target)
		} else {
			fmt.Fprintf(a.out, "foreign: %s is a real directory (run `dotllm init` to adopt it)\n", st.Local)
		}
	}
	return nil
}
