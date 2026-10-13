// Package screen draws frames into a grid of cells and writes only the cells
// that changed since the last frame to the terminal. It knows nothing about git.
package screen

// Color is a 24-bit colour, or Default for the terminal's own colour.
type Color uint32

// Default is the terminal's own foreground or background colour.
const Default Color = 0

const colorSet Color = 1 << 24

// Hex returns the colour 0xRRGGBB.
func Hex(rgb uint32) Color { return colorSet | Color(rgb&0xffffff) }

func (c Color) rgb() (r, g, b uint8) { return uint8(c >> 16), uint8(c >> 8), uint8(c) }

// Style is how a cell is drawn.
type Style struct {
	Fg, Bg  Color
	Bold    bool
	Dim     bool
	Reverse bool
}

// Cell is one terminal cell. Text is the character drawn there, plus any
// zero-width characters that combine with it. A wide character's second cell
// has empty Text and is never written on its own.
type Cell struct {
	Text  string
	Style Style
}

func (c Cell) continuation() bool { return c.Text == "" }

const replacement = "�"

// Grid is one frame: Width x Height cells.
type Grid struct {
	w, h  int
	cells []Cell
}

// NewGrid returns a blank grid.
func NewGrid(w, h int) *Grid {
	g := &Grid{w: w, h: h, cells: make([]Cell, w*h)}
	g.Clear()
	return g
}

// Size returns the grid's width and height.
func (g *Grid) Size() (int, int) { return g.w, g.h }

// Clear blanks every cell.
func (g *Grid) Clear() {
	for i := range g.cells {
		g.cells[i] = Cell{Text: " "}
	}
}

// Cell returns the cell at x, y.
func (g *Grid) Cell(x, y int) Cell { return g.cells[y*g.w+x] }

// Fill paints the rectangle with spaces in the given style.
func (g *Grid) Fill(x, y, w, h int, st Style) {
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			g.set(col, row, " ", st)
		}
	}
}

// Put draws s at x, y in the given style, clipped to maxWidth columns and to the
// grid's edge, and returns the columns used. Control characters are drawn as
// U+FFFD. Callers expand tabs first.
func (g *Grid) Put(x, y int, s string, st Style, maxWidth int) int {
	if y < 0 || y >= g.h || x < 0 {
		return 0
	}
	limit := min(x+maxWidth, g.w)
	col := x
	for _, r := range s {
		w := DrawnWidth(r)
		text := string(r)
		if isControl(r) {
			text = replacement
		}
		if w == 0 {
			if col > x {
				prev := col - 1
				if g.Cell(prev, y).continuation() && prev > x {
					prev--
				}
				g.cells[y*g.w+prev].Text += text
			}
			continue
		}
		if col+w > limit {
			if col < limit {
				g.set(col, y, " ", st) // half of a wide character doesn't fit
				col++
			}
			break
		}
		g.set(col, y, text, st)
		if w == 2 {
			g.set(col+1, y, "", st)
		}
		col += w
	}
	return col - x
}

// set writes one cell, blanking the other half of any wide character it breaks.
func (g *Grid) set(x, y int, text string, st Style) {
	if x < 0 || x >= g.w || y < 0 || y >= g.h {
		return
	}
	i := y*g.w + x
	old := g.cells[i]
	if old.continuation() && text != "" && x > 0 {
		g.cells[i-1] = Cell{Text: " ", Style: g.cells[i-1].Style}
	}
	if !old.continuation() && x+1 < g.w && g.cells[i+1].continuation() {
		g.cells[i+1] = Cell{Text: " ", Style: g.cells[i+1].Style}
	}
	g.cells[i] = Cell{Text: text, Style: st}
}
