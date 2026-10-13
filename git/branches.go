package git

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

// Branch is a local branch as the branch list shows it.
type Branch struct {
	Name    string // without refs/heads/, so feature/x stays whole
	Current bool
	Behind  int  // commits on its upstream it doesn't have, as of the last fetch
	Gone    bool // it has an upstream, but the upstream no longer exists
	Time    time.Time
	Subject string // the last commit's summary
}

// branchFormat asks for one line per branch with its fields separated by NUL.
// A subject never holds a newline: git joins the first paragraph's lines.
const branchFormat = "%(HEAD)%00%(refname:lstrip=2)%00%(upstream:track,nobracket)%00%(committerdate:unix)%00%(contents:subject)"

// Branches lists the local branches, the most recently committed to first.
// It doesn't fetch, so how far behind each one is dates from the last fetch.
func (r *Repo) Branches() ([]Branch, error) {
	out, err := r.read(nil, "for-each-ref", "--sort=-committerdate", "--format="+branchFormat, "refs/heads/")
	if err != nil {
		return nil, err
	}
	return parseBranches(string(out))
}

func parseBranches(out string) ([]Branch, error) {
	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\x00", 5)
		if len(f) != 5 {
			return nil, fmt.Errorf("unexpected git for-each-ref line %q", line)
		}
		unix, err := strconv.ParseInt(f[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected commit time in %q", line)
		}
		b := Branch{Name: f[1], Current: f[0] == "*", Time: time.Unix(unix, 0), Subject: f[4]}
		// The track is "", "gone", or "ahead N", "behind N" or both, joined by ", ".
		for _, part := range strings.Split(f[2], ", ") {
			switch {
			case part == "gone":
				b.Gone = true
			case strings.HasPrefix(part, "behind "):
				if b.Behind, err = strconv.Atoi(strings.TrimPrefix(part, "behind ")); err != nil {
					return nil, fmt.Errorf("unexpected upstream track in %q", line)
				}
			}
		}
		branches = append(branches, b)
	}
	return branches, nil
}

// Switch checks out a local branch with git switch, which only takes branches
// and refuses to overwrite uncommitted work. git won't create a branch named
// like an option, but a .git from somewhere else can hold one, so the name
// follows "--" like a path.
func (r *Repo) Switch(name string) error {
	return r.write(nil, "switch", "--", name)
}

// LastFetch returns when the repository last fetched, from FETCH_HEAD's
// modification time, or the zero time if it never has.
func (r *Repo) LastFetch() (time.Time, error) {
	p, err := r.gitPath("FETCH_HEAD")
	if err != nil {
		return time.Time{}, err
	}
	fi, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}
