package ui

import (
	"strings"

	"stage/screen"
)

// drawBox draws a single-line border around r with the title set into the top edge.
func drawBox(g *screen.Grid, r rect, title string, focused bool, pal palette) {
	border, titleStyle := pal.unfocused, pal.title
	if focused {
		border, titleStyle = pal.focused, pal.focused
	}
	horizontal := strings.Repeat("─", r.w-2)
	g.Put(r.x, r.y, "┌"+horizontal+"┐", border, r.w)
	for y := r.y + 1; y < r.y+r.h-1; y++ {
		g.Put(r.x, y, "│", border, 1)
		g.Put(r.x+r.w-1, y, "│", border, 1)
	}
	g.Put(r.x, r.y+r.h-1, "└"+horizontal+"┘", border, r.w)
	g.Put(r.x+2, r.y, " "+title+" ", titleStyle, r.w-4)
}
