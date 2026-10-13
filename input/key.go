// Package input turns the bytes a terminal sends into key events.
package input

import (
	"fmt"
	"strings"
)

// Code says what kind of key a Key is.
type Code int

// Key codes. Rune covers every printable character and every Ctrl+letter.
const (
	Rune Code = iota
	Enter
	Tab
	BackTab
	Backspace
	Delete
	Insert
	Esc
	Up
	Down
	Left
	Right
	Home
	End
	PgUp
	PgDn
	F1
	F2
	F3
	F4
	F5
	F6
	F7
	F8
	F9
	F10
	F11
	F12
	FocusIn
	FocusOut
	Paste
	Unknown
)

// Mod is a set of modifier keys.
type Mod uint8

// Modifiers. Ctrl+letter is reported as Rune with the lower-case letter and Ctrl.
const (
	Shift Mod = 1 << iota
	Alt
	Ctrl
)

// Key is one key press, focus change, or paste.
type Key struct {
	Code Code
	Rune rune   // for Code Rune
	Mod  Mod    // modifiers held
	Text string // for Code Paste: the pasted text, verbatim except that line breaks are \n
}

// Is reports whether k is the rune r with exactly the modifiers m.
func (k Key) Is(r rune, m Mod) bool { return k.Code == Rune && k.Rune == r && k.Mod == m }

// IsCode reports whether k is the special key c with exactly the modifiers m.
func (k Key) IsCode(c Code, m Mod) bool { return k.Code == c && k.Mod == m }

var codeNames = [...]string{
	Rune: "Rune", Enter: "Enter", Tab: "Tab", BackTab: "Shift+Tab", Backspace: "Backspace",
	Delete: "Delete", Insert: "Insert", Esc: "Esc", Up: "Up", Down: "Down", Left: "Left",
	Right: "Right", Home: "Home", End: "End", PgUp: "PgUp", PgDn: "PgDn",
	F1: "F1", F2: "F2", F3: "F3", F4: "F4", F5: "F5", F6: "F6", F7: "F7", F8: "F8",
	F9: "F9", F10: "F10", F11: "F11", F12: "F12",
	FocusIn: "FocusIn", FocusOut: "FocusOut", Paste: "Paste", Unknown: "Unknown",
}

// String names the key the way the brief's key table does, for example "Ctrl+S" or "Alt+<".
func (k Key) String() string {
	var b strings.Builder
	if k.Mod&Ctrl != 0 {
		b.WriteString("Ctrl+")
	}
	if k.Mod&Alt != 0 {
		b.WriteString("Alt+")
	}
	if k.Mod&Shift != 0 {
		b.WriteString("Shift+")
	}
	switch {
	case k.Code == Paste:
		return fmt.Sprintf("Paste(%q)", k.Text)
	case k.Code == Rune && k.Rune == ' ':
		b.WriteString("Space")
	case k.Code == Rune && k.Mod&Ctrl != 0:
		b.WriteString(strings.ToUpper(string(k.Rune)))
	case k.Code == Rune:
		b.WriteRune(k.Rune)
	default:
		b.WriteString(codeNames[k.Code])
	}
	return b.String()
}
