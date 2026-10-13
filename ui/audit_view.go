package ui

import (
	"path/filepath"
	"strings"

	"stage/screen"
)

// auditLines is how much of .git/stage.log the view loads.
const auditLines = 500

// auditView shows the end of the audit trail in place of the Diff pane: the
// git commands stage ran that changed the repository, newest at the bottom.
type auditView struct {
	open  bool
	lines []string
	path  string
	top   int // first entry shown
	atEnd bool
}

// load reads the trail and, the first time or if the view was at the
// bottom, scrolls to the newest entry.
func (v *auditView) load(lines []string, path string) {
	v.lines, v.path = lines, path
	if v.atEnd || !v.open {
		v.top = len(lines)
		v.atEnd = true
	}
	v.top = min(v.top, len(lines))
}

func (v *auditView) scroll(delta int) {
	v.top = min(max(v.top+delta, 0), max(len(v.lines)-1, 0))
	v.atEnd = false
}

func (v *auditView) title(root string) string {
	if rel, err := filepath.Rel(root, v.path); err == nil && !strings.HasPrefix(rel, "..") {
		return "Commands run · " + rel
	}
	return "Commands run · " + v.path
}

func (v *auditView) draw(g *screen.Grid, r rect, pal palette) {
	in := r.inner()
	x, w := in.x+1, in.w-2
	if len(v.lines) == 0 {
		g.Put(x, in.y, "stage hasn't changed anything in this repository yet", pal.hint, w)
		return
	}
	// Lay out from the bottom when scrolled to the end, so the newest entry
	// is on the last row; otherwise from top.
	type row struct {
		text  string
		entry int
		first bool
	}
	var rows []row
	start := v.top
	if v.atEnd {
		start = len(v.lines)
		for used := 0; start > 0; start-- {
			used += len(wrap(v.lines[start-1], w))
			if used > in.h {
				break
			}
		}
	}
	for i := start; i < len(v.lines) && len(rows) < in.h; i++ {
		for n, piece := range wrap(v.lines[i], w) {
			rows = append(rows, row{piece, i, n == 0})
		}
	}
	if v.atEnd {
		v.top = start
	}
	for y := 0; y < in.h && y < len(rows); y++ {
		rw := rows[y]
		line := v.lines[rw.entry]
		style := screen.Style{}
		if strings.Contains(line, "  failed  ") {
			style = pal.errorText
		}
		if rw.first && len(rw.text) >= 19 {
			// The timestamp is quieter than the command.
			g.Put(x, in.y+y, rw.text[:19], pal.hint, w)
			g.Put(x+19, in.y+y, rw.text[19:], style, w-19)
			continue
		}
		g.Put(x, in.y+y, rw.text, style, w)
	}
}
