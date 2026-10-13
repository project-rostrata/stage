package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	out := strings.Join([]string{
		"# branch.oid 1234",
		"# branch.head main",
		"1 M. N... 100644 100644 100644 aaa bbb staged.go",
		"1 .M N... 100644 100644 100644 aaa aaa modified.go",
		"1 MM N... 100644 100644 100644 aaa bbb both.go",
		"1 A. N... 000000 100644 100644 000 bbb dir/with space.txt",
		"1 .D N... 100644 100644 000000 aaa aaa deleted.txt",
		"2 R. N... 100644 100644 100644 aaa aaa R100 new name.go", "old name.go",
		"u UU N... 100644 100644 100644 100644 a b c conflict.go",
		"? untracked/file.txt",
		"? new\nline.txt",
		"",
	}, "\x00")
	got, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := Status{
		Branch: "main",
		Staged: []Entry{
			{Staged, 'M', "staged.go", ""},
			{Staged, 'M', "both.go", ""},
			{Staged, 'A', "dir/with space.txt", ""},
			{Staged, 'R', "new name.go", "old name.go"},
		},
		Modified: []Entry{
			{Modified, 'M', "modified.go", ""},
			{Modified, 'M', "both.go", ""},
			{Modified, 'D', "deleted.txt", ""},
		},
		Untracked: []Entry{
			{Untracked, '?', "untracked/file.txt", ""},
			{Untracked, '?', "new\nline.txt", ""},
		},
		Unmerged: []Entry{{Unmerged, 'U', "conflict.go", ""}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseStatus:\n got %+v\nwant %+v", got, want)
	}
}

func TestParseStatusDetachedHead(t *testing.T) {
	got, err := parseStatus([]byte("# branch.head (detached)\x00"))
	if err != nil || got.Branch != "" {
		t.Errorf("detached HEAD: %+v, %v", got, err)
	}
}

func TestParseDiff(t *testing.T) {
	out := `diff --git a/f.txt b/f.txt
index 1111111..2222222 100644
--- a/f.txt
+++ b/f.txt
@@ -1,4 +1,4 @@ func main()
 one
-two
+TWO
 three
@@ -10 +10,2 @@
 ten
+eleven
\ No newline at end of file
`
	d, err := parseDiff(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Header) != 4 || d.Binary || d.Combined || len(d.Hunks) != 2 {
		t.Fatalf("parseDiff: %+v", d)
	}
	h := d.Hunks[0]
	if h.Header != "@@ -1,4 +1,4 @@ func main()" || h.OldStart != 1 || h.OldCount != 4 || h.NewStart != 1 || h.NewCount != 4 {
		t.Errorf("first hunk header: %+v", h)
	}
	wantLines := []Line{
		{Context, "one", 1, 1},
		{Removed, "two", 2, 0},
		{Added, "TWO", 0, 2},
		{Context, "three", 3, 3},
	}
	if !reflect.DeepEqual(h.Lines, wantLines) {
		t.Errorf("first hunk lines:\n got %+v\nwant %+v", h.Lines, wantLines)
	}
	h = d.Hunks[1]
	if h.OldStart != 10 || h.OldCount != 1 || h.NewCount != 2 {
		t.Errorf("a missing count means 1: %+v", h)
	}
	if last := h.Lines[len(h.Lines)-1]; last.Kind != NoEOL {
		t.Errorf("last line should be the no-newline marker: %+v", last)
	}
}

// repo creates a temporary repository with one commit. Global and system git
// config are ignored, so the human's own settings can't change the result;
// TestDiffIgnoresHostileConfig puts some back on purpose.
func repo(t *testing.T) (*Repo, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.name", "Test")
	run(t, dir, "config", "user.email", "test@example.com")
	write(t, dir, "a.txt", "one\ntwo\nthree\n")
	write(t, dir, "old.txt", "rename me\nplease\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "first")
	r, err := Open(filepath.Join(dir, "."))
	if err != nil {
		t.Fatal(err)
	}
	return r, dir
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusAndDiffAgainstGit(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "a.txt", "one\nTWO\nthree\n")
	run(t, dir, "add", "a.txt")
	write(t, dir, "a.txt", "one\nTWO\nthree\nfour\n")
	run(t, dir, "mv", "old.txt", "new.txt")
	write(t, dir, "sub dir/new file.txt", "hello\n")
	write(t, dir, "bin.dat", "\x00\x01\x02")

	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" {
		t.Errorf("branch %q", st.Branch)
	}
	wantStaged := []Entry{{Staged, 'M', "a.txt", ""}, {Staged, 'R', "new.txt", "old.txt"}}
	if !reflect.DeepEqual(st.Staged, wantStaged) {
		t.Errorf("staged %+v", st.Staged)
	}
	if !reflect.DeepEqual(st.Modified, []Entry{{Modified, 'M', "a.txt", ""}}) {
		t.Errorf("modified %+v", st.Modified)
	}
	wantUntracked := []Entry{{Untracked, '?', "bin.dat", ""}, {Untracked, '?', "sub dir/new file.txt", ""}}
	if !reflect.DeepEqual(st.Untracked, wantUntracked) {
		t.Errorf("untracked %+v", st.Untracked)
	}

	staged, err := r.Diff(st.Staged[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := lineTexts(staged); got != "-two +TWO" {
		t.Errorf("staged diff of a.txt: %q", got)
	}
	unstaged, err := r.Diff(st.Modified[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := lineTexts(unstaged); got != "+four" {
		t.Errorf("unstaged diff of a.txt: %q", got)
	}
	renamed, err := r.Diff(st.Staged[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(renamed.Hunks) != 0 || !contains(renamed.Header, "rename from old.txt") {
		t.Errorf("rename diff: %+v", renamed)
	}
	untracked, err := r.Diff(st.Untracked[1])
	if err != nil {
		t.Fatal(err)
	}
	// git ends ---/+++ lines with a tab when the path has a space; the header is kept verbatim.
	if got := lineTexts(untracked); got != "+hello" || !contains(untracked.Header, "+++ b/sub dir/new file.txt\t") {
		t.Errorf("untracked diff: %+v", untracked)
	}
	binary, err := r.Diff(st.Untracked[0])
	if err != nil {
		t.Fatal(err)
	}
	if !binary.Binary {
		t.Errorf("bin.dat should be binary: %+v", binary)
	}
}

func TestDiffIgnoresHostileConfig(t *testing.T) {
	r, dir := repo(t)
	run(t, dir, "config", "diff.noprefix", "true")
	run(t, dir, "config", "diff.mnemonicPrefix", "true")
	run(t, dir, "config", "color.ui", "always")
	run(t, dir, "config", "diff.external", "false")
	write(t, dir, "a.txt", "one\ntwo\nthree\nfour\n")
	d, err := r.Diff(Entry{Section: Modified, Code: 'M', Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(d.Header, "--- a/a.txt") || !contains(d.Header, "+++ b/a.txt") || lineTexts(d) != "+four" {
		t.Errorf("diff under hostile config: %+v", d)
	}
}

func TestOpenOutsideARepository(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	if _, err := Open(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("Open outside a repository: %v", err)
	}
}

// lineTexts summarises a diff's added and removed lines, e.g. "-two +TWO".
func lineTexts(d FileDiff) string {
	var parts []string
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Kind == Added || l.Kind == Removed {
				parts = append(parts, string(l.Kind)+l.Text)
			}
		}
	}
	return strings.Join(parts, " ")
}

func contains(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}
