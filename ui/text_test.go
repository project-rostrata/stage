package ui

import (
	"reflect"
	"testing"
)

func TestExpandTabs(t *testing.T) {
	tests := []struct{ in, want string }{
		{"no tabs", "no tabs"},
		{"\tx", "        x"},
		{"ab\tx", "ab      x"},
		{"中\tx", "中      x"}, // a wide character counts as two columns
		{"12345678\tx", "12345678        x"},
	}
	for _, tt := range tests {
		if got := expandTabs(tt.in); got != tt.want {
			t.Errorf("expandTabs(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  []string
	}{
		{"short", 10, []string{"short"}},
		{"", 10, []string{""}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"ab中文", 3, []string{"ab", "中", "文"}}, // a wide character never straddles two pieces
		{"aébc", 2, []string{"aé", "bc"}},
	}
	for _, tt := range tests {
		if got := wrap(tt.in, tt.width); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("wrap(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}

func TestShortenLeft(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"main.go", 10, "main.go"},
		{"dir with space/new.txt", 10, "…e/new.txt"},
		{"日本語/ファイル", 7, "…ァイル"}, // the ellipsis and three wide characters fill 7 columns
	}
	for _, tt := range tests {
		if got := shortenLeft(tt.in, tt.width); got != tt.want {
			t.Errorf("shortenLeft(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}

func TestLayoutMatchesGitCola(t *testing.T) {
	l := newLayout(120, 40)
	if l.tooSmall {
		t.Fatal("120x40 should fit")
	}
	if l.commit.w != 30 || l.status.w != 30 || l.diff.x != 30 || l.diff.w != 90 {
		t.Errorf("left column should be a quarter of the width: %+v", l)
	}
	if l.commit.h+l.status.h != 39 || l.diff.h != 39 || l.bar.y != 39 {
		t.Errorf("panes should fill all but the status bar row: %+v", l)
	}
	if small := newLayout(40, 10); !small.tooSmall {
		t.Errorf("40x10 should be too small")
	}
}

func TestShortenRight(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly", 7, "exactly"},
		{"abcdefgh", 5, "abcd…"},
		{"中文字", 4, "中…"}, // a wide character that doesn't fit is left out whole
		{"abc", 0, ""},
	}
	for _, tt := range tests {
		if got := shortenRight(tt.in, tt.width); got != tt.want {
			t.Errorf("shortenRight(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}
