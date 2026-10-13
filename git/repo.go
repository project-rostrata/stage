// Package git runs the git command and parses what it prints. It is the only
// package that starts processes.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repo is a git working tree. Every command runs at its top level.
type Repo struct {
	root  string
	draft string // the draft message file's path, once looked up
}

// Open finds the working tree that contains dir.
func Open(dir string) (*Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
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
	return &Repo{root: strings.TrimRight(string(out), "\n")}, nil
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
// stdin as its input (a patch, a commit message), or none.
func (r *Repo) write(stdin []byte, args ...string) error {
	_, err := r.run(stdin, nil, nil, args...)
	return err
}

// run runs git at the top level. GIT_LITERAL_PATHSPECS=1 makes every path
// argument mean exactly that path, never a pattern: without it, deleting an
// untracked file named "a*" with git clean also deletes ab.txt.
func (r *Repo) run(stdin []byte, okCodes []int, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Env = append(append(os.Environ(), "GIT_LITERAL_PATHSPECS=1"), env...)
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
