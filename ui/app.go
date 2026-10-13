package ui

import (
	"fmt"

	"stage/input"
	"stage/screen"
	"stage/term"
)

// pane identifies one of the three panes.
type pane int

const (
	paneCommit pane = iota
	paneStatus
	paneDiff
)

// App is stage's state and its single-threaded event loop.
type App struct {
	term    *term.Terminal
	scr     *screen.Screen
	pal     palette
	lay     layout
	focus   pane
	lastKey string
	quit    bool
}

// New returns the app, focused on Status as git-cola starts.
func New(t *term.Terminal, s *screen.Screen) *App {
	return &App{term: t, scr: s, pal: darkPalette(), focus: paneStatus}
}

// Run draws and handles keys until the human quits or a signal asks stage to exit.
func (a *App) Run() error {
	if err := a.resize(); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	var pending []byte
	dirty := true
	for !a.quit {
		if a.term.QuitRequested() {
			return nil
		}
		if a.term.Resized() {
			if err := a.resize(); err != nil {
				return err
			}
			dirty = true
		}
		if dirty {
			a.draw(a.scr.Frame())
			if err := a.scr.Flush(screen.Cursor{}); err != nil {
				return err
			}
			dirty = false
		}

		n, err := a.term.Read(buf)
		if err != nil {
			return fmt.Errorf("read terminal: %w", err)
		}
		var keys []input.Key
		switch {
		case n > 0:
			keys, pending = input.Decode(append(pending, buf[:n]...), false)
		case len(pending) > 0:
			// Nothing more arrived in time, so a dangling ESC is the Esc key.
			keys, pending = input.Decode(pending, true)
		}
		for _, k := range keys {
			a.handle(k)
			dirty = true
		}
	}
	return nil
}

func (a *App) resize() error {
	w, h, err := a.term.Size()
	if err != nil {
		return err
	}
	a.scr.Resize(w, h)
	a.lay = newLayout(w, h)
	return nil
}

func (a *App) handle(k input.Key) {
	if k.Code != input.FocusIn && k.Code != input.FocusOut {
		a.lastKey = k.String()
	}
	switch {
	case k.Is('q', input.Ctrl):
		a.quit = true
	case k.Is('l', input.Ctrl):
		a.focus = paneCommit
	case k.Is('k', input.Ctrl):
		a.focus = paneStatus
	case k.Is('j', input.Ctrl):
		a.focus = paneDiff
	case k.IsCode(input.Tab, 0):
		a.focus = (a.focus + 1) % 3
	case k.IsCode(input.BackTab, 0):
		a.focus = (a.focus + 2) % 3
	case k.IsCode(input.Esc, 0) && a.focus == paneCommit:
		a.focus = paneStatus
	}
}

func (a *App) draw(g *screen.Grid) {
	l := a.lay
	if l.tooSmall {
		w, _ := g.Size()
		g.Put(0, l.bar.y, fmt.Sprintf("stage needs at least %dx%d", minWidth, minHeight), a.pal.statusBar, w)
		return
	}
	drawBox(g, l.commit, "Commit", a.focus == paneCommit, a.pal)
	drawBox(g, l.status, "Status", a.focus == paneStatus, a.pal)
	drawBox(g, l.diff, "Diff", a.focus == paneDiff, a.pal)
	a.drawBar(g, l.bar)
}

// drawBar draws the status bar: the last key pressed on the left (so the keys
// GNOME Terminal sends can be checked), and how to quit on the right.
func (a *App) drawBar(g *screen.Grid, r rect) {
	hint := "Ctrl+Q quit"
	if a.lastKey != "" {
		g.Put(r.x+1, r.y, "key: "+a.lastKey, a.pal.statusBar, r.w-len(hint)-3)
	}
	g.Put(r.x+r.w-len(hint)-1, r.y, hint, a.pal.hint, len(hint))
}
