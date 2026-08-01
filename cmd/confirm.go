package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// SentinelName is the file inside .llm/ that records the confirm policy.
const SentinelName = "cleanup.sentinel"

// Policy values returned by ReadPolicy.
const (
	PolicyAsk  = "ask"
	PolicyAuto = "auto"
)

// Two grant lines are recognised, and nothing else:
//
//	auto all            -> every run in this workspace
//	auto run <name>     -> only the run called <name>
//
// Any other line is ignored. A missing file, an unreadable file, a truncated
// write, or a word from some future version therefore all mean "ask". You can
// only lose a confirmation prompt on purpose.
const (
	grantAll    = "auto all"
	grantPrefix = "auto run "
)

const sentinelHeader = `# dotllm cleanup policy.
# Only the exact lines below turn confirmation off:
#   auto all         -> every run in this workspace
#   auto run <name>  -> just that run
# Anything else means "ask". Delete this file to go back to asking.
`

type confirmArgs struct {
	set  string // "" = report, else "ask" or "auto"
	run  string // scope: a run name
	all  bool   // scope: everything in this workspace
	json bool
}

func newConfirmCmd(a *app) *cobra.Command {
	var runName string
	var all, jsonOut bool

	cmd := &cobra.Command{
		Use:   "confirm [ask|auto]",
		Short: "Show or set whether tools should ask before destructive cleanup",
		Long: `confirm records whether a tool should stop and ask you before it deletes
something -- a worktree, a branch, a background agent.

  dotllm confirm --run build-parser     is THIS run allowed to skip the prompt?
  dotllm confirm auto --run build-parser   let that one run skip it
  dotllm confirm auto --all             let every run here skip it
  dotllm confirm ask                    go back to asking for everything

The answer lives in .llm/` + SentinelName + `.

Turning it off is always scoped, and you have to say which scope you mean.
That is deliberate: one agent switching confirmation off for its own run must
never switch it off for some other agent's run that you never thought about.

Everything that is not an exact grant means "ask" -- no file, no .llm, an
unreadable file, a half-written file, or a run name that does not match.

Scripts should read the word, not the exit code:

  policy="$(dotllm confirm --run "$slug" 2>/dev/null || echo ask)"
  [ "$policy" = auto ] || show_the_prompt`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := confirmArgs{run: runName, all: all, json: jsonOut}
			if len(args) == 1 {
				in.set = args[0]
			}
			return runConfirm(a, in)
		},
	}

	f := cmd.Flags()
	f.StringVar(&runName, "run", "", "scope: a single run name")
	f.BoolVar(&all, "all", false, "scope: every run in this workspace")
	f.BoolVar(&jsonOut, "json", false, "print the result as JSON")
	return cmd
}

func runConfirm(a *app, in confirmArgs) error {
	dir, err := a.wd()
	if err != nil {
		return err
	}
	sentinel := filepath.Join(dir, ".llm", SentinelName)

	if in.set != "" {
		if err := writePolicy(sentinel, in); err != nil {
			return err
		}
	}

	policy := ReadPolicy(sentinel, in.run)
	if in.json {
		return printJSON(a.out, map[string]any{
			"policy":   policy,
			"run":      in.run,
			"sentinel": sentinel,
		})
	}
	fmt.Fprintln(a.out, policy)
	return nil
}

// ReadPolicy answers one question: must a tool ask before destroying this run?
//
// It never returns an error. Every failure -- no .llm, no file, no read
// permission, junk content, a run name that does not match -- answers
// PolicyAsk, because asking is always safe and skipping the prompt is not.
//
// An empty run name matches only a blanket "auto all" grant.
func ReadPolicy(sentinel, run string) string {
	f, err := os.Open(sentinel)
	if err != nil {
		return PolicyAsk
	}
	defer f.Close()

	want := grantPrefix + strings.ToLower(strings.TrimSpace(run))
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.ToLower(strings.TrimSpace(sc.Text()))
		if line == grantAll {
			return PolicyAuto
		}
		if run != "" && line == want {
			return PolicyAuto
		}
	}
	// A read error partway through is still a failure, and failures mean ask.
	if sc.Err() != nil {
		return PolicyAsk
	}
	return PolicyAsk
}

func writePolicy(sentinel string, in confirmArgs) error {
	switch strings.ToLower(strings.TrimSpace(in.set)) {
	case PolicyAsk:
		return clearGrant(sentinel, in)
	case PolicyAuto:
		return addGrant(sentinel, in)
	default:
		return fmt.Errorf("unknown policy %q: use \"ask\" or \"auto\"", in.set)
	}
}

// grantLine turns the requested scope into the one line that encodes it.
// Refusing an unscoped "auto" is the point: a blanket grant must be typed as
// a blanket grant.
func grantLine(in confirmArgs) (string, error) {
	switch {
	case in.all && in.run != "":
		return "", fmt.Errorf("choose one scope: --all or --run, not both")
	case in.all:
		return grantAll, nil
	case in.run != "":
		name := strings.ToLower(strings.TrimSpace(in.run))
		if name == "" || strings.ContainsAny(name, "\n\r") {
			return "", fmt.Errorf("invalid --run name %q", in.run)
		}
		return grantPrefix + name, nil
	default:
		return "", fmt.Errorf("say which runs may skip the prompt: --run <name>, or --all")
	}
}

func addGrant(sentinel string, in confirmArgs) error {
	line, err := grantLine(in)
	if err != nil {
		return err
	}
	if err := requireLLMDir(sentinel); err != nil {
		return err
	}

	lines := readLines(sentinel)
	for _, l := range lines {
		if strings.ToLower(strings.TrimSpace(l)) == line {
			return nil // already granted
		}
	}
	return writeLines(sentinel, append(lines, line))
}

// clearGrant with a scope removes just that grant; without one it removes the
// whole file, so "dotllm confirm ask" always fully resets.
func clearGrant(sentinel string, in confirmArgs) error {
	if !in.all && in.run == "" {
		if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	line, err := grantLine(in)
	if err != nil {
		return err
	}
	kept := make([]string, 0)
	for _, l := range readLines(sentinel) {
		if strings.ToLower(strings.TrimSpace(l)) != line {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeLines(sentinel, kept)
}

// readLines returns the sentinel's grant lines, dropping comments and blanks.
// An unreadable file reads as no grants, which is the safe direction.
func readLines(sentinel string) []string {
	f, err := os.Open(sentinel)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// writeLines writes through a temp file and renames, so a crash mid-write can
// never leave a half-written line that a later read might accept.
func writeLines(sentinel string, lines []string) error {
	tmp, err := os.CreateTemp(filepath.Dir(sentinel), ".confirm-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	body := sentinelHeader
	for _, l := range lines {
		body += l + "\n"
	}
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), sentinel)
}

func requireLLMDir(sentinel string) error {
	dir := filepath.Dir(sentinel)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("no .llm workspace at %s: run `dotllm init` first", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}
