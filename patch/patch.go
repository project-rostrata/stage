// Package patch builds the partial patches stage hands to git apply: the
// lines chosen in the Diff pane, and only those. It is pure, with no I/O, so
// the logic deciding what gets staged or reverted is fully testable.
//
// The approach is git add -p's, which git-cola also uses. It is written here
// from the algorithm, not copied from either.
package patch

import (
	"errors"
	"fmt"
	"strings"

	"stage/git"
)

// Mode says which way the patch will be applied.
type Mode int

const (
	// Forward is applied as it is: staging chosen lines from the unstaged
	// diff onto the index. Its old side must match the index.
	Forward Mode = iota
	// Reverse is applied with -R: unstaging from the staged diff, or
	// reverting from the unstaged diff. Its new side must match the target.
	Reverse
)

// Selection is the chosen lines: for each hunk index, the indexes of the
// chosen lines within that hunk's Lines.
type Selection map[int]map[int]bool

// ErrNothingSelected means the selection holds no added or removed lines.
var ErrNothingSelected = errors.New("no changed lines selected")

// Build returns a patch containing only the selected changes of d.
//
// Every unselected change is turned into whatever leaves the target as it is:
//   - Forward: an unselected removed line is still in the index, so it becomes
//     context, and an unselected added line isn't there yet, so it is dropped.
//   - Reverse: the roles swap. An unselected added line is in the target, so
//     it becomes context, and an unselected removed line is dropped.
//
// The changed side's hunk starts are recomputed, so an earlier hunk that was
// left out or only partly included doesn't throw later ones off.
func Build(d git.FileDiff, sel Selection, mode Mode) (string, error) {
	if d.Binary || d.Combined {
		return "", errors.New("can't stage part of this file")
	}
	var body strings.Builder
	offset := 0 // how far the derived side has drifted from the fixed side, so far
	chosen, total := 0, 0
	for hi, h := range d.Hunks {
		lines, oldCount, newCount, n := hunkLines(h, sel[hi], mode)
		total += countChanges(h)
		if n == 0 {
			continue
		}
		chosen += n
		oldStart, newStart := starts(h, oldCount, newCount, offset, mode)
		fmt.Fprintf(&body, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, l := range lines {
			body.WriteString(l)
			body.WriteByte('\n')
		}
		if mode == Forward {
			offset += newCount - oldCount
		} else {
			offset += oldCount - newCount
		}
	}
	if chosen == 0 {
		return "", ErrNothingSelected
	}
	head, err := header(d.Header, chosen == total)
	if err != nil {
		return "", err
	}
	return head + body.String(), nil
}

// hunkLines returns one hunk's patch lines, its old and new line counts, and
// how many selected changes it holds.
func hunkLines(h git.Hunk, chosen map[int]bool, mode Mode) ([]string, int, int, int) {
	var out []string
	oldCount, newCount, n := 0, 0, 0
	lastKept := false
	for i, l := range h.Lines {
		kind := l.Kind
		switch {
		case kind == git.NoEOL:
			// "\ No newline at end of file" belongs to the line before it.
			if lastKept {
				out = append(out, `\`+l.Text)
			}
			continue
		case kind == git.Context:
		case chosen[i]:
			n++
		case (kind == git.Removed) == (mode == Forward):
			kind = git.Context // the line stays in the target
		default:
			lastKept = false // the line isn't in the target: leave it out
			continue
		}
		out = append(out, string(kind)+l.Text)
		lastKept = true
		if kind != git.Added {
			oldCount++
		}
		if kind != git.Removed {
			newCount++
		}
	}
	return out, oldCount, newCount, n
}

func countChanges(h git.Hunk) int {
	n := 0
	for _, l := range h.Lines {
		if l.Kind == git.Added || l.Kind == git.Removed {
			n++
		}
	}
	return n
}

// starts computes a hunk's two start lines. The side the patch is matched
// against (old for Forward, new for Reverse) keeps git's original start; the
// other side is derived from it plus the drift from earlier hunks. A side
// with no lines starts at the line before, by unified diff convention.
func starts(h git.Hunk, oldCount, newCount, offset int, mode Mode) (int, int) {
	first := func(start, count int) int {
		if count == 0 {
			return start + 1
		}
		return start
	}
	fromFirst := func(firstLine, count int) int {
		if count == 0 {
			return firstLine - 1
		}
		return firstLine
	}
	if mode == Forward {
		return h.OldStart, fromFirst(first(h.OldStart, h.OldCount)+offset, newCount)
	}
	return fromFirst(first(h.NewStart, h.NewCount)+offset, oldCount), h.NewStart
}

// header writes the file header. It always describes a change to one path,
// the new one: a rename or copy is never re-applied or undone by a line
// patch. A file's creation or deletion is kept only when every change is
// included, because a partial patch leaves the file in place.
func header(lines []string, whole bool) (string, error) {
	var minus, plus, created, deleted string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "--- "):
			minus = strings.TrimPrefix(l, "--- ")
		case strings.HasPrefix(l, "+++ "):
			plus = strings.TrimPrefix(l, "+++ ")
		case strings.HasPrefix(l, "new file mode "):
			created = l
		case strings.HasPrefix(l, "deleted file mode "):
			deleted = l
		}
	}
	if minus == "" || plus == "" {
		return "", errors.New("diff has no ---/+++ header")
	}
	newName := plus
	if newName == "/dev/null" {
		newName = swapPrefix(minus, "a/", "b/")
	}
	oldName := swapPrefix(newName, "b/", "a/")

	var b strings.Builder
	fmt.Fprintf(&b, "diff --git %s %s\n", strings.TrimSuffix(oldName, "\t"), strings.TrimSuffix(newName, "\t"))
	fromName, toName := oldName, newName
	if whole && created != "" {
		b.WriteString(created + "\n")
		fromName = "/dev/null"
	}
	if whole && deleted != "" {
		b.WriteString(deleted + "\n")
		toName = "/dev/null"
	}
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", fromName, toName)
	return b.String(), nil
}

// swapPrefix changes a path's a/ or b/ prefix, keeping git's quoting if the
// path is quoted.
func swapPrefix(name, from, to string) string {
	if rest, ok := strings.CutPrefix(name, `"`+from); ok {
		return `"` + to + rest
	}
	if rest, ok := strings.CutPrefix(name, from); ok {
		return to + rest
	}
	return name
}
