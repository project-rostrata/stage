package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Commit is one commit as the history view lists it.
type Commit struct {
	Hash    string
	Parent  string // the first parent, or "" for a root commit
	Time    time.Time
	Subject string // the first line of the message
}

// Short is the abbreviated hash shown in the list.
func (c Commit) Short() string { return c.Hash[:min(len(c.Hash), 7)] }

// logFormat asks for one line per commit with its fields separated by NUL.
// %P lists every parent, separated by spaces; a subject never holds a newline.
const logFormat = "%H%x00%P%x00%ct%x00%s"

// Log lists up to n commits of the current branch, newest first. A branch
// with no commits yet has an empty history rather than an error.
//
// --no-show-signature: a repository's config can set log.showSignature and
// gpg.program, and git would run that program for every signed commit shown.
func (r *Repo) Log(n int) ([]Commit, error) {
	head, err := r.read([]int{1}, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return nil, err
	}
	if len(head) == 0 {
		return nil, nil
	}
	out, err := r.read(nil, "log", "--no-show-signature", "--no-color", "--format="+logFormat,
		"--max-count="+strconv.Itoa(n), "HEAD", "--")
	if err != nil {
		return nil, err
	}
	return parseLog(string(out))
}

func parseLog(out string) ([]Commit, error) {
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\x00", 4)
		if len(f) != 4 {
			return nil, fmt.Errorf("unexpected git log line %q", line)
		}
		parents := strings.Fields(f[1])
		if !isHash(f[0]) || (len(parents) > 0 && !isHash(parents[0])) {
			return nil, fmt.Errorf("unexpected commit hash in %q", line)
		}
		unix, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected commit time in %q", line)
		}
		c := Commit{Hash: f[0], Time: time.Unix(unix, 0), Subject: f[3]}
		if len(parents) > 0 {
			c.Parent = parents[0]
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// isHash reports whether s is a full object name. Hashes go on git's command
// line before "--", so one that could read as an option is refused.
func isHash(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// FileChange is one path a commit changed.
type FileChange struct {
	Code     byte   // A, M, D, R, C or T from git's name-status
	Path     string // byte-for-byte as git printed it
	OrigPath string // a rename or copy's source
}

// commitDiffCommand builds a git diff-tree command line comparing a commit
// with its first parent, or with nothing for a root commit. A merge shows
// what it brought into the branch. The options keep the output in the shape
// parseDiff reads, whatever the human's diff settings, as diffCommand does.
func commitDiffCommand(c Commit, opts []string, paths ...string) []string {
	args := append([]string{"diff-tree", "-r", "-M", "--no-commit-id", "--no-color", "--no-ext-diff",
		"--no-textconv", "--src-prefix=a/", "--dst-prefix=b/"}, opts...)
	if c.Parent == "" {
		args = append(args, "--root", c.Hash)
	} else {
		args = append(args, c.Parent, c.Hash)
	}
	return append(append(args, "--"), paths...)
}

// CommitFiles lists the paths c changed.
func (r *Repo) CommitFiles(c Commit) ([]FileChange, error) {
	out, err := r.read(nil, commitDiffCommand(c, []string{"--name-status", "-z"})...)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out)
}

// parseNameStatus reads `--name-status -z` output: a status, then a path, or
// for a rename or copy a status with a score and two paths.
func parseNameStatus(out []byte) ([]FileChange, error) {
	if len(out) == 0 {
		return nil, nil
	}
	var files []FileChange
	fields := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	for i := 0; i < len(fields); i++ {
		status := fields[i]
		if len(status) == 0 || i+1 >= len(fields) {
			return nil, fmt.Errorf("unexpected git diff-tree output near %q", status)
		}
		f := FileChange{Code: status[0]}
		if f.Code == 'R' || f.Code == 'C' {
			if i+2 >= len(fields) {
				return nil, fmt.Errorf("unexpected git diff-tree output near %q", status)
			}
			f.OrigPath = string(fields[i+1])
			i++
		}
		f.Path = string(fields[i+1])
		i++
		files = append(files, f)
	}
	return files, nil
}

// CommitDiff returns the diff of one path c changed.
func (r *Repo) CommitDiff(c Commit, f FileChange) (FileDiff, error) {
	paths := []string{f.Path}
	if f.OrigPath != "" {
		paths = []string{f.OrigPath, f.Path}
	}
	out, err := r.read(nil, commitDiffCommand(c, []string{"-p"}, paths...)...)
	if err != nil {
		return FileDiff{}, err
	}
	return parseDiff(string(out))
}
