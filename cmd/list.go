package cmd

import (
	"fmt"
	"strings"

	"github.com/felarof01/dotllm/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd(a *app) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list [repo-substring]",
		Short: "Browse tracked workspaces in the home archive",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			return runList(a, filter, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the listing as JSON")
	return cmd
}

func runList(a *app, filter string, jsonOut bool) error {
	groups, err := store.Scan(a.root)
	if err != nil {
		return err
	}
	if filter != "" {
		lf := strings.ToLower(filter)
		kept := make([]store.DateGroup, 0, len(groups))
		for _, g := range groups {
			wss := make([]store.Workspace, 0, len(g.Workspaces))
			for _, w := range g.Workspaces {
				if strings.Contains(strings.ToLower(w.Repo), lf) {
					wss = append(wss, w)
				}
			}
			if len(wss) > 0 {
				g.Workspaces = wss
				kept = append(kept, g)
			}
		}
		groups = kept
	}

	if jsonOut {
		if groups == nil {
			groups = []store.DateGroup{}
		}
		return printJSON(a.out, groups)
	}
	if len(groups) == 0 {
		fmt.Fprintln(a.out, "nothing tracked yet")
		return nil
	}
	for _, g := range groups {
		if g.Legacy {
			fmt.Fprintln(a.out, "legacy repo-first archives")
		} else {
			fmt.Fprintln(a.out, g.Date)
		}
		for _, w := range g.Workspaces {
			fmt.Fprintf(a.out, "  %s  (%d item(s))\n", w.Name, w.Files)
		}
	}
	return nil
}
