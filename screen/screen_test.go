package screen

import (
	"bytes"
	"strings"
	"testing"
)

func TestWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'é', 1}, {'─', 1}, {'▌', 1},
		{'中', 2}, {'한', 2}, {'Ａ', 2}, {'😀', 2},
		{'́', 0}, {'‍', 0}, {'️', 0},
		{'\t', 0}, {0x7f, 0},
	}
	for _, tt := range tests {
		if got := Width(tt.r); got != tt.want {
			t.Errorf("Width(%U) = %d, want %d", tt.r, got, tt.want)
		}
	}
}

func row(g *Grid, y int) string {
	w, _ := g.Size()
	var b strings.Builder
	for x := 0; x < w; x++ {
		b.WriteString(g.Cell(x, y).Text)
	}
	return b.String()
}

func TestPut(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxWidth int
		want     string
		used     int
	}{
		{"plain", "abc", 10, "abc   ", 3},
		{"clipped", "abcdefgh", 4, "abcd  ", 4},
		{"wide characters take two cells", "中文", 10, "中文  ", 4},
		{"half a wide character becomes a space", "a中", 2, "a     ", 2},
		{"combining mark joins its base", "éx", 10, "éx    ", 2},
		{"control characters are made visible", "a\x1bb", 10, "a�b   ", 3},
		{"so are C1 controls, which can start an escape sequence", "a\u009bb", 10, "a�b   ", 3},
		{"clipped at the grid's edge", "abcdefgh", 100, "abcdef", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrid(6, 1)
			used := g.Put(0, 0, tt.s, Style{}, tt.maxWidth)
			if got := row(g, 0); got != tt.want || used != tt.used {
				t.Errorf("Put(%q) = %q (%d cols), want %q (%d cols)", tt.s, got, used, tt.want, tt.used)
			}
		})
	}
}

func TestPutOverWideCharacter(t *testing.T) {
	g := NewGrid(6, 1)
	g.Put(0, 0, "中文", Style{}, 6)
	g.Put(1, 0, "x", Style{}, 1) // breaks 中 on its right half
	if got := row(g, 0); got != " x文  " {
		t.Errorf("overwriting a right half: %q", got)
	}
	g.Put(2, 0, "y", Style{}, 1) // breaks 文 on its left half
	if got := row(g, 0); got != " xy   " {
		t.Errorf("overwriting a left half: %q", got)
	}
}

func flush(t *testing.T, s *Screen, out *bytes.Buffer) string {
	t.Helper()
	out.Reset()
	if err := s.Flush(Cursor{}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestFlushWritesOnlyChangedCells(t *testing.T) {
	var out bytes.Buffer
	s := New(&out)
	s.Resize(10, 3)

	s.Frame().Put(0, 0, "hello", Style{}, 10)
	first := flush(t, s, &out)
	if !strings.Contains(first, clearScreen) || !strings.Contains(first, "hello") {
		t.Fatalf("first frame should clear and draw everything: %q", first)
	}

	g := s.Frame()
	g.Put(0, 0, "hello", Style{}, 10)
	g.Put(2, 2, "!", Style{Bold: true}, 10)
	second := flush(t, s, &out)
	want := cursorHide + "\x1b[3;3H" + "\x1b[0;1m" + "!" + sgrReset
	if second != want {
		t.Errorf("second frame wrote %q, want %q", second, want)
	}

	g = s.Frame()
	g.Put(0, 0, "hello", Style{}, 10)
	g.Put(2, 2, "!", Style{Bold: true}, 10)
	if third := flush(t, s, &out); third != cursorHide+sgrReset {
		t.Errorf("an unchanged frame wrote %q", third)
	}
}

func TestFlushColoursAndWideCharacters(t *testing.T) {
	var out bytes.Buffer
	s := New(&out)
	s.Resize(10, 1)
	flush(t, s, &out)

	g := s.Frame()
	g.Put(0, 0, "中x", Style{Fg: Hex(0x93b558), Bg: Hex(0x1e3a1e)}, 10)
	got := flush(t, s, &out)
	want := cursorHide + "\x1b[1;1H" + "\x1b[0;38;2;147;181;88;48;2;30;58;30m" + "中x" + sgrReset
	if got != want {
		t.Errorf("wrote %q, want %q", got, want)
	}
}

func TestInvalidateRedrawsEverything(t *testing.T) {
	var out bytes.Buffer
	s := New(&out)
	s.Resize(5, 1)
	s.Frame().Put(0, 0, "ab", Style{}, 5)
	flush(t, s, &out)

	s.Invalidate()
	s.Frame().Put(0, 0, "ab", Style{}, 5)
	got := flush(t, s, &out)
	if !strings.Contains(got, clearScreen) || !strings.Contains(got, "ab") {
		t.Errorf("after Invalidate, wrote %q", got)
	}
}
