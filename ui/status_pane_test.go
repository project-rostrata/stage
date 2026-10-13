package ui

import (
	"testing"

	"stage/git"
)

func TestSelectionAfterStatusChanges(t *testing.T) {
	before := git.Status{
		Staged:    []git.Entry{{Section: git.Staged, Code: 'M', Path: "a.go"}},
		Modified:  []git.Entry{{Section: git.Modified, Code: 'M', Path: "b.go"}},
		Untracked: []git.Entry{{Section: git.Untracked, Code: '?', Path: "c.txt"}, {Section: git.Untracked, Code: '?', Path: "d.txt"}},
	}
	tests := []struct {
		name  string
		pick  int // index into before's entries
		after git.Status
		want  string
	}{
		{
			name: "a path still in its section stays selected",
			pick: 3,
			after: git.Status{
				Staged:    []git.Entry{{Section: git.Staged, Code: 'A', Path: "c.txt"}, {Section: git.Staged, Code: 'M', Path: "a.go"}},
				Untracked: []git.Entry{{Section: git.Untracked, Code: '?', Path: "d.txt"}},
			},
			want: "d.txt",
		},
		{
			name: "after staging an untracked file, the next untracked file is selected",
			pick: 2,
			after: git.Status{
				Staged:    []git.Entry{{Section: git.Staged, Code: 'A', Path: "c.txt"}, {Section: git.Staged, Code: 'M', Path: "a.go"}},
				Modified:  []git.Entry{{Section: git.Modified, Code: 'M', Path: "b.go"}},
				Untracked: []git.Entry{{Section: git.Untracked, Code: '?', Path: "d.txt"}},
			},
			want: "d.txt",
		},
		{
			name: "after staging the last modified file, the selection stays at the same place",
			pick: 1,
			after: git.Status{
				Staged:    []git.Entry{{Section: git.Staged, Code: 'M', Path: "a.go"}, {Section: git.Staged, Code: 'M', Path: "b.go"}},
				Untracked: []git.Entry{{Section: git.Untracked, Code: '?', Path: "c.txt"}, {Section: git.Untracked, Code: '?', Path: "d.txt"}},
			},
			want: "b.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p statusPane
			p.setStatus(before)
			p.cursor = tt.pick
			p.setStatus(tt.after)
			if e, _ := p.selected(); e.Path != tt.want {
				t.Errorf("selected %q (%v), want %q", e.Path, e.Section, tt.want)
			}
		})
	}
}
