package git

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// draftName is the file in the git directory that keeps an unsent commit
// message, the way git-cola keeps GIT_COLA_MSG.
const draftName = "STAGE_MSG"

// draftPath asks git where the draft lives, once. --git-path gets this right
// for linked worktrees and a relocated git directory.
func (r *Repo) draftPath() (string, error) {
	if r.draft != "" {
		return r.draft, nil
	}
	out, err := r.read(nil, "rev-parse", "--git-path", draftName)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(string(out), "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	r.draft = p
	return p, nil
}

// LoadDraft returns the saved commit message, or "" if there is none.
func (r *Repo) LoadDraft() (string, error) {
	p, err := r.draftPath()
	if err != nil {
		return "", err
	}
	f, err := openFile(p, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read draft: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read draft: %w", err)
	}
	return string(b), nil
}

// SaveDraft keeps the message being written. An empty message removes the draft.
func (r *Repo) SaveDraft(msg string) error {
	p, err := r.draftPath()
	if err != nil {
		return err
	}
	if msg == "" {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove draft: %w", err)
		}
		return nil
	}
	f, err := openFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	if _, err := f.WriteString(msg); err != nil {
		f.Close()
		return fmt.Errorf("save draft: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	return nil
}

// CommitDraft runs git commit with the saved draft as the message. It is given
// the terminal: hooks, GPG or SSH signing and pinentry may need to prompt.
// Committing from a file rather than stdin leaves stdin free for them.
func (r *Repo) CommitDraft(stdin io.Reader, stdout, stderr io.Writer) error {
	p, err := r.draftPath()
	if err != nil {
		return err
	}
	// git follows links, so the draft it reads must be the file stage wrote.
	if fi, err := os.Lstat(p); err != nil {
		return fmt.Errorf("read draft: %w", err)
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a plain file; stage won't commit from it", p)
	}
	// git runs at the top level, so a draft inside it is named relative to
	// it: the same file, and a shorter line in the audit trail.
	if rel, err := filepath.Rel(r.root, p); err == nil && !strings.HasPrefix(rel, "..") {
		p = rel
	}
	args := guarded([]string{"commit", "--file", p})
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	err = cmd.Run()
	if logErr := r.record(args, err); logErr != nil && err == nil {
		return fmt.Errorf("committed, but %w", logErr)
	}
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}
