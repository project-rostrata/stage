package input

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	esc        = 0x1b
	pasteStart = "200"
	pasteEnd   = "\x1b[201~"
)

// Decode turns raw terminal bytes into keys. It returns the keys it decoded and
// any bytes left over from an incomplete sequence at the end of b, which the
// caller keeps and passes back with the next read.
//
// final says no more bytes arrived in time. A dangling ESC is then the Esc key
// itself rather than the start of a sequence. A paste is the exception: it is
// always waited for until its end marker arrives.
func Decode(b []byte, final bool) ([]Key, []byte) {
	var keys []Key
	for len(b) > 0 {
		k, n := decodeOne(b, final)
		if n == 0 {
			break
		}
		keys = append(keys, k)
		b = b[n:]
	}
	return keys, b
}

// decodeOne decodes the key at the start of b and how many bytes it used.
// n == 0 means the sequence is incomplete.
func decodeOne(b []byte, final bool) (Key, int) {
	c := b[0]
	switch {
	case c == esc:
		return decodeEsc(b, final)
	case c < 0x20 || c == 0x7f:
		return control(c), 1
	case c < 0x80:
		return Key{Code: Rune, Rune: rune(c)}, 1
	}
	if !utf8.FullRune(b) {
		if final {
			return Key{Code: Unknown}, 1
		}
		return Key{}, 0
	}
	r, size := utf8.DecodeRune(b)
	if r == utf8.RuneError && size == 1 {
		return Key{Code: Unknown}, 1
	}
	return Key{Code: Rune, Rune: r}, size
}

// control maps a C0 control byte, or DEL, to its key. With ICRNL off, Enter
// sends CR, so LF is Ctrl+J. VTE sends BS for Ctrl+Backspace and DEL for Backspace.
func control(c byte) Key {
	switch c {
	case 0x00:
		return Key{Code: Rune, Rune: ' ', Mod: Ctrl}
	case 0x08:
		return Key{Code: Backspace, Mod: Ctrl}
	case '\t':
		return Key{Code: Tab}
	case '\r':
		return Key{Code: Enter}
	case 0x7f:
		return Key{Code: Backspace}
	}
	if c <= 0x1a {
		return Key{Code: Rune, Rune: rune('a' + c - 1), Mod: Ctrl}
	}
	// 0x1c-0x1f: Ctrl+\ Ctrl+] Ctrl+^ Ctrl+_
	return Key{Code: Rune, Rune: rune(c + 0x40), Mod: Ctrl}
}

func decodeEsc(b []byte, final bool) (Key, int) {
	if len(b) == 1 {
		if final {
			return Key{Code: Esc}, 1
		}
		return Key{}, 0
	}
	switch b[1] {
	case '[':
		return decodeCSI(b, final)
	case 'O':
		return decodeSS3(b, final)
	case esc:
		return Key{Code: Esc}, 1
	}
	// Anything else is Alt held with that key, sent as an ESC prefix.
	k, n := decodeOne(b[1:], final)
	if n == 0 {
		return Key{}, 0
	}
	k.Mod |= Alt
	return k, n + 1
}

// decodeSS3 handles ESC O x, which some terminals send for arrows, Home/End and F1-F4.
func decodeSS3(b []byte, final bool) (Key, int) {
	if len(b) == 2 {
		if final {
			return Key{Code: Rune, Rune: 'O', Mod: Alt}, 2
		}
		return Key{}, 0
	}
	if code, ok := letterKey(b[2]); ok {
		return Key{Code: code}, 3
	}
	return Key{Code: Rune, Rune: 'O', Mod: Alt}, 2
}

// decodeCSI handles ESC [ params final.
func decodeCSI(b []byte, final bool) (Key, int) {
	i := 2
	for i < len(b) && b[i] >= 0x20 && b[i] <= 0x3f {
		i++
	}
	if i == len(b) {
		if final && i == 2 {
			return Key{Code: Rune, Rune: '[', Mod: Alt}, 2
		}
		if final {
			return Key{Code: Unknown}, i
		}
		return Key{}, 0
	}
	f := b[i]
	n := i + 1
	if f < 0x40 || f > 0x7e {
		return Key{Code: Unknown}, i
	}
	params := strings.Split(string(b[2:i]), ";")

	if f == '~' && params[0] == pasteStart {
		end := bytes.Index(b[n:], []byte(pasteEnd))
		if end < 0 {
			return Key{}, 0
		}
		return Key{Code: Paste, Text: normalizeNewlines(string(b[n : n+end]))}, n + end + len(pasteEnd)
	}

	mod := modifiers(params)
	if f == '~' {
		return Key{Code: tildeKey(params[0]), Mod: mod}, n
	}
	if len(b[2:i]) == 0 {
		switch f {
		case 'I':
			return Key{Code: FocusIn}, n
		case 'O':
			return Key{Code: FocusOut}, n
		case 'Z':
			return Key{Code: BackTab}, n
		}
	}
	if code, ok := letterKey(f); ok {
		return Key{Code: code, Mod: mod}, n
	}
	return Key{Code: Unknown}, n
}

// normalizeNewlines turns the CR or CRLF that terminals send for pasted line
// breaks into LF.
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// letterKey maps the final letter of CSI and SS3 sequences.
func letterKey(f byte) (Code, bool) {
	switch f {
	case 'A':
		return Up, true
	case 'B':
		return Down, true
	case 'C':
		return Right, true
	case 'D':
		return Left, true
	case 'H':
		return Home, true
	case 'F':
		return End, true
	case 'P':
		return F1, true
	case 'Q':
		return F2, true
	case 'R':
		return F3, true
	case 'S':
		return F4, true
	}
	return Unknown, false
}

// tildeKey maps the number in ESC [ n ~ sequences.
func tildeKey(p string) Code {
	switch p {
	case "1", "7":
		return Home
	case "2":
		return Insert
	case "3":
		return Delete
	case "4", "8":
		return End
	case "5":
		return PgUp
	case "6":
		return PgDn
	case "11":
		return F1
	case "12":
		return F2
	case "13":
		return F3
	case "14":
		return F4
	case "15":
		return F5
	case "17":
		return F6
	case "18":
		return F7
	case "19":
		return F8
	case "20":
		return F9
	case "21":
		return F10
	case "23":
		return F11
	case "24":
		return F12
	}
	return Unknown
}

// modifiers reads xterm's modifier parameter (the second one): 1 + a bitmask of
// Shift 1, Alt 2, Ctrl 4, and Meta 8, which stage treats as Alt.
func modifiers(params []string) Mod {
	if len(params) < 2 {
		return 0
	}
	m, err := strconv.Atoi(params[1])
	if err != nil || m < 2 {
		return 0
	}
	bits := m - 1
	var mod Mod
	if bits&1 != 0 {
		mod |= Shift
	}
	if bits&(2|8) != 0 {
		mod |= Alt
	}
	if bits&4 != 0 {
		mod |= Ctrl
	}
	return mod
}
