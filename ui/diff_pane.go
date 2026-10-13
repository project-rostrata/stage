package ui

import (
	"fmt"
	"strings"

	"stage/git"
	"stage/screen"
)

// diffLine is one logical line of the Diff pane, before wrapping.
type diffLine struct {
	kind     lineKind
	text     string // tabs already expanded; for a hunk line, includes its +, - or space
	old, new int    // line numbers, 0 for none
	hunk     int    // index of the hunk it belongs to, -1 for header lines
}

type lineKind int

const (
	kindInfo lineKind = iota // a mode, rename or binary line from the diff header
	kindHunk                 // an @@ line
	kindContext
	kindAdded
	kindRemoved
	kindNoEOL
)

// diffPane shows one path's diff, with a cursor line.
type diffPane struct {
	entry    git.Entry
	hasEntry bool
	lines    []diffLine
	numWidth int
	cursor   int // index into lines
	top      int // first visible line; it always starts at its first wrapped row
	message  string
}

// headerShown lists the diff header lines worth showing: the ones that say
// something the hunks don't (a new mode, a rename, a binary file).
var headerShown = []string{"new file mode", "deleted file mode", "old mode", "new mode",
	"rename from", "rename to", "copy from", "copy to", "Binary files"}

// setDiff shows d for e. Reloading the same entry keeps the cursor where it was.
func (p *diffPane) setDiff(e git.Entry, d git.FileDiff) {
	same := p.hasEntry && p.entry.Section == e.Section && p.entry.Path == e.Path
	p.entry, p.hasEntry, p.message = e, true, ""
	p.lines = p.lines[:0]
	maxNo := 0
	for _, h := range d.Header {
		for _, prefix := range headerShown {
			if strings.HasPrefix(h, prefix) {
				p.lines = append(p.lines, diffLine{kind: kindInfo, text: h, hunk: -1})
			}
		}
	}
	for i, h := range d.Hunks {
		p.lines = append(p.lines, diffLine{kind: kindHunk, text: expandTabs(h.Header), hunk: i})
		for _, l := range h.Lines {
			dl := diffLine{text: string(l.Kind) + expandTabs(l.Text), old: l.Old, new: l.New, hunk: i}
			switch l.Kind {
			case git.Added:
				dl.kind = kindAdded
			case git.Removed:
				dl.kind = kindRemoved
			case git.NoEOL:
				dl.kind = kindNoEOL
			default:
				dl.kind = kindContext
			}
			if d.Combined {
				dl.text = expandTabs(l.Text)
			}
			maxNo = max(maxNo, dl.old, dl.new)
			p.lines = append(p.lines, dl)
		}
	}
	p.numWidth = max(len(fmt.Sprint(maxNo)), 2)
	if len(p.lines) == 0 {
		p.message = "No changes to show"
	}
	if !same {
		p.cursor, p.top = 0, 0
	}
	p.cursor = min(p.cursor, max(len(p.lines)-1, 0))
}

// clear shows a message instead of a diff.
func (p *diffPane) clear(message string) {
	p.hasEntry, p.lines, p.cursor, p.top, p.message = false, p.lines[:0], 0, 0, message
}

func (p *diffPane) title() string {
	if !p.hasEntry {
		return "Diff"
	}
	if p.entry.Section == git.Staged {
		return "Diff · " + p.entry.Path + " (staged)"
	}
	return "Diff · " + p.entry.Path
}

func (p *diffPane) move(delta int) {
	if len(p.lines) == 0 {
		return
	}
	p.cursor = min(max(p.cursor+delta, 0), len(p.lines)-1)
}

// moveHunk puts the cursor on the next (dir 1) or previous (dir -1) hunk header.
func (p *diffPane) moveHunk(dir int) {
	for i := p.cursor + dir; i >= 0 && i < len(p.lines); i += dir {
		if p.lines[i].kind == kindHunk {
			p.cursor = i
			return
		}
	}
}

// gutterWidth is the cursor marker, the old and new line numbers, and a space after each.
func (p *diffPane) gutterWidth() int { return 1 + p.numWidth + 1 + p.numWidth + 1 }

func (p *diffPane) draw(g *screen.Grid, r rect, focused bool, pal palette) {
	in := r.inner()
	if p.message != "" {
		g.Put(in.x+1, in.y, p.message, pal.hint, in.w-1)
		return
	}
	gutter := p.gutterWidth()
	textWidth := in.w - gutter
	if textWidth < 4 {
		return
	}
	p.scrollToCursor(in.h, textWidth)

	y := in.y
	for i := p.top; i < len(p.lines) && y < in.y+in.h; i++ {
		l := p.lines[i]
		style := pal.lineStyle(l.kind)
		for n, piece := range wrap(l.text, textWidth) {
			if y >= in.y+in.h {
				break
			}
			if style.Bg != screen.Default {
				g.Fill(in.x+gutter, y, textWidth, 1, style)
			}
			if n == 0 {
				p.drawGutter(g, in.x, y, l, i == p.cursor && focused, pal)
			}
			g.Put(in.x+gutter, y, piece, style, textWidth)
			y++
		}
	}
}

func (p *diffPane) drawGutter(g *screen.Grid, x, y int, l diffLine, isCursor bool, pal palette) {
	if isCursor {
		g.Put(x, y, "▶", pal.focused, 1)
	}
	num := func(n int) string {
		if n == 0 {
			return strings.Repeat(" ", p.numWidth)
		}
		return fmt.Sprintf("%*d", p.numWidth, n)
	}
	if l.kind == kindHunk || l.kind == kindInfo || l.kind == kindNoEOL {
		return
	}
	g.Put(x+1, y, num(l.old)+" "+num(l.new), pal.unfocused, p.numWidth*2+1)
}

// scrollToCursor moves top so the whole cursor line is on screen.
func (p *diffPane) scrollToCursor(height, textWidth int) {
	if p.cursor < p.top {
		p.top = p.cursor
	}
	for p.top < p.cursor && p.rowsBetween(p.top, p.cursor, textWidth) > height {
		p.top++
	}
}

// rowsBetween counts the screen rows lines from..to (inclusive) take once wrapped.
func (p *diffPane) rowsBetween(from, to, textWidth int) int {
	n := 0
	for i := from; i <= to; i++ {
		n += len(wrap(p.lines[i].text, textWidth))
	}
	return n
}
