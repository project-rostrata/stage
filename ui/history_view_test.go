package ui

import (
	"reflect"
	"testing"
	"time"

	"stage/git"
	"stage/screen"
)

func TestHistoryKeepsTheCursorOnItsCommit(t *testing.T) {
	v := newHistoryView(time.UTC)
	v.setCommits([]git.Commit{{Hash: "b"}, {Hash: "a"}})
	v.commit.move(1, len(v.commits))
	// A new commit arrives on top.
	v.setCommits([]git.Commit{{Hash: "c"}, {Hash: "b"}, {Hash: "a"}})
	if c, _ := v.selectedCommit(); c.Hash != "a" {
		t.Errorf("selected %q after a new commit, want a", c.Hash)
	}
	// Once it's gone (an amend, a reset), the cursor goes back to the top.
	v.setCommits([]git.Commit{{Hash: "d"}})
	if c, _ := v.selectedCommit(); c.Hash != "d" {
		t.Errorf("selected %q after a was dropped, want d", c.Hash)
	}
}

func TestHistoryFilesKeepTheirCursorOnlyWithinACommit(t *testing.T) {
	v := newHistoryView(time.UTC)
	files := []git.FileChange{{Code: 'M', Path: "a"}, {Code: 'A', Path: "b"}}
	v.setFiles("c1", files)
	v.file.move(1, len(v.files))
	v.setFiles("c1", files) // a reload
	if f, _ := v.selectedFile(); f.Path != "b" {
		t.Errorf("selected %q after reloading the same commit, want b", f.Path)
	}
	v.setFiles("c2", files)
	if f, _ := v.selectedFile(); f.Path != "a" {
		t.Errorf("selected %q in another commit, want a", f.Path)
	}
	v.setFiles("c3", nil)
	if _, ok := v.selectedFile(); ok {
		t.Error("a commit with no files has a selected file")
	}
}

func TestHistoryReadsFurtherAtTheEnd(t *testing.T) {
	v := newHistoryView(time.UTC)
	v.setCommits(make([]git.Commit, historyPage))
	if v.atEnd() {
		t.Error("at the end on the first commit")
	}
	v.commit.move(toEnd, len(v.commits))
	if !v.atEnd() {
		t.Error("not at the end on the last commit read")
	}
	// Fewer than asked for means there are no more.
	v.setCommits(make([]git.Commit, 3))
	v.commit.move(toEnd, len(v.commits))
	if v.atEnd() {
		t.Error("at the end of a short history, which git has no more of")
	}
}

func TestHistoryRows(t *testing.T) {
	zone := time.FixedZone("EDT", -4*60*60)
	at := time.Date(2026, 10, 13, 16, 38, 0, 0, zone)
	v := newHistoryView(zone)
	v.setCommits([]git.Commit{
		{Hash: "42a07840000000000000000000000000000000000", Time: at.UTC(), Subject: "Move to the branch list after a commit"},
		{Hash: "6c741b10000000000000000000000000000000000", Time: at.Add(-time.Hour), Subject: "Add a branch list"},
	})
	v.setFiles("42a0784", []git.FileChange{{Code: 'M', Path: "ui/app.go"}, {Code: 'R', Path: "docs/new.md", OrigPath: "docs/old.md"}})

	g := screen.NewGrid(30, 4)
	v.drawCommits(g, rect{0, 0, 30, 4}, true, darkPalette())
	want := []string{" 42a0784 Move to the branch…", " 6c741b1 Add a branch list"}
	if got := innerRows(g); !reflect.DeepEqual(got, want) {
		t.Errorf("commits:\n got %q\nwant %q", got, want)
	}
	g = screen.NewGrid(30, 4)
	v.drawFiles(g, rect{0, 0, 30, 4}, false, darkPalette())
	want = []string{" M ui/app.go", " R …cs/old.md → docs/new.md"}
	if got := innerRows(g); !reflect.DeepEqual(got, want) {
		t.Errorf("files:\n got %q\nwant %q", got, want)
	}
	if got := v.filesTitle(); got != "2026-10-13 16:38" {
		t.Errorf("files title %q", got)
	}
}
