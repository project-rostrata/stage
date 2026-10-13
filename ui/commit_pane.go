package ui

import (
	"strings"

	"stage/screen"
)

// draw draws the message into the Commit pane, as git-cola lays it out: the
// summary on the first line, a rule, and the description below, wrapped. It
// returns where the terminal cursor goes when the pane is focused.
func (e *editor) draw(g *screen.Grid, r rect, focused bool, pal palette) (int, int) {
	in := r.inner()
	x, w := in.x+1, in.w-1
	if w < 4 || in.h < 3 {
		return x, in.y
	}

	// The summary scrolls sideways to keep the cursor in view.
	summary := []rune(e.lines[0])
	start := 0
	if e.row == 0 {
		for width(summary[start:e.col]) > w-1 {
			start++
		}
	}
	if len(summary) == 0 {
		g.Put(x, in.y, "Commit summary", pal.hint, w)
	} else {
		g.Put(x, in.y, string(summary[start:]), screen.Style{}, w)
	}
	g.Put(in.x, in.y+1, strings.Repeat("─", in.w), pal.unfocused, in.w)

	// The description wraps; top keeps the cursor's row in view.
	height := in.h - 2
	type row struct{ line, from, to int }
	var rows []row
	cursorRow, cursorCol := -1, 0
	for li := 1; li < len(e.lines); li++ {
		l := []rune(e.lines[li])
		from := 0
		for {
			to := from
			for to < len(l) && width(l[from:to+1]) <= w-1 {
				to++
			}
			if li == e.row && e.col >= from && (e.col < to || to == len(l)) {
				cursorRow, cursorCol = len(rows), width(l[from:e.col])
			}
			rows = append(rows, row{li, from, to})
			if to >= len(l) {
				break
			}
			from = to
		}
	}
	if e.row > 0 {
		e.top = min(e.top, cursorRow)
		e.top = max(e.top, cursorRow-height+1)
	}
	e.top = max(min(e.top, len(rows)-height), 0)

	if len(e.lines) == 2 && e.lines[1] == "" && !(focused && e.row > 0) {
		g.Put(x, in.y+2, "Extended description...", pal.hint, w)
	}
	for i := 0; i < height && e.top+i < len(rows); i++ {
		rw := rows[e.top+i]
		g.Put(x, in.y+2+i, string([]rune(e.lines[rw.line])[rw.from:rw.to]), screen.Style{}, w)
	}

	if e.row == 0 {
		return x + width(summary[start:e.col]), in.y
	}
	return x + cursorCol, in.y + 2 + cursorRow - e.top
}

func width(r []rune) int {
	n := 0
	for _, c := range r {
		n += screen.DrawnWidth(c)
	}
	return n
}
