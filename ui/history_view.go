package ui

import (
	"time"

	"stage/git"
	"stage/screen"
)

// historyPage is how many more commits are read each time the cursor reaches
// the last one read, so a long history isn't read all at once.
const historyPage = 200

// historyView is the current branch's history in the three panes' places:
// the commits where Commit is, the files the selected commit changed where
// Status is, and the selected file's diff on the right. It only reads.
type historyView struct {
	commits []git.Commit
	limit   int // how many commits to ask git for
	commit  list
	files   []git.FileChange
	file    list
	diff    diffPane
	shownOf string // the hash whose files are listed
	focus   pane   // paneCommit is the commit list, paneStatus the files
	loc     *time.Location
}

func newHistoryView(loc *time.Location) historyView {
	return historyView{limit: historyPage, diff: diffPane{mark: -1}, focus: paneCommit, loc: loc}
}

// setCommits replaces the commits, keeping the cursor on the same commit if
// it's still listed.
func (v *historyView) setCommits(commits []git.Commit) {
	old, had := v.selectedCommit()
	v.commits = commits
	v.commit.cursor = 0
	if had {
		for i, c := range commits {
			if c.Hash == old.Hash {
				v.commit.cursor = i
			}
		}
	}
}

// setFiles replaces the files. Reloading the same commit keeps the cursor
// on the same path; another commit starts at the top.
func (v *historyView) setFiles(hash string, files []git.FileChange) {
	old, had := v.selectedFile()
	same := hash == v.shownOf
	v.files, v.shownOf = files, hash
	v.file.cursor, v.file.top = 0, 0
	if same && had {
		for i, f := range files {
			if f.Path == old.Path {
				v.file.cursor = i
			}
		}
	}
	if !same {
		// Another commit's diff of the same path starts at its top too.
		v.diff.clear("")
	}
}

func (v *historyView) selectedCommit() (git.Commit, bool) {
	if v.commit.cursor >= len(v.commits) {
		return git.Commit{}, false
	}
	return v.commits[v.commit.cursor], true
}

func (v *historyView) selectedFile() (git.FileChange, bool) {
	if v.file.cursor >= len(v.files) {
		return git.FileChange{}, false
	}
	return v.files[v.file.cursor], true
}

// atEnd reports whether the cursor is on the last commit read while git may have more.
func (v *historyView) atEnd() bool {
	return len(v.commits) == v.limit && v.commit.cursor == len(v.commits)-1
}

// filesTitle is the selected commit's local time. Its hash is on its row,
// and the column is too narrow for both.
func (v *historyView) filesTitle() string {
	c, ok := v.selectedCommit()
	if !ok {
		return "Files"
	}
	return c.Time.In(v.loc).Format(timeLayout)
}

// drawCommits shows one commit per row: its short hash, then its summary.
func (v *historyView) drawCommits(g *screen.Grid, r rect, focused bool, pal palette) {
	in := r.inner()
	if len(v.commits) == 0 {
		g.Put(in.x+1, in.y, "No commits yet", pal.hint, in.w-1)
		return
	}
	v.commit.scroll(len(v.commits), in.h)
	for y := 0; y < in.h && v.commit.top+y < len(v.commits); y++ {
		i := v.commit.top + y
		c := v.commits[i]
		text := v.commit.rowStyle(g, in, y, i, focused, pal)
		hash := pal.hint
		hash.Bg = text.Bg
		g.Put(in.x+1, in.y+y, c.Short(), hash, min(7, in.w-1))
		g.Put(in.x+9, in.y+y, shortenRight(c.Subject, in.w-9), text, max(in.w-9, 0))
	}
}

// drawFiles shows one changed path per row, with its status letter.
func (v *historyView) drawFiles(g *screen.Grid, r rect, focused bool, pal palette) {
	in := r.inner()
	if len(v.files) == 0 {
		if len(v.commits) > 0 {
			g.Put(in.x+1, in.y, "No files changed", pal.hint, in.w-1)
		}
		return
	}
	v.file.scroll(len(v.files), in.h)
	for y := 0; y < in.h && v.file.top+y < len(v.files); y++ {
		i := v.file.top + y
		f := v.files[i]
		text := v.file.rowStyle(g, in, y, i, focused, pal)
		label := f.Path
		if f.OrigPath != "" {
			label = f.OrigPath + " → " + f.Path
		}
		// A committed change is coloured as a staged one is.
		code := pal.code[git.Staged]
		code.Bg = text.Bg
		g.Put(in.x+1, in.y+y, string(f.Code), code, 1)
		g.Put(in.x+3, in.y+y, shortenLeft(label, in.w-4), text, in.w-4)
	}
}

// list is a cursor and the first visible row of a one-row-per-item list.
type list struct{ cursor, top int }

func (l *list) move(delta, n int) {
	l.cursor = min(max(l.cursor+delta, 0), max(n-1, 0))
}

// scroll keeps the cursor within the height rows shown.
func (l *list) scroll(n, height int) {
	if l.cursor < l.top {
		l.top = l.cursor
	}
	if l.cursor >= l.top+height {
		l.top = l.cursor - height + 1
	}
	l.top = max(min(l.top, n-height), 0)
}

// rowStyle fills the cursor's row and returns the style for its text, which
// is bold while the list is focused, as in Status.
func (l *list) rowStyle(g *screen.Grid, in rect, y, i int, focused bool, pal palette) screen.Style {
	if i != l.cursor {
		return screen.Style{}
	}
	g.Fill(in.x, in.y+y, in.w, 1, pal.cursorLine)
	st := pal.cursorLine
	st.Bold = focused
	return st
}
