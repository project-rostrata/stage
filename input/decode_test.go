package input

import (
	"reflect"
	"testing"
)

// The byte sequences are what VTE (GNOME Terminal) sends with its default profile.
func TestDecodeKeys(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Key
	}{
		{"letter", "j", []Key{{Code: Rune, Rune: 'j'}}},
		{"upper case", "G", []Key{{Code: Rune, Rune: 'G'}}},
		{"several keys in one read", "gg\x13", []Key{{Code: Rune, Rune: 'g'}, {Code: Rune, Rune: 'g'}, {Code: Rune, Rune: 's', Mod: Ctrl}}},
		{"wide character", "中", []Key{{Code: Rune, Rune: '中'}}},
		{"Ctrl+S", "\x13", []Key{{Code: Rune, Rune: 's', Mod: Ctrl}}},
		{"Ctrl+Q", "\x11", []Key{{Code: Rune, Rune: 'q', Mod: Ctrl}}},
		{"Ctrl+C", "\x03", []Key{{Code: Rune, Rune: 'c', Mod: Ctrl}}},
		{"Ctrl+J is LF", "\n", []Key{{Code: Rune, Rune: 'j', Mod: Ctrl}}},
		{"Enter is CR", "\r", []Key{{Code: Enter}}},
		{"Tab", "\t", []Key{{Code: Tab}}},
		{"Ctrl+Space", "\x00", []Key{{Code: Rune, Rune: ' ', Mod: Ctrl}}},
		{"Backspace", "\x7f", []Key{{Code: Backspace}}},
		{"Ctrl+Backspace", "\x08", []Key{{Code: Backspace, Mod: Ctrl}}},
		{"Ctrl+_", "\x1f", []Key{{Code: Rune, Rune: '_', Mod: Ctrl}}},
		{"Alt+f", "\x1bf", []Key{{Code: Rune, Rune: 'f', Mod: Alt}}},
		{"Alt+<", "\x1b<", []Key{{Code: Rune, Rune: '<', Mod: Alt}}},
		{"Alt+Backspace", "\x1b\x7f", []Key{{Code: Backspace, Mod: Alt}}},
		{"Alt+Ctrl+U", "\x1b\x15", []Key{{Code: Rune, Rune: 'u', Mod: Ctrl | Alt}}},
		{"Up", "\x1b[A", []Key{{Code: Up}}},
		{"Up in application mode", "\x1bOA", []Key{{Code: Up}}},
		{"Shift+Down", "\x1b[1;2B", []Key{{Code: Down, Mod: Shift}}},
		{"Ctrl+Right", "\x1b[1;5C", []Key{{Code: Right, Mod: Ctrl}}},
		{"Home", "\x1b[H", []Key{{Code: Home}}},
		{"End", "\x1b[F", []Key{{Code: End}}},
		{"Delete", "\x1b[3~", []Key{{Code: Delete}}},
		{"PgUp", "\x1b[5~", []Key{{Code: PgUp}}},
		{"PgDn", "\x1b[6~", []Key{{Code: PgDn}}},
		{"F5", "\x1b[15~", []Key{{Code: F5}}},
		{"Ctrl+F5", "\x1b[15;5~", []Key{{Code: F5, Mod: Ctrl}}},
		{"F1", "\x1bOP", []Key{{Code: F1}}},
		{"Shift+Tab", "\x1b[Z", []Key{{Code: BackTab}}},
		{"focus in", "\x1b[I", []Key{{Code: FocusIn}}},
		{"focus out", "\x1b[O", []Key{{Code: FocusOut}}},
		{"paste", "\x1b[200~line one\nline\ttwo\x1b[201~", []Key{{Code: Paste, Text: "line one\nline\ttwo"}}},
		{"pasted line breaks become LF", "\x1b[200~a\rb\r\nc\x1b[201~", []Key{{Code: Paste, Text: "a\nb\nc"}}},
		{"paste keeps control bytes as text", "\x1b[200~\x13\x1b[A\x1b[201~j", []Key{{Code: Paste, Text: "\x13\x1b[A"}, {Code: Rune, Rune: 'j'}}},
		{"unknown sequence is skipped", "\x1b[99xj", []Key{{Code: Unknown}, {Code: Rune, Rune: 'j'}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rest := Decode([]byte(tt.in), false)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode(%q) = %v, want %v", tt.in, got, tt.want)
			}
			if len(rest) != 0 {
				t.Errorf("Decode(%q) left %q", tt.in, rest)
			}
		})
	}
}

// Sequences can be split across reads. Until the read times out (final),
// an incomplete tail is kept for the next read instead of being guessed at.
func TestDecodeIncomplete(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantRest  string
		finalKeys []Key
	}{
		{"lone ESC is Esc only after the timeout", "\x1b", "\x1b", []Key{{Code: Esc}}},
		{"half a CSI sequence", "\x1b[1;", "\x1b[1;", []Key{{Code: Unknown}}},
		{"ESC [ alone is Alt+[", "\x1b[", "\x1b[", []Key{{Code: Rune, Rune: '[', Mod: Alt}}},
		{"ESC O alone is Alt+O", "\x1bO", "\x1bO", []Key{{Code: Rune, Rune: 'O', Mod: Alt}}},
		{"first byte of a UTF-8 character", "\xe4\xb8", "\xe4\xb8", []Key{{Code: Unknown}, {Code: Unknown}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, rest := Decode([]byte(tt.in), false)
			if len(keys) != 0 || string(rest) != tt.wantRest {
				t.Fatalf("Decode(%q, false) = %v, rest %q; want no keys, rest %q", tt.in, keys, rest, tt.wantRest)
			}
			keys, rest = Decode(rest, true)
			if !reflect.DeepEqual(keys, tt.finalKeys) || len(rest) != 0 {
				t.Errorf("Decode(%q, true) = %v, rest %q; want %v", tt.in, keys, rest, tt.finalKeys)
			}
		})
	}
}

func TestDecodeSplitAcrossReads(t *testing.T) {
	keys, rest := Decode([]byte("j\x1b[1"), false)
	if !reflect.DeepEqual(keys, []Key{{Code: Rune, Rune: 'j'}}) || string(rest) != "\x1b[1" {
		t.Fatalf("first read: %v, rest %q", keys, rest)
	}
	keys, rest = Decode(append(rest, ";2A"...), false)
	if !reflect.DeepEqual(keys, []Key{{Code: Up, Mod: Shift}}) || len(rest) != 0 {
		t.Errorf("second read: %v, rest %q", keys, rest)
	}
}

func TestPasteWaitsForItsEnd(t *testing.T) {
	keys, rest := Decode([]byte("\x1b[200~half a pas"), true)
	if len(keys) != 0 || string(rest) != "\x1b[200~half a pas" {
		t.Fatalf("incomplete paste: %v, rest %q", keys, rest)
	}
	keys, rest = Decode(append(rest, "te\x1b[201~"...), true)
	if !reflect.DeepEqual(keys, []Key{{Code: Paste, Text: "half a paste"}}) || len(rest) != 0 {
		t.Errorf("completed paste: %v, rest %q", keys, rest)
	}
}

func TestKeyString(t *testing.T) {
	tests := []struct {
		key  Key
		want string
	}{
		{Key{Code: Rune, Rune: 's', Mod: Ctrl}, "Ctrl+S"},
		{Key{Code: Rune, Rune: '<', Mod: Alt}, "Alt+<"},
		{Key{Code: Rune, Rune: ' ', Mod: Ctrl}, "Ctrl+Space"},
		{Key{Code: Down, Mod: Shift}, "Shift+Down"},
		{Key{Code: F5}, "F5"},
		{Key{Code: Rune, Rune: 'G'}, "G"},
	}
	for _, tt := range tests {
		if got := tt.key.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.key, got, tt.want)
		}
	}
}
