// Package workspace links a project's local .llm into the home archive and
// reports the link's state. The home copy is the source of truth: Init moves
// an existing real .llm into the archive (never overwriting) and replaces it
// with a symlink, so writing through ./.llm lands bytes directly in the archive.
package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LocalName is the directory name linked in each project.
const LocalName = ".llm"

// renameFunc is the move primitive; overridable in tests to exercise the
// cross-device copy-then-remove fallback.
var renameFunc = os.Rename

// copyFileFunc is the regular-file copy primitive; overridable in tests to
// simulate a mid-copy failure on the cross-device path.
var copyFileFunc = copyFile

// Kind classifies the local .llm.
type Kind string

const (
	Absent   Kind = "absent"   // no .llm present
	Managed  Kind = "managed"  // symlink into the archive
	Dangling Kind = "dangling" // symlink whose target is missing
	Foreign  Kind = "foreign"  // a real dir/file, or a symlink outside the archive
)

// Status is the result of inspecting a directory's .llm.
type Status struct {
	Kind      Kind   `json:"kind"`
	Local     string `json:"local"`            // path to ./.llm
	Target    string `json:"target,omitempty"` // symlink target (resolved), if any
	IsSymlink bool   `json:"is_symlink"`
	Files     int    `json:"files"` // regular files under the target, when Managed
}

// Stat inspects dir/.llm, using root to tell Managed from Foreign symlinks.
func Stat(dir, root string) (Status, error) {
	local := filepath.Join(dir, LocalName)
	st := Status{Local: local, Kind: Absent}

	fi, err := os.Lstat(local)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		// A real directory or file occupying the .llm name.
		st.Kind = Foreign
		return st, nil
	}

	st.IsSymlink = true
	target, err := resolveLink(dir, local)
	if err != nil {
		return st, err
	}
	st.Target = target

	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			st.Kind = Dangling
			return st, nil
		}
		return st, err
	}

	// Compare real paths so a symlinked component in DOTLLM_HOME (e.g. macOS
	// /var -> /private/var) doesn't make a managed link look foreign.
	if isUnder(evalSymlinks(target), evalSymlinks(root)) {
		st.Kind = Managed
		st.Files = countFiles(target)
	} else {
		st.Kind = Foreign
	}
	return st, nil
}

// evalSymlinks resolves symlinks in p, returning p unchanged if it can't.
func evalSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// InitOptions configures Init.
type InitOptions struct {
	Dir       string // working directory whose .llm to manage
	Canonical string // ~/.llm/<date>/<repo>[/task] (absolute)
	Force     bool   // re-point a .llm symlink that points elsewhere
}

// InitResult reports what Init did.
type InitResult struct {
	Local     string   `json:"local"`
	Canonical string   `json:"canonical"`
	Linked    bool     `json:"linked"`     // a new/updated symlink was created
	AlreadyOK bool     `json:"already_ok"` // was already correctly linked
	Adopted   []string `json:"adopted,omitempty"`
}

// Init ensures the canonical archive dir exists and that dir/.llm is a symlink
// to it. Behavior by current state of dir/.llm:
//   - absent: create the archive dir and the symlink.
//   - correct symlink: no-op.
//   - other symlink: error unless Force, which re-points it.
//   - real directory: adopt — move its contents into the archive (never
//     overwriting an existing archive file), then replace it with the symlink.
//   - real file: error.
func Init(opts InitOptions) (InitResult, error) {
	canonical, err := filepath.Abs(opts.Canonical)
	if err != nil {
		return InitResult{}, err
	}
	local := filepath.Join(opts.Dir, LocalName)
	res := InitResult{Local: local, Canonical: canonical}

	fi, err := os.Lstat(local)
	switch {
	case err != nil && os.IsNotExist(err):
		if err := ensureLink(canonical, local); err != nil {
			return res, err
		}
		res.Linked = true
		return res, nil

	case err != nil:
		return res, err

	case fi.Mode()&os.ModeSymlink != 0:
		return initSymlink(opts, res, local, canonical)

	case fi.IsDir():
		return initAdopt(res, local, canonical)

	default:
		return res, fmt.Errorf("%s exists as a file; remove it and re-run", local)
	}
}

func initSymlink(opts InitOptions, res InitResult, local, canonical string) (InitResult, error) {
	target, err := resolveLink(opts.Dir, local)
	if err != nil {
		return res, err
	}
	if filepath.Clean(target) == canonical {
		if err := os.MkdirAll(canonical, 0o755); err != nil {
			return res, err
		}
		res.AlreadyOK = true
		return res, nil
	}
	if !opts.Force {
		return res, fmt.Errorf("%s already points to %s; use --force to re-point it to %s", local, target, canonical)
	}
	if err := os.Remove(local); err != nil {
		return res, err
	}
	if err := ensureLink(canonical, local); err != nil {
		return res, err
	}
	res.Linked = true
	return res, nil
}

func initAdopt(res InitResult, local, canonical string) (InitResult, error) {
	entries, err := os.ReadDir(local)
	if err != nil {
		return res, err
	}

	// Pre-check for conflicts so adoption is all-or-nothing: never move a file
	// when a same-named entry already exists in the archive.
	var conflicts []string
	for _, e := range entries {
		if _, err := os.Lstat(filepath.Join(canonical, e.Name())); err == nil {
			conflicts = append(conflicts, e.Name())
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return res, fmt.Errorf("cannot adopt %s: %s already exist in %s (kept the archive copies; move or remove the local ones, then re-run)",
			local, strings.Join(conflicts, ", "), canonical)
	}

	if err := os.MkdirAll(canonical, 0o755); err != nil {
		return res, err
	}
	for _, e := range entries {
		if err := move(filepath.Join(local, e.Name()), filepath.Join(canonical, e.Name())); err != nil {
			return res, err
		}
		res.Adopted = append(res.Adopted, e.Name())
	}
	// local is now empty; replace it with the symlink.
	if err := os.Remove(local); err != nil {
		return res, err
	}
	if err := ensureLink(canonical, local); err != nil {
		return res, fmt.Errorf("adopted %d item(s) into %s but could not create the link at %s (your files are safe in the archive): %w",
			len(res.Adopted), canonical, local, err)
	}
	res.Linked = true
	return res, nil
}

// ensureLink creates the archive dir and a symlink local -> canonical (absolute).
func ensureLink(canonical, local string) error {
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		return err
	}
	return os.Symlink(canonical, local)
}

// resolveLink reads the symlink at path and returns its absolute, cleaned target
// (relative targets are resolved against dir).
func resolveLink(dir, path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	return filepath.Clean(target), nil
}

// move renames src to dst, falling back to copy-then-remove across devices. The
// fallback is all-or-nothing: if the copy fails partway it removes the partial
// dst, so the source is left intact and a later retry isn't blocked by the
// adopt conflict pre-check.
func move(src, dst string) error {
	err := renameFunc(src, dst)
	if err == nil {
		return nil
	}
	if !isCrossDevice(err) {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		_ = os.RemoveAll(dst) // roll back the partial copy
		return err
	}
	return os.RemoveAll(src)
}

// copyTree recursively copies src to dst, preserving symlinks as symlinks (it
// never dereferences them) and file permission bits.
func copyTree(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case fi.IsDir():
		if err := os.MkdirAll(dst, fi.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return copyFileFunc(src, dst, fi.Mode().Perm())
	}
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// isUnder reports whether path is root or nested within it.
func isUnder(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	sep := string(os.PathSeparator)
	if root == sep { // every absolute path is under the filesystem root
		return strings.HasPrefix(path, sep)
	}
	return strings.HasPrefix(path, root+sep)
}

// countFiles counts non-directory entries (files and symlinks) under dir, for
// the cosmetic Status.Files display.
func countFiles(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}
