package ui

import (
	"strings"
	"testing"
	"time"

	"stage/git"
	"stage/screen"
)

func TestBranchViewKeepsTheCursorOnItsBranch(t *testing.T) {
	v := branchView{loc: time.UTC}
	v.load([]git.Branch{{Name: "main"}, {Name: "feature"}, {Name: "old"}}, time.Time{})
	v.move(1)
	// feature gets a new commit and moves to the top.
	v.load([]git.Branch{{Name: "feature"}, {Name: "main"}, {Name: "old"}}, time.Time{})
	if b, _ := v.selected(); b.Name != "feature" {
		t.Errorf("selected %q after reload, want feature", b.Name)
	}
	// Once it's gone, the cursor goes back to the top.
	v.load([]git.Branch{{Name: "main"}, {Name: "old"}}, time.Time{})
	if b, _ := v.selected(); b.Name != "main" {
		t.Errorf("selected %q after feature was deleted, want main", b.Name)
	}
}

// drawn draws the view into a grid and returns its inner rows, trimmed.
func drawn(v *branchView, w, h int) []string {
	g := screen.NewGrid(w, h)
	v.draw(g, rect{0, 0, w, h}, darkPalette())
	return innerRows(g)
}

// innerRows returns the rows inside a grid's border, trimmed.
func innerRows(g *screen.Grid) []string {
	w, h := g.Size()
	var rows []string
	for y := 1; y < h-1; y++ {
		var b strings.Builder
		for x := 1; x < w-1; x++ {
			b.WriteString(g.Cell(x, y).Text)
		}
		rows = append(rows, strings.TrimRight(b.String(), " "))
	}
	return rows
}

func TestBranchViewRows(t *testing.T) {
	zone := time.FixedZone("EDT", -4*60*60)
	at := time.Date(2026, 10, 13, 16, 38, 0, 0, zone)
	v := branchView{loc: zone}
	v.load([]git.Branch{
		{Name: "main", Current: true, Time: at.UTC(), Subject: "Harden stage against a hostile repository"},
		{Name: "a-rather-long-branch-name", Behind: 3, Time: at.Add(-time.Hour), Subject: "Short"},
		{Name: "old", Gone: true, Time: at.Add(-24 * time.Hour), Subject: "Old work"},
	}, time.Time{})
	got := drawn(&v, 62, 5)
	// 58 columns to draw in: the name column is a third of them, and the
	// behind column is as wide as its widest entry, on every row.
	want := []string{
		" * main                       2026-10-13 16:38  Harden sta…",
		"   a-rather-long-bran…  ↓3    2026-10-13 15:38  Short",
		"   old                  gone  2026-10-12 16:38  Old work",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
	if v.title() != "Branches · never fetched" {
		t.Errorf("title %q", v.title())
	}
	v.fetched = at
	if v.title() != "Branches · fetched 2026-10-13 16:38" {
		t.Errorf("title %q", v.title())
	}
}
