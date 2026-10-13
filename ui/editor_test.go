package ui

import (
	"testing"

	"stage/input"
)

func keys(e *editor, ks ...input.Key) {
	for _, k := range ks {
		e.handle(k)
	}
}

func typ(e *editor, s string) {
	for _, r := range s {
		e.handle(input.Key{Code: input.Rune, Rune: r})
	}
}

var (
	enter     = input.Key{Code: input.Enter}
	backspace = input.Key{Code: input.Backspace}
)

func ctrl(r rune) input.Key { return input.Key{Code: input.Rune, Rune: r, Mod: input.Ctrl} }
func alt(r rune) input.Key  { return input.Key{Code: input.Rune, Rune: r, Mod: input.Alt} }

func TestMessageRoundTrip(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"Summary only\n", "Summary only\n"},
		{"Summary\n\nBody line one\nBody line two\n", "Summary\n\nBody line one\nBody line two\n"},
		{"Summary\n\nBody\n\n\n", "Summary\n\nBody\n"},
		{"Summary\n\nPara one\n\nPara two\n", "Summary\n\nPara one\n\nPara two\n"},
	}
	for _, tt := range tests {
		e := newEditor(tt.in)
		if got := e.message(); got != tt.want {
			t.Errorf("newEditor(%q).message() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTypingAMessage(t *testing.T) {
	e := newEditor("")
	typ(&e, "Fix the thing")
	keys(&e, enter)
	typ(&e, "It was broken.")
	keys(&e, enter, enter)
	typ(&e, "Now it isn't.")
	if got, want := e.message(), "Fix the thing\n\nIt was broken.\n\nNow it isn't.\n"; got != want {
		t.Errorf("message %q, want %q", got, want)
	}
}

func TestEnterInTheSummaryMovesToTheDescription(t *testing.T) {
	e := newEditor("Summary\n\nBody\n")
	keys(&e, ctrl('a'), ctrl('f'), enter)
	if e.row != 1 || e.col != 0 || e.summary() != "Summary" {
		t.Errorf("row %d col %d summary %q", e.row, e.col, e.summary())
	}
}

func TestBackspaceJoinsDescriptionLinesButNotTheSummary(t *testing.T) {
	e := newEditor("Summary\n\nfirst\nsecond\n")
	e.row, e.col = 2, 0
	keys(&e, backspace)
	if got := e.message(); got != "Summary\n\nfirstsecond\n" {
		t.Errorf("after joining: %q", got)
	}
	e.row, e.col = 1, 0
	keys(&e, backspace)
	if got := e.message(); got != "Summary\n\nfirstsecond\n" {
		t.Errorf("backspace at the description's start changed it: %q", got)
	}
}

func TestKillAndYank(t *testing.T) {
	e := newEditor("Summary\n\none two three\nnext\n")
	e.row, e.col = 1, 4
	keys(&e, ctrl('k'))
	if got := e.lines[1]; got != "one " {
		t.Fatalf("Ctrl+K: %q", got)
	}
	keys(&e, ctrl('k')) // at the end of the line, Ctrl+K joins the next one
	if got := e.lines[1]; got != "one next" {
		t.Fatalf("Ctrl+K at end of line: %q", got)
	}
	keys(&e, ctrl('a'), ctrl('k'), ctrl('y'), ctrl('y'))
	if got := e.lines[1]; got != "one nextone next" {
		t.Errorf("Ctrl+Y: %q", got)
	}
}

func TestWords(t *testing.T) {
	e := newEditor("Fix the café's door")
	keys(&e, ctrl('a'), alt('f'), alt('f'))
	if e.col != 7 {
		t.Errorf("Alt+F twice: col %d", e.col)
	}
	keys(&e, alt('d'))
	if e.summary() != "Fix the's door" {
		t.Errorf("Alt+D: %q", e.summary())
	}
	keys(&e, ctrl('e'), input.Key{Code: input.Backspace, Mod: input.Alt})
	if e.summary() != "Fix the's " {
		t.Errorf("Alt+Backspace: %q", e.summary())
	}
	keys(&e, ctrl('w'))
	if e.summary() != "Fix the'" {
		t.Errorf("Ctrl+W: %q", e.summary())
	}
}

func TestPasteIntoTheSummary(t *testing.T) {
	e := newEditor("")
	e.handle(input.Key{Code: input.Paste, Text: "Title\nbody one\nbody two"})
	if got, want := e.message(), "Title\n\nbody one\nbody two\n"; got != want {
		t.Errorf("paste: %q, want %q", got, want)
	}
	if e.row != 2 || e.col != len("body two") {
		t.Errorf("cursor after paste: row %d col %d", e.row, e.col)
	}
}

func TestPasteAWholeMessage(t *testing.T) {
	e := newEditor("")
	e.handle(input.Key{Code: input.Paste, Text: "Title\n\nBody"})
	if got, want := e.message(), "Title\n\nBody\n"; got != want {
		t.Errorf("paste: %q, want %q", got, want)
	}
}

func TestCursorMovementAcrossLines(t *testing.T) {
	e := newEditor("ab\n\ncd\n")
	keys(&e, ctrl('e'), ctrl('f'))
	if e.row != 1 || e.col != 0 {
		t.Errorf("Ctrl+F at end of summary: row %d col %d", e.row, e.col)
	}
	keys(&e, ctrl('b'))
	if e.row != 0 || e.col != 2 {
		t.Errorf("Ctrl+B at start of description: row %d col %d", e.row, e.col)
	}
	keys(&e, ctrl('n'), ctrl('e'), ctrl('p'))
	if e.row != 0 || e.col != 2 {
		t.Errorf("Ctrl+P keeps the column within the line: row %d col %d", e.row, e.col)
	}
}
