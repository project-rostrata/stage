package ui

import (
	"strings"

	"stage/screen"
)

const tabWidth = 8

// expandTabs replaces each tab with spaces up to the next tab stop, counting
// columns from the start of s.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col += screen.DrawnWidth(r)
	}
	return b.String()
}

// wrap splits s into pieces at most width columns wide. A wide character
// never straddles two pieces, and zero-width characters stay with the
// character before them. It always returns at least one piece.
func wrap(s string, width int) []string {
	if width < 2 || screen.StringWidth(s) <= width {
		return []string{s}
	}
	var pieces []string
	var cur strings.Builder
	col := 0
	for _, r := range s {
		w := screen.DrawnWidth(r)
		if col+w > width {
			pieces = append(pieces, cur.String())
			cur.Reset()
			col = 0
		}
		cur.WriteRune(r)
		col += w
	}
	return append(pieces, cur.String())
}

// shortenLeft fits s into width columns by replacing its start with "…", so
// a long path keeps its file name.
func shortenLeft(s string, width int) string {
	if screen.StringWidth(s) <= width || width < 2 {
		return s
	}
	runes := []rune(s)
	col := 1 // the ellipsis
	i := len(runes)
	for i > 0 && col+screen.DrawnWidth(runes[i-1]) <= width {
		i--
		col += screen.DrawnWidth(runes[i])
	}
	return "…" + string(runes[i:])
}
