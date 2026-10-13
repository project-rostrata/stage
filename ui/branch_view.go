package ui

import (
	"fmt"
	"time"

	"stage/git"
	"stage/screen"
)

// timeLayout is how the branch list shows a time, in the local time zone.
const timeLayout = "2006-01-02 15:04"

// branchView lists the local branches, the most recently committed to first,
// over the whole body in place of the three panes.
type branchView struct {
	branches []git.Branch
	fetched  time.Time // the last fetch, which "behind" dates from; zero if never
	loc      *time.Location
	cursor   int
	top      int
}

// load replaces the list, keeping the cursor on the same branch if it's still there.
func (v *branchView) load(branches []git.Branch, fetched time.Time) {
	old, had := v.selected()
	v.branches, v.fetched = branches, fetched
	v.cursor = 0
	if had {
		for i, b := range branches {
			if b.Name == old.Name {
				v.cursor = i
			}
		}
	}
}

func (v *branchView) selected() (git.Branch, bool) {
	if v.cursor < 0 || v.cursor >= len(v.branches) {
		return git.Branch{}, false
	}
	return v.branches[v.cursor], true
}

func (v *branchView) move(delta int) {
	v.cursor = min(max(v.cursor+delta, 0), max(len(v.branches)-1, 0))
}

func (v *branchView) title() string {
	if v.fetched.IsZero() {
		return "Branches · never fetched"
	}
	return "Branches · fetched " + v.fetched.In(v.loc).Format(timeLayout)
}

// behind is a branch's behind column: ↓ and a count, "gone" if its upstream
// was deleted, or nothing.
func behind(b git.Branch) string {
	switch {
	case b.Gone:
		return "gone"
	case b.Behind > 0:
		return fmt.Sprintf("↓%d", b.Behind)
	}
	return ""
}

// draw shows one branch per row: * for the current one, the name, how far
// behind its upstream it is, its last commit's local time and summary.
func (v *branchView) draw(g *screen.Grid, r rect, pal palette) {
	in := r.inner()
	x, w := in.x+1, in.w-2
	if len(v.branches) == 0 {
		g.Put(x, in.y, "No branches yet", pal.hint, w)
		return
	}
	nameW, behindW := 0, 0
	for _, b := range v.branches {
		nameW = max(nameW, screen.StringWidth(b.Name))
		behindW = max(behindW, screen.StringWidth(behind(b)))
	}
	nameW = min(nameW, max(w/3, 8))
	if behindW > 0 {
		behindW += 2
	}

	if v.cursor < v.top {
		v.top = v.cursor
	}
	if v.cursor >= v.top+in.h {
		v.top = v.cursor - in.h + 1
	}
	v.top = max(min(v.top, len(v.branches)-in.h), 0)

	for y := 0; y < in.h && v.top+y < len(v.branches); y++ {
		i := v.top + y
		b := v.branches[i]
		bg := screen.Style{}
		if i == v.cursor {
			bg = pal.cursorLine
			g.Fill(in.x, in.y+y, in.w, 1, bg)
		}
		style := func(st screen.Style) screen.Style {
			st.Bg = bg.Bg
			return st
		}
		col, end := x, x+w
		put := func(s string, st screen.Style, width int) {
			g.Put(col, in.y+y, s, style(st), max(min(width, end-col), 0))
			col += width
		}
		name := screen.Style{}
		if b.Current {
			put("*", pal.branch, 2)
			name = pal.branch
		} else {
			put("", screen.Style{}, 2)
		}
		put(shortenRight(b.Name, nameW), name, nameW+2)
		if b.Gone {
			put(behind(b), pal.hint, behindW)
		} else {
			put(behind(b), pal.behind, behindW)
		}
		put(b.Time.In(v.loc).Format(timeLayout), pal.hint, len(timeLayout)+2)
		put(shortenRight(b.Subject, end-col), screen.Style{}, end-col)
	}
}
