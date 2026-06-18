package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/felarof01/dotllm/internal/trust"
	"github.com/spf13/cobra"
)

type trustArgs struct {
	paths []string
	quiet bool
	json  bool
}

func newTrustCmd(a *app) *cobra.Command {
	var quiet, jsonOut bool

	cmd := &cobra.Command{
		Use:   "trust [path...]",
		Short: "Mark directories as trusted for Claude Code and Codex (skip the trust prompt)",
		Long: `trust pre-approves the current directory (or the given paths) in both
~/.codex/config.toml and ~/.claude.json, so the agents' first-run "Do you trust
the contents of this directory?" prompt never blocks an autonomously launched
agent.

Run it BEFORE launching an agent in a fresh directory or worktree — it should be
the first thing an agent does. It is idempotent, so it is safe to run on every
launch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrust(a, trustArgs{paths: args, quiet: quiet, json: jsonOut})
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&quiet, "quiet", "q", false, "print nothing on success (for hooks)")
	f.BoolVar(&jsonOut, "json", false, "print the result as JSON")
	return cmd
}

func runTrust(a *app, in trustArgs) error {
	paths := in.paths
	if len(paths) == 0 {
		dir, err := a.wd()
		if err != nil {
			return err
		}
		paths = []string{dir}
	}

	results := make([]trust.Result, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		res, err := trust.Dir(a.home, abs)
		if err != nil {
			return fmt.Errorf("%s: %w", abs, err)
		}
		results = append(results, res)
	}

	if in.json {
		return printJSON(a.out, results)
	}
	if in.quiet {
		return nil
	}
	for _, r := range results {
		fmt.Fprintf(a.out, "trusted: %s  (codex: %s, claude: %s)\n",
			r.Path, addedStr(r.CodexAdded), addedStr(r.ClaudeAdded))
	}
	return nil
}

func addedStr(added bool) string {
	if added {
		return "added"
	}
	return "already"
}
