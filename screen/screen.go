package screen

import (
	"bytes"
	"io"
	"strconv"
)

const (
	clearScreen = "\x1b[2J"
	cursorHide  = "\x1b[?25l"
	cursorShow  = "\x1b[?25h"
	sgrReset    = "\x1b[0m"
)

// Cursor is where the terminal's cursor goes after a frame, if it is shown at all.
type Cursor struct {
	X, Y    int
	Visible bool
}

// Screen double-buffers frames: Frame returns a blank grid to draw into, and
// Flush writes only the cells that differ from the previous frame.
type Screen struct {
	out          io.Writer
	front, back  *Grid
	clearPending bool
	buf          bytes.Buffer
}

// New returns a screen that writes to out. Call Resize before the first frame.
func New(out io.Writer) *Screen {
	return &Screen{out: out, front: NewGrid(0, 0), back: NewGrid(0, 0)}
}

// Resize sets the screen's size. The next Flush clears the terminal and redraws everything.
func (s *Screen) Resize(w, h int) {
	s.front = NewGrid(w, h)
	s.back = NewGrid(w, h)
	s.clearPending = true
}

// Invalidate makes the next Flush redraw everything, for when something else has
// written to the terminal (after a suspend, for example).
func (s *Screen) Invalidate() {
	w, h := s.front.Size()
	s.Resize(w, h)
}

// Frame returns the blank grid to draw the next frame into.
func (s *Screen) Frame() *Grid {
	s.back.Clear()
	return s.back
}

// Flush writes the differences between the frame just drawn and the previous
// one, in a single write.
func (s *Screen) Flush(cur Cursor) error {
	s.buf.Reset()
	s.buf.WriteString(cursorHide)
	if s.clearPending {
		// After a clear, every cell is a blank in the default style, which is
		// exactly what a fresh front grid holds, so the diff below is correct.
		s.buf.WriteString(sgrReset + clearScreen)
		s.clearPending = false
	}
	w, h := s.back.Size()
	atX, atY := -1, -1
	styled := false
	var current Style
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := s.back.Cell(x, y)
			if c == s.front.Cell(x, y) || c.continuation() {
				continue
			}
			if x != atX || y != atY {
				s.moveTo(x, y)
			}
			if !styled || c.Style != current {
				s.writeStyle(c.Style)
				current, styled = c.Style, true
			}
			s.buf.WriteString(c.Text)
			atX, atY = x+1, y
			if x+1 < w && s.back.Cell(x+1, y).continuation() {
				atX++
			}
		}
	}
	s.buf.WriteString(sgrReset)
	if cur.Visible {
		s.moveTo(cur.X, cur.Y)
		s.buf.WriteString(cursorShow)
	}
	s.front, s.back = s.back, s.front
	_, err := s.out.Write(s.buf.Bytes())
	return err
}

func (s *Screen) moveTo(x, y int) {
	s.buf.WriteString("\x1b[")
	s.buf.WriteString(strconv.Itoa(y + 1))
	s.buf.WriteByte(';')
	s.buf.WriteString(strconv.Itoa(x + 1))
	s.buf.WriteByte('H')
}

// writeStyle sets the whole style from scratch, so no attribute leaks from the last cell.
func (s *Screen) writeStyle(st Style) {
	s.buf.WriteString("\x1b[0")
	if st.Bold {
		s.buf.WriteString(";1")
	}
	if st.Dim {
		s.buf.WriteString(";2")
	}
	if st.Reverse {
		s.buf.WriteString(";7")
	}
	if st.Fg != Default {
		s.writeColor(";38;2;", st.Fg)
	}
	if st.Bg != Default {
		s.writeColor(";48;2;", st.Bg)
	}
	s.buf.WriteByte('m')
}

func (s *Screen) writeColor(prefix string, c Color) {
	r, g, b := c.rgb()
	s.buf.WriteString(prefix)
	s.buf.WriteString(strconv.Itoa(int(r)))
	s.buf.WriteByte(';')
	s.buf.WriteString(strconv.Itoa(int(g)))
	s.buf.WriteByte(';')
	s.buf.WriteString(strconv.Itoa(int(b)))
}
