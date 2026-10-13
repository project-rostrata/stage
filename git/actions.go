package git

import "errors"

// Stage adds a path's working-tree state to the index: its changes, its
// deletion, or the whole of an untracked file.
func (r *Repo) Stage(e Entry) error {
	switch e.Section {
	case Modified, Untracked:
		return r.write(nil, "add", "--", e.Path)
	case Unmerged:
		return errors.New("resolve the conflict outside stage")
	}
	return errors.New("already staged")
}

// Unstage takes a staged path's changes back out of the index, leaving the
// working tree alone. A rename is unstaged as a whole: both paths go back.
func (r *Repo) Unstage(e Entry) error {
	if e.Section != Staged {
		return errors.New("not staged")
	}
	paths := []string{e.Path}
	if e.OrigPath != "" {
		paths = []string{e.OrigPath, e.Path}
	}
	if !r.hasCommits() {
		// Before the first commit there is no HEAD to restore from.
		return r.write(nil, append([]string{"rm", "--cached", "--quiet", "--"}, paths...)...)
	}
	return r.write(nil, append([]string{"restore", "--staged", "--"}, paths...)...)
}

// RevertFile throws away a path's unstaged changes, putting back the staged
// (index) version. Staged changes are kept.
func (r *Repo) RevertFile(e Entry) error {
	if e.Section != Modified {
		return errors.New("no unstaged changes to revert")
	}
	return r.write(nil, "restore", "--worktree", "--", e.Path)
}

// DeleteUntracked removes an untracked file. git clean refuses to touch
// anything tracked, so a wrong entry can't delete committed work.
func (r *Repo) DeleteUntracked(e Entry) error {
	if e.Section != Untracked {
		return errors.New("only untracked files can be deleted")
	}
	return r.write(nil, "clean", "--force", "--quiet", "--", e.Path)
}

func (r *Repo) hasCommits() bool {
	_, err := r.read(nil, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

// ApplyToIndex applies a patch to the index only, as a stage (reverse false)
// or an unstage (reverse true). --recount lets git fix the hunks' line counts,
// and --whitespace=nowarn stops an apply.whitespace setting from rewriting or
// refusing the human's own lines.
func (r *Repo) ApplyToIndex(patch string, reverse bool) error {
	args := []string{"apply", "--cached", "--recount", "--whitespace=nowarn"}
	if reverse {
		args = append(args, "-R")
	}
	return r.write([]byte(patch), append(args, "-")...)
}

// RevertPatch un-applies a patch built from the unstaged diff, throwing those
// changes away in the working tree. The index is left alone.
func (r *Repo) RevertPatch(patch string) error {
	return r.write([]byte(patch), "apply", "-R", "--recount", "--whitespace=nowarn", "-")
}
