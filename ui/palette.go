// Package ui draws stage's three panes and handles keys. It knows nothing about termios.
package ui

import "stage/screen"

// palette holds every colour stage draws with (brief §4). The values are a
// starting point for a dark GNOME Terminal profile: visibility comes first.
type palette struct {
	focused    screen.Style // focused pane's border and title (the eel green's dark-surface token)
	unfocused  screen.Style // other panes' borders, and line numbers
	title      screen.Style // other panes' titles
	added      screen.Style
	removed    screen.Style
	addedSel   screen.Style
	removedSel screen.Style
	hunk       screen.Style
	cursorLine screen.Style
	statusBar  screen.Style
	hint       screen.Style
}

func darkPalette() palette {
	grey := screen.Hex(0x6c6c6c)
	return palette{
		focused:    screen.Style{Fg: screen.Hex(0x93b558), Bold: true},
		unfocused:  screen.Style{Fg: grey},
		title:      screen.Style{Fg: screen.Hex(0xb0b0b0)},
		added:      screen.Style{Bg: screen.Hex(0x1e3a1e)},
		removed:    screen.Style{Bg: screen.Hex(0x4a1e1e)},
		addedSel:   screen.Style{Bg: screen.Hex(0x2f6b2f)},
		removedSel: screen.Style{Bg: screen.Hex(0x7a2c2c)},
		hunk:       screen.Style{Fg: screen.Hex(0x7fb4ca)},
		cursorLine: screen.Style{Bg: screen.Hex(0x3a3a3a)},
		statusBar:  screen.Style{Fg: screen.Hex(0xb0b0b0)},
		hint:       screen.Style{Fg: grey},
	}
}
