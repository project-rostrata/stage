package git

import (
	"fmt"
	"strconv"
	"strings"
)

// LineKind is the first column of a diff line.
type LineKind byte

// The kinds of line in a hunk.
const (
	Context LineKind = ' '
	Added   LineKind = '+'
	Removed LineKind = '-'
	NoEOL   LineKind = '\\' // "\ No newline at end of file"
)

// Line is one line of a hunk, without its first column.
type Line struct {
	Kind LineKind
	Text string
	Old  int // line number in the old file, or 0 for an added line
	New  int // line number in the new file, or 0 for a removed line
}

// Hunk is one @@ section of a diff.
type Hunk struct {
	Header             string // the whole @@ line
	OldStart, OldCount int
	NewStart, NewCount int
	Lines              []Line
}

// FileDiff is the diff of one path.
type FileDiff struct {
	// Header is everything before the first hunk: the diff --git line, mode and
	// rename lines, index, --- and +++. Partial patches reuse it.
	Header []string
	Hunks  []Hunk
	Binary bool
	// Combined is set for an unmerged path, whose diff compares several
	// parents. Its hunks are shown as they are, and nothing can be staged from them.
	Combined bool
}

// diffCommand builds a git diff command line whose output keeps the shape stage
// parses and git apply accepts, whatever the human's diff settings: plain a/ and
// b/ prefixes, no colour, no external diff tool, no textconv, and paths from the
// top level.
func diffCommand(opts []string, paths ...string) []string {
	args := append([]string{"diff"}, opts...)
	args = append(args, "--no-color", "--no-ext-diff", "--no-textconv", "--no-relative",
		"--src-prefix=a/", "--dst-prefix=b/", "--")
	return append(args, paths...)
}

// Diff returns the diff that the Status pane shows for e: staged changes for a
// Staged entry, unstaged for Modified and Unmerged, and the whole file as
// added for Untracked.
func (r *Repo) Diff(e Entry) (FileDiff, error) {
	var args []string
	var ok []int
	switch e.Section {
	case Staged:
		if e.OrigPath != "" {
			args = diffCommand([]string{"--cached", "-M"}, e.OrigPath, e.Path)
		} else {
			args = diffCommand([]string{"--cached", "-M"}, e.Path)
		}
	case Modified, Unmerged:
		args = diffCommand(nil, e.Path)
	case Untracked:
		args = diffCommand([]string{"--no-index"}, "/dev/null", e.Path)
		ok = []int{1}
	}
	out, err := r.read(ok, args...)
	if err != nil {
		return FileDiff{}, err
	}
	return parseDiff(string(out))
}

// parseDiff reads the unified diff of a single file.
func parseDiff(out string) (FileDiff, error) {
	var d FileDiff
	lines := strings.Split(out, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var hunk *Hunk
	oldNo, newNo := 0, 0
	for _, l := range lines {
		if strings.HasPrefix(l, "@@@") {
			d.Combined = true
		}
		if strings.HasPrefix(l, "@@") {
			h, err := parseHunkHeader(l, d.Combined)
			if err != nil {
				return FileDiff{}, err
			}
			d.Hunks = append(d.Hunks, h)
			hunk = &d.Hunks[len(d.Hunks)-1]
			oldNo, newNo = h.OldStart, h.NewStart
			continue
		}
		if hunk == nil {
			if strings.HasPrefix(l, "Binary files ") || l == "GIT binary patch" {
				d.Binary = true
			}
			d.Header = append(d.Header, l)
			continue
		}
		if d.Combined {
			hunk.Lines = append(hunk.Lines, Line{Kind: Context, Text: l})
			continue
		}
		if l == "" {
			// Some tools strip the space from an empty context line.
			l = " "
		}
		line := Line{Kind: LineKind(l[0]), Text: l[1:]}
		switch line.Kind {
		case Context:
			line.Old, line.New = oldNo, newNo
			oldNo++
			newNo++
		case Added:
			line.New = newNo
			newNo++
		case Removed:
			line.Old = oldNo
			oldNo++
		case NoEOL:
		default:
			return FileDiff{}, fmt.Errorf("diff: unexpected line %q", l)
		}
		hunk.Lines = append(hunk.Lines, line)
	}
	return d, nil
}

// parseHunkHeader reads "@@ -a,b +c,d @@ section". A missing count means 1.
// For a combined diff only the header text is kept.
func parseHunkHeader(l string, combined bool) (Hunk, error) {
	h := Hunk{Header: l}
	if combined {
		return h, nil
	}
	f := strings.Fields(l)
	if len(f) < 4 || f[0] != "@@" || f[3] != "@@" {
		return Hunk{}, fmt.Errorf("diff: bad hunk header %q", l)
	}
	var err error
	if h.OldStart, h.OldCount, err = parseRange(f[1], '-'); err != nil {
		return Hunk{}, fmt.Errorf("diff: bad hunk header %q", l)
	}
	if h.NewStart, h.NewCount, err = parseRange(f[2], '+'); err != nil {
		return Hunk{}, fmt.Errorf("diff: bad hunk header %q", l)
	}
	return h, nil
}

func parseRange(s string, sign byte) (int, int, error) {
	if len(s) < 2 || s[0] != sign {
		return 0, 0, fmt.Errorf("bad range %q", s)
	}
	start, count, found := strings.Cut(s[1:], ",")
	a, err := strconv.Atoi(start)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return a, 1, nil
	}
	b, err := strconv.Atoi(count)
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}
