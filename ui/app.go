package ui

import (
	"fmt"
	"strings"

	"stage/git"
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
	term   *term.Terminal
	scr    *screen.Screen
	repo   *git.Repo
	pal    palette
	lay    layout
	focus  pane
	branch string
	counts string
	status statusPane
	diff   diffPane
	errMsg string // the last error, shown in the status bar until the next action
	// pendingG is set after a lone g, waiting for the second g of gg.
	pendingG bool
	quit     bool
}

// New returns the app, focused on Status as git-cola starts.
func New(t *term.Terminal, s *screen.Screen, r *git.Repo) *App {
	return &App{term: t, scr: s, repo: r, pal: darkPalette(), focus: paneStatus}
}

// Run draws and handles keys until the human quits or a signal asks stage to exit.
func (a *App) Run() error {
	if err := a.resize(); err != nil {
		return err
	}
	a.reload()
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

// reload re-reads the status and the selected path's diff, keeping the
// selection and the diff's cursor where they can stay.
func (a *App) reload() {
	st, err := a.repo.Status()
	if err != nil {
		a.errMsg = err.Error()
		return
	}
	a.branch = st.Branch
	a.counts = counts(st)
	a.status.setStatus(st)
	a.loadDiff()
}

// loadDiff shows the diff of the path selected in Status.
func (a *App) loadDiff() {
	e, ok := a.status.selected()
	if !ok {
		a.diff.clear("No changes")
		return
	}
	d, err := a.repo.Diff(e)
	if err != nil {
		a.diff.clear("")
		a.errMsg = err.Error()
		return
	}
	a.diff.setDiff(e, d)
}

func counts(st git.Status) string {
	var parts []string
	for _, c := range []struct {
		n    int
		name string
	}{{len(st.Staged), "staged"}, {len(st.Modified), "modified"}, {len(st.Untracked), "untracked"}, {len(st.Unmerged), "unmerged"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.name))
		}
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " · ")
}

func (a *App) handle(k input.Key) {
	if k.Code != input.FocusOut {
		a.errMsg = ""
	}
	wasG := a.pendingG
	a.pendingG = false
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
	case k.Is('r', input.Ctrl), k.IsCode(input.F5, 0), k.Code == input.FocusIn:
		a.reload()
	case a.focus == paneCommit:
		if k.IsCode(input.Esc, 0) {
			a.focus = paneStatus
		}
	case k.Is('g', 0) && !wasG:
		a.pendingG = true
	default:
		if delta, ok := a.movement(k, wasG); ok {
			a.moveFocused(delta)
			return
		}
		if a.focus == paneDiff && k.Is('n', 0) {
			a.diff.moveHunk(1)
		}
		if a.focus == paneDiff && k.Is('p', 0) {
			a.diff.moveHunk(-1)
		}
	}
}

// Far enough to reach either end of any list.
const toEnd = 1 << 30

// movement maps the movement keys (brief §4) to a distance in lines.
func (a *App) movement(k input.Key, afterG bool) (int, bool) {
	page := a.pageHeight()
	switch {
	case k.Is('j', 0), k.IsCode(input.Down, 0), k.Is('n', input.Ctrl):
		return 1, true
	case k.Is('k', 0), k.IsCode(input.Up, 0), k.Is('p', input.Ctrl):
		return -1, true
	case k.Is('d', 0):
		return max(page/2, 1), true
	case k.Is('u', 0):
		return -max(page/2, 1), true
	case k.Is('v', input.Ctrl), k.IsCode(input.PgDn, 0):
		return page, true
	case k.Is('v', input.Alt), k.IsCode(input.PgUp, 0):
		return -page, true
	case k.Is('G', 0), k.Is('>', input.Alt), k.IsCode(input.End, 0):
		return toEnd, true
	case k.Is('g', 0) && afterG, k.Is('<', input.Alt), k.IsCode(input.Home, 0):
		return -toEnd, true
	}
	return 0, false
}

func (a *App) pageHeight() int {
	if a.focus == paneDiff {
		return max(a.lay.diff.h-3, 1)
	}
	return max(a.lay.status.h-3, 1)
}

func (a *App) moveFocused(delta int) {
	switch a.focus {
	case paneStatus:
		before := a.status.cursor
		a.status.move(delta)
		if a.status.cursor != before {
			a.loadDiff()
		}
	case paneDiff:
		a.diff.move(delta)
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
	a.status.draw(g, l.status, a.focus == paneStatus, a.pal)
	drawBox(g, l.diff, a.diff.title(), a.focus == paneDiff, a.pal)
	a.diff.draw(g, l.diff, a.focus == paneDiff, a.pal)
	a.drawBar(g, l.bar)
}

// drawBar draws the status bar: the branch and counts, or the last error, on
// the left, and the main keys on the right.
func (a *App) drawBar(g *screen.Grid, r rect) {
	hint := "Tab pane · Ctrl+R refresh · Ctrl+Q quit"
	room := r.w - len([]rune(hint)) - 3
	if a.errMsg != "" {
		msg := strings.ReplaceAll(a.errMsg, "\n", " ")
		g.Put(r.x+1, r.y, msg, a.pal.errorText, r.w-2)
		return
	}
	x := r.x + 1
	if a.branch != "" {
		x += g.Put(x, r.y, a.branch, a.pal.branch, room)
		x += g.Put(x, r.y, "  ", a.pal.statusBar, 2)
	}
	g.Put(x, r.y, a.counts, a.pal.statusBar, max(room-(x-r.x), 0))
	g.Put(r.x+r.w-len([]rune(hint))-1, r.y, hint, a.pal.hint, len([]rune(hint)))
}
