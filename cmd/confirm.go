package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// SentinelName is the file inside .llm/ that records the confirm policy.
const SentinelName = "cleanup.sentinel"

// autoWord is the ONLY content that turns confirmation off. Everything else --
// a missing file, an unreadable file, a typo, a half-written file -- reads as
// "ask". That asymmetry is the whole safety property: you cannot lose a
// confirmation prompt by accident, only by writing this exact word on purpose.
const autoWord = "auto"

// Policy values returned by readPolicy.
const (
	PolicyAsk  = "ask"
	PolicyAuto = "auto"
)

type confirmArgs struct {
	set  string // "" = report, else "ask" or "auto"
	json bool
}

func newConfirmCmd(a *app) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "confirm [ask|auto]",
		Short: "Show or set whether tools should ask before destructive cleanup",
		Long: `confirm records one choice for this .llm workspace: should a tool stop and
ask you before it deletes something?

  dotllm confirm         print the current policy -- "ask" or "auto"
  dotllm confirm auto    stop asking; cleanup runs without a prompt
  dotllm confirm ask     go back to asking (the default)

The answer lives in .llm/` + SentinelName + `. "auto" is written only when you
ask for it. Anything else means "ask": no file, an unreadable file, a partly
written file, or a word this version does not recognize. So the safe answer is
what you get when something goes wrong.

Scripts should read the word, not the exit code:

  policy="$(dotllm confirm 2>/dev/null || echo ask)"
  [ "$policy" = auto ] || show_the_prompt`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := confirmArgs{json: jsonOut}
			if len(args) == 1 {
				in.set = args[0]
			}
			return runConfirm(a, in)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the result as JSON")
	return cmd
}

func runConfirm(a *app, in confirmArgs) error {
	dir, err := a.wd()
	if err != nil {
		return err
	}
	sentinel := filepath.Join(dir, ".llm", SentinelName)

	if in.set != "" {
		if err := writePolicy(sentinel, in.set); err != nil {
			return err
		}
	}

	policy := readPolicy(sentinel)
	if in.json {
		return printJSON(a.out, map[string]any{
			"policy":   policy,
			"sentinel": sentinel,
		})
	}
	fmt.Fprintln(a.out, policy)
	return nil
}

// readPolicy answers one question: must a tool ask before destroying work?
//
// It never returns an error. Every failure -- no .llm, no file, no read
// permission, junk content -- answers PolicyAsk, because asking is always safe
// and skipping the prompt is not.
func readPolicy(sentinel string) string {
	b, err := os.ReadFile(sentinel)
	if err != nil {
		return PolicyAsk
	}
	if strings.ToLower(strings.TrimSpace(string(b))) == autoWord {
		return PolicyAuto
	}
	return PolicyAsk
}

// writePolicy sets the policy. "ask" removes the sentinel rather than writing
// a word, so the default state is the absence of a file -- there is nothing to
// misread.
func writePolicy(sentinel, want string) error {
	switch strings.ToLower(strings.TrimSpace(want)) {
	case PolicyAsk:
		if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case PolicyAuto:
		if err := requireLLMDir(sentinel); err != nil {
			return err
		}
		// Write to a temp file and rename, so a crash mid-write can never
		// leave a half-written word that some future reader treats as "auto".
		tmp, err := os.CreateTemp(filepath.Dir(sentinel), ".confirm-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(autoWord + "\n"); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), sentinel)
	default:
		return fmt.Errorf("unknown policy %q: use \"ask\" or \"auto\"", want)
	}
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
