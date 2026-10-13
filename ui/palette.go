// Package ui draws stage's three panes and handles keys. It knows nothing about termios.
package ui

import (
	"stage/git"
	"stage/screen"
)

// palette holds every colour stage draws with (brief §4). The values are a
// starting point for a dark GNOME Terminal profile: visibility comes first.
type palette struct {
	focused    screen.Style // focused pane's border and title, and the cursor marker (the eel green's dark-surface token)
	unfocused  screen.Style // other panes' borders, and line numbers
	title      screen.Style // other panes' titles
	heading    screen.Style // Status section headings
	code       [4]screen.Style
	added      screen.Style
	removed    screen.Style
	addedSel   screen.Style
	removedSel screen.Style
	hunk       screen.Style
	info       screen.Style // mode, rename and binary lines; "\ No newline at end of file"
	cursorLine screen.Style
	statusBar  screen.Style
	branch     screen.Style
	behind     screen.Style // how far a branch is behind its upstream
	errorText  screen.Style
	hint       screen.Style
}

func darkPalette() palette {
	grey := screen.Hex(0x6c6c6c)
	light := screen.Hex(0xb0b0b0)
	green := screen.Hex(0x93b558)
	var code [4]screen.Style
	code[git.Staged] = screen.Style{Fg: green, Bold: true}
	amber := screen.Hex(0xe0b060)
	code[git.Modified] = screen.Style{Fg: amber, Bold: true}
	code[git.Untracked] = screen.Style{Fg: light, Bold: true}
	code[git.Unmerged] = screen.Style{Fg: screen.Hex(0xf07070), Bold: true}
	return palette{
		focused:    screen.Style{Fg: green, Bold: true},
		unfocused:  screen.Style{Fg: grey},
		title:      screen.Style{Fg: light},
		heading:    screen.Style{Fg: light, Bold: true},
		code:       code,
		added:      screen.Style{Bg: screen.Hex(0x1e3a1e)},
		removed:    screen.Style{Bg: screen.Hex(0x4a1e1e)},
		addedSel:   screen.Style{Bg: screen.Hex(0x2f6b2f)},
		removedSel: screen.Style{Bg: screen.Hex(0x7a2c2c)},
		hunk:       screen.Style{Fg: screen.Hex(0x7fb4ca)},
		info:       screen.Style{Fg: screen.Hex(0x8a8a8a)},
		cursorLine: screen.Style{Bg: screen.Hex(0x3a3a3a)},
		statusBar:  screen.Style{Fg: light},
		branch:     screen.Style{Fg: green, Bold: true},
		behind:     screen.Style{Fg: amber, Bold: true},
		errorText:  screen.Style{Fg: screen.Hex(0xf07070), Bold: true},
		hint:       screen.Style{Fg: grey},
	}
}

// lineStyle is how a diff line of the given kind is drawn, and whether it's selected.
func (pal palette) lineStyle(k lineKind, selected bool) screen.Style {
	switch {
	case k == kindAdded && selected:
		return pal.addedSel
	case k == kindRemoved && selected:
		return pal.removedSel
	}
	switch k {
	case kindAdded:
		return pal.added
	case kindRemoved:
		return pal.removed
	case kindHunk:
		return pal.hunk
	case kindInfo, kindNoEOL:
		return pal.info
	}
	return screen.Style{}
}
