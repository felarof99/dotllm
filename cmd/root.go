package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/felarof01/dotllm/internal/repo"
	"github.com/felarof01/dotllm/internal/store"
	"github.com/spf13/cobra"
)

// app carries injected dependencies so git, $HOME, the clock, and the working
// directory are all fakeable in tests.
type app struct {
	out    io.Writer
	errOut io.Writer
	repo   repo.Resolver
	now    func() time.Time
	root   string // archive root (~/.llm or $DOTLLM_HOME)
	home   string // user home dir (holds ~/.codex/config.toml and ~/.claude.json)
	wd     func() (string, error)
}

func newApp() (*app, error) {
	root, err := store.Root()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &app{
		out:    os.Stdout,
		errOut: os.Stderr,
		repo:   repo.Git{},
		now:    time.Now,
		root:   root,
		home:   home,
		wd:     os.Getwd,
	}, nil
}

func newRootCmdWithApp(a *app) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dotllm",
		Short: "Mirror a project's .llm/ into a central home archive via symlink",
		Long: `dotllm makes a project's .llm/ a symlink into a central archive at
~/.llm/<repo>/<yyyy-mm-dd>[_<task>]/, so agent scratch files auto-track centrally
and survive a deleted checkout.

Run "dotllm init" in any directory; it is idempotent, so it is safe to fire on
every new tmux pane. With no arguments, dotllm prints the current status.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(a, statusArgs{}) // bare invocation -> status
		},
	}
	rootCmd.AddCommand(newInitCmd(a))
	rootCmd.AddCommand(newStatusCmd(a))
	rootCmd.AddCommand(newListCmd(a))
	rootCmd.AddCommand(newPruneCmd(a))
	rootCmd.AddCommand(newTrustCmd(a))
	rootCmd.SetOut(a.out)
	rootCmd.SetErr(a.errOut)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	return rootCmd
}

// Execute runs the CLI, printing errors to stderr and exiting non-zero on failure.
func Execute() {
	a, err := newApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := newRootCmdWithApp(a).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
