package ui

import (
	"strings"
	"unicode"

	"stage/input"
)

// editor is the commit message: a one-line summary and a multi-line
// description, edited with Emacs keys. lines[0] is the summary; lines[1:] is
// the description, which always has at least one (possibly empty) line.
type editor struct {
	lines  []string
	row    int    // the line the cursor is on
	col    int    // the cursor's position in that line, in runes
	killed string // the last text killed, for Ctrl+Y
	top    int    // the first description row shown, after wrapping
}

// newEditor loads a message: its first line is the summary, and the rest,
// after the blank line git puts between them, is the description.
func newEditor(msg string) editor {
	msg = strings.TrimRight(msg, "\n")
	summary, rest, _ := strings.Cut(msg, "\n")
	rest = strings.TrimPrefix(rest, "\n")
	// The cursor starts at the end of the summary, to carry on writing.
	return editor{lines: append([]string{summary}, strings.Split(rest, "\n")...), col: len([]rune(summary))}
}

// message joins the summary and description the way git expects: a blank
// line between them and a newline at the end. It is "" when both are empty.
func (e *editor) message() string {
	summary := e.lines[0]
	desc := strings.TrimRight(strings.Join(e.lines[1:], "\n"), "\n \t")
	switch {
	case summary == "" && desc == "":
		return ""
	case desc == "":
		return summary + "\n"
	}
	return summary + "\n\n" + desc + "\n"
}

func (e *editor) summary() string { return e.lines[0] }

func (e *editor) line() []rune { return []rune(e.lines[e.row]) }

func (e *editor) setLine(r []rune) { e.lines[e.row] = string(r) }

// handle applies one key, reporting whether it was an editing key and
// whether the message changed.
func (e *editor) handle(k input.Key) (handled, changed bool) {
	switch {
	case k.Code == input.Paste:
		e.insert(k.Text)
		return true, true
	case k.Code == input.Rune && k.Mod == 0:
		e.insert(string(k.Rune))
		return true, true
	case k.IsCode(input.Enter, 0):
		e.newline()
		return true, true
	case k.IsCode(input.Backspace, 0):
		return true, e.deleteBack()
	case k.IsCode(input.Delete, 0), k.Is('d', input.Ctrl):
		return true, e.deleteForward()
	case k.Is('a', input.Ctrl), k.IsCode(input.Home, 0):
		e.col = 0
	case k.Is('e', input.Ctrl), k.IsCode(input.End, 0):
		e.col = len(e.line())
	case k.Is('f', input.Ctrl), k.IsCode(input.Right, 0):
		e.forward()
	case k.Is('b', input.Ctrl), k.IsCode(input.Left, 0):
		e.back()
	case k.Is('n', input.Ctrl), k.IsCode(input.Down, 0):
		e.vertical(1)
	case k.Is('p', input.Ctrl), k.IsCode(input.Up, 0):
		e.vertical(-1)
	case k.Is('f', input.Alt), k.IsCode(input.Right, input.Ctrl):
		e.col = e.wordEnd()
	case k.Is('b', input.Alt), k.IsCode(input.Left, input.Ctrl):
		e.col = e.wordStart()
	case k.Is('d', input.Alt):
		return true, e.kill(e.col, e.wordEnd())
	case k.IsCode(input.Backspace, input.Alt), k.IsCode(input.Backspace, input.Ctrl), k.Is('w', input.Ctrl):
		start := e.wordStart()
		changed := e.kill(start, e.col)
		e.col = start
		return true, changed
	case k.Is('k', input.Ctrl):
		return true, e.killLine()
	case k.Is('y', input.Ctrl):
		if e.killed == "" {
			return true, false
		}
		e.insert(e.killed)
		return true, true
	default:
		return false, false
	}
	return true, false
}

// insert types text at the cursor. Line breaks in pasted text start new
// description lines; in the summary, everything after the first line break
// goes to the start of the description.
func (e *editor) insert(text string) {
	parts := strings.Split(text, "\n")
	if e.row == 0 && len(parts) > 1 {
		e.insert(parts[0])
		e.row, e.col = 1, 0
		// The blank line git puts after the summary is already implied.
		e.insert(strings.TrimPrefix(strings.Join(parts[1:], "\n"), "\n"))
		return
	}
	l := e.line()
	before, after := string(l[:e.col]), string(l[e.col:])
	if len(parts) == 1 {
		e.setLine([]rune(before + text + after))
		e.col += len([]rune(text))
		return
	}
	newLines := make([]string, 0, len(parts))
	newLines = append(newLines, before+parts[0])
	newLines = append(newLines, parts[1:len(parts)-1]...)
	last := parts[len(parts)-1]
	newLines = append(newLines, last+after)
	e.lines = append(e.lines[:e.row], append(newLines, e.lines[e.row+1:]...)...)
	e.row += len(parts) - 1
	e.col = len([]rune(last))
}

// newline splits the description line at the cursor. In the summary, Enter
// moves to the description, as in git-cola.
func (e *editor) newline() {
	if e.row == 0 {
		e.row, e.col = 1, 0
		return
	}
	e.insert("\n")
}

func (e *editor) deleteBack() bool {
	switch {
	case e.col > 0:
		l := e.line()
		e.setLine(append(l[:e.col-1:e.col-1], l[e.col:]...))
		e.col--
		return true
	case e.row > 1:
		e.row--
		e.col = len(e.line())
		e.joinNext()
		return true
	}
	return false // the summary and the description are never joined
}

func (e *editor) deleteForward() bool {
	l := e.line()
	switch {
	case e.col < len(l):
		e.setLine(append(l[:e.col:e.col], l[e.col+1:]...))
		return true
	case e.row >= 1 && e.row < len(e.lines)-1:
		e.joinNext()
		return true
	}
	return false
}

func (e *editor) joinNext() {
	e.lines[e.row] += e.lines[e.row+1]
	e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
}

// killLine kills to the end of the line, or at the end of a description line,
// the line break, as Emacs does.
func (e *editor) killLine() bool {
	l := e.line()
	if e.col < len(l) {
		return e.kill(e.col, len(l))
	}
	if e.row >= 1 && e.row < len(e.lines)-1 {
		e.joinNext()
		e.killed = "\n"
		return true
	}
	return false
}

// kill removes the runes from..to of the current line and keeps them for Ctrl+Y.
func (e *editor) kill(from, to int) bool {
	if from >= to {
		return false
	}
	l := e.line()
	e.killed = string(l[from:to])
	e.setLine(append(l[:from:from], l[to:]...))
	return true
}

func (e *editor) forward() {
	switch {
	case e.col < len(e.line()):
		e.col++
	case e.row < len(e.lines)-1:
		e.row, e.col = e.row+1, 0
	}
}

func (e *editor) back() {
	switch {
	case e.col > 0:
		e.col--
	case e.row > 0:
		e.row--
		e.col = len(e.line())
	}
}

func (e *editor) vertical(delta int) {
	e.row = min(max(e.row+delta, 0), len(e.lines)-1)
	e.col = min(e.col, len(e.line()))
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// wordEnd is the end of the word at or after the cursor.
func (e *editor) wordEnd() int {
	l, i := e.line(), e.col
	for i < len(l) && !isWordRune(l[i]) {
		i++
	}
	for i < len(l) && isWordRune(l[i]) {
		i++
	}
	return i
}

// wordStart is the start of the word before the cursor.
func (e *editor) wordStart() int {
	l, i := e.line(), e.col
	for i > 0 && !isWordRune(l[i-1]) {
		i--
	}
	for i > 0 && isWordRune(l[i-1]) {
		i--
	}
	return i
}
