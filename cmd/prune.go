package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/felarof01/dotllm/internal/store"
	"github.com/spf13/cobra"
)

func newPruneCmd(a *app) *cobra.Command {
	var yes, dryRun, jsonOut bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove empty workspace directories from the archive",
		Long: `prune removes empty dated workspace directories (the kind left behind when
init fires on a tmux pane where no files were written) and any repo directory
left empty afterward. It is safe by default: it previews and deletes nothing
unless --yes is given.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(a, yes, dryRun, jsonOut)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&yes, "yes", false, "actually delete (default previews only)")
	f.BoolVar(&dryRun, "dry-run", false, "preview only (never delete)")
	f.BoolVar(&jsonOut, "json", false, "print the prunable set as JSON")
	return cmd
}

func runPrune(a *app, yes, dryRun, jsonOut bool) error {
	groups, err := store.Scan(a.root)
	if err != nil {
		return err
	}
	var empties []store.Workspace
	for _, g := range groups {
		for _, w := range g.Workspaces {
			if w.Files == 0 {
				empties = append(empties, w)
			}
		}
	}

	if jsonOut {
		if empties == nil {
			empties = []store.Workspace{}
		}
		return printJSON(a.out, empties)
	}
	if len(empties) == 0 {
		fmt.Fprintln(a.out, "nothing to prune")
		return nil
	}

	doDelete := yes && !dryRun
	for _, w := range empties {
		if !doDelete {
			fmt.Fprintf(a.out, "would remove %s\n", w.Path)
			continue
		}
		if err := os.RemoveAll(w.Path); err != nil {
			return err
		}
		removeEmptyParents(w.Path, a.root)
		fmt.Fprintf(a.out, "removed %s\n", w.Path)
	}
	if !doDelete {
		fmt.Fprintf(a.out, "\n%d empty workspace(s); re-run with --yes to delete\n", len(empties))
	}
	return nil
}

// removeEmptyParents removes now-empty parents of path, stopping at root.
// Best-effort: any read/remove error leaves the remaining ancestors alone.
func removeEmptyParents(path, root string) {
	root = filepath.Clean(root)
	dir := filepath.Dir(filepath.Clean(path))
	for dir != "." && dir != string(os.PathSeparator) && dir != root {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
