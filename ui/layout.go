package ui

// rect is a rectangle of cells.
type rect struct{ x, y, w, h int }

// inner is the rectangle inside a one-cell border.
func (r rect) inner() rect { return rect{r.x + 1, r.y + 1, r.w - 2, r.h - 2} }

const (
	minWidth  = 50
	minHeight = 12
	// commitHeight fits the border, the summary line, a rule, and three lines of description.
	commitHeight = 7
	minLeftWidth = 24
)

// layout is where each pane goes, as in the human's git-cola window: Commit
// above Status on the left, about a quarter of the width, and Diff on the
// right. The bottom row is the status bar.
type layout struct {
	commit, status, diff, bar rect
	tooSmall                  bool
}

func newLayout(w, h int) layout {
	if w < minWidth || h < minHeight {
		return layout{tooSmall: true, bar: rect{0, h - 1, w, 1}}
	}
	left := max(w/4, minLeftWidth)
	body := h - 1
	return layout{
		commit: rect{0, 0, left, commitHeight},
		status: rect{0, commitHeight, left, body - commitHeight},
		diff:   rect{left, 0, w - left, body},
		bar:    rect{0, body, w, 1},
	}
}
