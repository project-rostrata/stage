package git

import (
	"bytes"
	"fmt"
	"strings"
)

// Section is the Status pane heading an entry is listed under.
type Section int

// The sections, in the order the Status pane shows them.
const (
	Staged Section = iota
	Modified
	Untracked
	Unmerged
)

// Entry is one path under one section. A file with both staged and unstaged
// changes has an entry in Staged and another in Modified, as in git-cola.
type Entry struct {
	Section  Section
	Code     byte   // M, A, D, R, C or T from git's XY field; '?' untracked; 'U' unmerged
	Path     string // byte-for-byte as git printed it
	OrigPath string // a rename or copy's source, in Staged only
}

// Status is the working tree's state, grouped by section.
type Status struct {
	Branch    string // "" when HEAD is detached
	Staged    []Entry
	Modified  []Entry
	Untracked []Entry
	Unmerged  []Entry
}

// Status runs git status. Untracked files are listed one by one, as git-cola does.
func (r *Repo) Status() (Status, error) {
	out, err := r.read(nil, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out)
}

// parseStatus reads `git status --porcelain=v2 -z --branch` output.
func parseStatus(out []byte) (Status, error) {
	var st Status
	records := bytes.Split(out, []byte{0})
	for i := 0; i < len(records); i++ {
		rec := string(records[i])
		if rec == "" {
			continue
		}
		switch rec[0] {
		case '#':
			if head, ok := strings.CutPrefix(rec, "# branch.head "); ok && head != "(detached)" {
				st.Branch = head
			}
		case '1':
			// 1 XY sub mH mI mW hH hI path
			f, err := fields(rec, 9)
			if err != nil {
				return Status{}, err
			}
			st.add(f[1], f[8], "")
		case '2':
			// 2 XY sub mH mI mW hH hI Xscore path, then the original path as its own record
			f, err := fields(rec, 10)
			if err != nil {
				return Status{}, err
			}
			if i+1 >= len(records) {
				return Status{}, fmt.Errorf("git status: rename of %q has no original path", f[9])
			}
			i++
			st.add(f[1], f[9], string(records[i]))
		case 'u':
			// u XY sub m1 m2 m3 mW h1 h2 h3 path
			f, err := fields(rec, 11)
			if err != nil {
				return Status{}, err
			}
			st.Unmerged = append(st.Unmerged, Entry{Section: Unmerged, Code: 'U', Path: f[10]})
		case '?':
			st.Untracked = append(st.Untracked, Entry{Section: Untracked, Code: '?', Path: rec[2:]})
		}
	}
	return st, nil
}

// add files one changed path under Staged, Modified or both, from its XY code.
func (st *Status) add(xy, path, orig string) {
	if x := xy[0]; x != '.' {
		st.Staged = append(st.Staged, Entry{Section: Staged, Code: x, Path: path, OrigPath: orig})
	}
	if y := xy[1]; y != '.' {
		st.Modified = append(st.Modified, Entry{Section: Modified, Code: y, Path: path})
	}
}

// fields splits a status record into n space-separated fields. The last field
// is the path, which may itself contain spaces.
func fields(rec string, n int) ([]string, error) {
	f := strings.SplitN(rec, " ", n)
	if len(f) != n || len(f[1]) != 2 {
		return nil, fmt.Errorf("git status: unexpected record %q", rec)
	}
	return f, nil
}

// Entries returns every entry in the order the Status pane lists them.
func (st Status) Entries() []Entry {
	all := make([]Entry, 0, len(st.Staged)+len(st.Modified)+len(st.Untracked)+len(st.Unmerged))
	all = append(all, st.Staged...)
	all = append(all, st.Modified...)
	all = append(all, st.Untracked...)
	return append(all, st.Unmerged...)
}
