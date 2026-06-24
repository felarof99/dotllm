# dotllm

Mirror a project's `.llm/` scratch directory into one central, browsable archive
in your home directory — so the files you ask your agent to drop in `.llm/`
auto-track centrally and survive a deleted checkout.

## Model

`dotllm` makes a project's `.llm/` a **symlink** into a home archive:

```
project/.llm  ->  ~/.llm/<repo>/<yyyy-mm-dd>[_<task>]/
```

The **home copy is the source of truth**. Because `.llm/` is a symlink, writing
`project/.llm/notes.md` physically writes into the archive — the "sync" is
instant and copy-free, and removing the local symlink never loses data.

### How the path is built

- **`<repo>`** — basename of the git toplevel (`git rev-parse --show-toplevel`),
  or the current directory's basename when you're not in a git work tree.
  Override with `--repo`, or use `--project` when you want a shared high-level
  label that is not tied to the checkout name.
- **`<yyyy-mm-dd>`** — today's date. ISO format so the archive sorts by time.
  Override with `--date YYYY-MM-DD` when multiple agents need to pin the same
  workspace root.
- **`<task>`** — optional, omitted by default. Pass a label via `--name` or the
  positional argument to get a separate `<date>_<task>` bucket — handy when you
  run several distinct tasks in one repo on one day.

By default the bucket is just `<repo>/<date>`, so every pane you open in a repo
on the same day shares one workspace and auto-init (below) creates at most one
empty dir per repo/day — `dotllm prune` cleans those up.

For agents launched from different checkouts or directories, pass the same
project/date pair:

```sh
dotllm init --project BrowserOS --date 2026-06-23
```

Both agents will link `./.llm` to `~/.llm/BrowserOS/2026-06-23/`.

## Install

```sh
make install      # builds to $(go env GOPATH)/bin/dotllm
```

## Commands

```
dotllm init [task]     Create/re-link this dir's .llm into the archive.
                       -n, --name <task>   task label (default: none -> plain <date> bucket)
                           --repo <name>   override the detected repo name
                           --project <label>
                                           shared high-level project label (alias for --repo)
                           --date <yyyy-mm-dd>
                                           override the date bucket (default: today)
                       -f, --force         re-point a .llm that points elsewhere
                       -q, --quiet         no output on success (for hooks)
                           --json          print the result as JSON

dotllm status          Show where ./.llm points: managed / not initialized /
                       dangling / foreign.  --json for machine output.

dotllm list [substr]   Browse the archive, grouped by repo, with file counts.
                       Optional case-insensitive repo-substring filter.  --json.

dotllm prune           Remove empty workspace dirs (and now-empty repo parents).
                       Safe by default: previews unless --yes.  --dry-run, --json.

dotllm                 With no arguments, prints status.
```

`dotllm init` is **idempotent** and **non-destructive**:

- Re-running when already linked is a no-op.
- If `./.llm` is a real directory with files, its contents are **moved** into the
  archive (never overwriting an existing archive file — a name clash aborts the
  adoption and tells you which files conflict) and it's replaced with the symlink.
- If the archive dir exists but the local link is gone, it's re-created.

## Config

- `DOTLLM_HOME` — archive root. Defaults to `~/.llm`. A leading `~` is expanded.

## Auto-init on every tmux pane

`dotllm init -q` is fast and idempotent, so wire it into new panes/windows.

**tmux** (`~/.tmux.conf`) — run it when you split or open a window:

```tmux
bind '"' split-window -c "#{pane_current_path}" \; send-keys 'dotllm init -q' Enter
bind '%' split-window -h -c "#{pane_current_path}" \; send-keys 'dotllm init -q' Enter
bind c  new-window -c "#{pane_current_path}" \; send-keys 'dotllm init -q' Enter
```

**Shell** (`~/.zshrc`) — link on demand without making empty dirs everywhere; this
inits only when you `cd` into a directory that already has a `.llm`:

```sh
dotllm_maybe() { [ -e .llm ] && dotllm init -q; }
chpwd_functions+=(dotllm_maybe)
```

Or just run `dotllm init` by hand when you start a task. `dotllm` never edits
your tmux or shell config itself — copy what you want.

## Caveats

- Two different repos that share a basename (e.g. two `app/`) share one archive
  bucket. Use `--repo` to disambiguate.
- Linked git **worktrees** share by default: `init` mirrors the main checkout's
  `.llm` when it exists, and otherwise falls back to the main checkout's repo
  name. Use an explicit root selector (`--repo`, `--project`, `--date`, or
  `--name`) when you want a distinct canonical bucket instead of mirroring.
- Add `.llm` to your project's `.gitignore` if you don't want the symlink tracked.
