package ui

import (
	"stage/git"
	"stage/screen"
)

var sectionTitles = [...]string{
	git.Staged:    "Staged",
	git.Modified:  "Modified",
	git.Untracked: "Untracked",
	git.Unmerged:  "Unmerged",
}

// statusPane lists changed paths under their section headings, with one selected.
type statusPane struct {
	entries []git.Entry
	cursor  int // index into entries
	top     int // first visible row, counting headings
}

// setStatus replaces the list. The same path stays selected if it is still
// listed under the same section. If it has moved (just staged, say), the
// selection stays at the same place in its old section, which is the next file
// there, as in git-cola. If that section is now empty, it stays at the same
// place in the whole list.
func (p *statusPane) setStatus(st git.Status) {
	old, had := p.selected()
	pos := p.positionInSection()
	p.entries = st.Entries()
	if had {
		var inSection []int
		for i, e := range p.entries {
			if e.Section == old.Section && e.Path == old.Path {
				p.cursor = i
				return
			}
			if e.Section == old.Section {
				inSection = append(inSection, i)
			}
		}
		if len(inSection) > 0 {
			p.cursor = inSection[min(pos, len(inSection)-1)]
			return
		}
	}
	p.cursor = min(p.cursor, max(len(p.entries)-1, 0))
}

// positionInSection is the selected entry's index within its own section.
func (p *statusPane) positionInSection() int {
	e, ok := p.selected()
	if !ok {
		return 0
	}
	n := 0
	for i := 0; i < p.cursor; i++ {
		if p.entries[i].Section == e.Section {
			n++
		}
	}
	return n
}

func (p *statusPane) selected() (git.Entry, bool) {
	if len(p.entries) == 0 {
		return git.Entry{}, false
	}
	return p.entries[p.cursor], true
}

// move moves the selection by delta entries, stopping at either end.
func (p *statusPane) move(delta int) {
	if len(p.entries) == 0 {
		return
	}
	p.cursor = min(max(p.cursor+delta, 0), len(p.entries)-1)
}

// row is one line of the pane: a heading, or an entry.
type row struct {
	heading string
	entry   int // index into entries, or -1 for a heading
}

func (p *statusPane) rows() []row {
	var rows []row
	section := git.Section(-1)
	for i, e := range p.entries {
		if e.Section != section {
			section = e.Section
			rows = append(rows, row{heading: sectionTitles[section], entry: -1})
		}
		rows = append(rows, row{entry: i})
	}
	return rows
}

func (p *statusPane) draw(g *screen.Grid, r rect, focused bool, pal palette) {
	in := r.inner()
	if len(p.entries) == 0 {
		g.Put(in.x+1, in.y, "No changes", pal.hint, in.w-1)
		return
	}
	rows := p.rows()
	cursorRow := 0
	for i, rw := range rows {
		if rw.entry == p.cursor {
			cursorRow = i
		}
	}
	// Keep the cursor visible, and its section heading too when it's the first entry.
	if cursorRow-1 < p.top {
		p.top = max(cursorRow-1, 0)
	}
	if cursorRow >= p.top+in.h {
		p.top = cursorRow - in.h + 1
	}
	p.top = max(min(p.top, len(rows)-in.h), 0)

	for y := 0; y < in.h && p.top+y < len(rows); y++ {
		rw := rows[p.top+y]
		if rw.entry < 0 {
			g.Put(in.x+1, in.y+y, rw.heading, pal.heading, in.w-1)
			continue
		}
		e := p.entries[rw.entry]
		label := e.Path
		if e.OrigPath != "" {
			label = e.OrigPath + " → " + e.Path
		}
		textStyle := screen.Style{}
		if rw.entry == p.cursor {
			textStyle = pal.cursorLine
			textStyle.Bold = focused
			g.Fill(in.x, in.y+y, in.w, 1, pal.cursorLine)
		}
		codeStyle := pal.code[e.Section]
		codeStyle.Bg = textStyle.Bg
		g.Put(in.x+1, in.y+y, string(e.Code), codeStyle, 1)
		g.Put(in.x+3, in.y+y, shortenLeft(label, in.w-4), textStyle, in.w-4)
	}
}
