// Package git runs the git command and parses what it prints. It is the only
// package that starts processes.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Repo is a git working tree. Every command runs at its top level.
type Repo struct {
	root  string
	paths map[string]string // files in the git directory, by name, once looked up
}

// Open finds the working tree that contains dir.
func Open(dir string) (*Repo, error) {
	cmd := exec.Command("git", guarded([]string{"rev-parse", "--show-toplevel"})...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	return &Repo{root: strings.TrimRight(string(out), "\n"), paths: map[string]string{}}, nil
}

// gitPath asks git where a file in the git directory lives, once per name.
// --git-path gets this right for linked worktrees and a relocated git directory.
func (r *Repo) gitPath(name string) (string, error) {
	if p, ok := r.paths[name]; ok {
		return p, nil
	}
	out, err := r.read(nil, "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(string(out), "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	r.paths[name] = p
	return p, nil
}

// Root returns the working tree's top-level directory.
func (r *Repo) Root() string { return r.root }

// read runs a git command that only reads. GIT_OPTIONAL_LOCKS=0 stops it taking
// the index lock just to refresh stat information, so it never gets in the way
// of the human's own git commands in another terminal. okCodes lists exit codes
// other than 0 that mean success (git diff --no-index exits 1 when the files differ).
func (r *Repo) read(okCodes []int, args ...string) ([]byte, error) {
	return r.run(nil, okCodes, []string{"GIT_OPTIONAL_LOCKS=0"}, args...)
}

// write runs a git command that changes the index or the working tree, with
// stdin as its input (a patch), or none, and records it in the audit trail.
// If the command worked but the trail couldn't be written, that is reported.
func (r *Repo) write(stdin []byte, args ...string) error {
	_, err := r.run(stdin, nil, nil, args...)
	if logErr := r.record(guarded(args), err); logErr != nil && err == nil {
		return fmt.Errorf("done, but %w", logErr)
	}
	return err
}

// guarded puts the options every command runs with in front of it. They are
// flags rather than environment variables so the audit trail shows them.
//
//   - core.fsmonitor=false: a repository's own config can name a program for
//     git to run on every status, diff and add. stage looks at whatever
//     repository it is started in, often, so it never runs one. The setting is
//     only a cache; turning it off changes no result.
//   - --literal-pathspecs: every path means exactly that path, never a
//     pattern. Without it, deleting an untracked file named "a*" with git
//     clean also deletes ab.txt.
func guarded(args []string) []string {
	return append([]string{"-c", "core.fsmonitor=false", "--literal-pathspecs"}, args...)
}

// run runs git at the top level, guarded.
func (r *Repo) run(stdin []byte, okCodes []int, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", guarded(args)...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		for _, code := range okCodes {
			if exit.ExitCode() == code {
				return out, nil
			}
		}
	}
	if err != nil {
		return nil, commandError(args, stderr.String(), err)
	}
	return out, nil
}

// commandError reports a failed git command with git's own message when it gave one.
func commandError(args []string, stderr string, err error) error {
	sub := "git"
	if len(args) > 0 {
		sub = "git " + args[0]
	}
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s: %s", sub, msg)
	}
	return fmt.Errorf("%s: %w", sub, err)
}

// openFile opens one of the files stage keeps in the git directory, never
// through a symbolic link: in a .git that came from someone else, the draft
// could link to a key that would be loaded into the message and committed, and
// the audit trail to ~/.bashrc.
func openFile(p string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(p, flag|syscall.O_NOFOLLOW, perm)
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%s is a symbolic link; stage won't follow it", p)
	}
	return f, err
}
