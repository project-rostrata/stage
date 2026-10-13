package patch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stage/git"
)

// These tests round-trip through real git: make a change in a temporary
// repository, build a patch from git's own diff, apply it with the same
// arguments stage uses, and check exactly what the index and the working
// tree hold afterwards.

type testRepo struct {
	t   *testing.T
	dir string
	r   *git.Repo
}

func newRepo(t *testing.T, files map[string]string) *testRepo {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	tr := &testRepo{t: t, dir: t.TempDir()}
	tr.git("init", "-q", "-b", "main")
	tr.git("config", "user.name", "Test")
	tr.git("config", "user.email", "test@example.com")
	for name, content := range files {
		tr.write(name, content)
	}
	tr.git("add", ".")
	tr.git("commit", "-q", "-m", "first")
	r, err := git.Open(tr.dir)
	if err != nil {
		t.Fatal(err)
	}
	tr.r = r
	return tr
}

func (tr *testRepo) git(args ...string) string {
	tr.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = tr.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		tr.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func (tr *testRepo) write(name, content string) {
	tr.t.Helper()
	path := filepath.Join(tr.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr *testRepo) worktree(name string) string {
	tr.t.Helper()
	b, err := os.ReadFile(filepath.Join(tr.dir, name))
	if err != nil {
		tr.t.Fatal(err)
	}
	return string(b)
}

func (tr *testRepo) index(name string) string {
	tr.t.Helper()
	return tr.git("show", ":"+name)
}

func (tr *testRepo) diff(e git.Entry) git.FileDiff {
	tr.t.Helper()
	d, err := tr.r.Diff(e)
	if err != nil {
		tr.t.Fatal(err)
	}
	return d
}

// pick selects the lines whose text (with its +/- column) is listed, in any hunk.
func pick(d git.FileDiff, lines ...string) Selection {
	sel := Selection{}
	for hi, h := range d.Hunks {
		for li, l := range h.Lines {
			for _, want := range lines {
				if string(l.Kind)+l.Text == want {
					if sel[hi] == nil {
						sel[hi] = map[int]bool{}
					}
					sel[hi][li] = true
				}
			}
		}
	}
	return sel
}

func (tr *testRepo) stage(path string, lines ...string) {
	tr.t.Helper()
	d := tr.diff(git.Entry{Section: git.Modified, Code: 'M', Path: path})
	p, err := Build(d, pick(d, lines...), Forward)
	if err != nil {
		tr.t.Fatal(err)
	}
	if err := tr.r.ApplyToIndex(p, false); err != nil {
		tr.t.Fatalf("%v\npatch:\n%s", err, p)
	}
}

func (tr *testRepo) unstage(e git.Entry, lines ...string) {
	tr.t.Helper()
	d := tr.diff(e)
	p, err := Build(d, pick(d, lines...), Reverse)
	if err != nil {
		tr.t.Fatal(err)
	}
	if err := tr.r.ApplyToIndex(p, true); err != nil {
		tr.t.Fatalf("%v\npatch:\n%s", err, p)
	}
}

func (tr *testRepo) revert(path string, lines ...string) {
	tr.t.Helper()
	d := tr.diff(git.Entry{Section: git.Modified, Code: 'M', Path: path})
	p, err := Build(d, pick(d, lines...), Reverse)
	if err != nil {
		tr.t.Fatal(err)
	}
	if err := tr.r.RevertPatch(p); err != nil {
		tr.t.Fatalf("%v\npatch:\n%s", err, p)
	}
}

func check(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

const ten = "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"

func TestStageOneAddedLine(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\n"})
	tr.write("f.txt", "a\nX\nb\nY\nc\n")
	tr.stage("f.txt", "+Y")
	check(t, "index", tr.index("f.txt"), "a\nb\nY\nc\n")
	check(t, "worktree", tr.worktree("f.txt"), "a\nX\nb\nY\nc\n")
}

func TestStageOneRemovedLine(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\nd\n"})
	tr.write("f.txt", "a\nd\n")
	tr.stage("f.txt", "-c")
	check(t, "index", tr.index("f.txt"), "a\nb\nd\n")
}

func TestStageOneReplacement(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "one\ntwo\nthree\nfour\n"})
	tr.write("f.txt", "ONE\none\nTWO\nthree\nfour\nFIVE\n")
	tr.stage("f.txt", "-two", "+TWO")
	check(t, "index", tr.index("f.txt"), "one\nTWO\nthree\nfour\n")
}

// Inside one block of removals followed by additions, an unselected removed
// line stays where it is, ahead of the block's added lines. That is how git
// add -p, git-gui and git-cola build these patches. A selection in stage is a
// contiguous range, as in git-cola, so this only shows when part of a block is
// staged.
func TestPartOfABlockGoesAfterItsRemainingLines(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "one\ntwo\nthree\n"})
	tr.write("f.txt", "ONE\nTWO\nTHREE\n")
	tr.stage("f.txt", "+ONE", "+TWO")
	check(t, "index", tr.index("f.txt"), "one\ntwo\nthree\nONE\nTWO\n")
}

func TestStageTheSecondHunkOnly(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": ten + ten})
	tr.write("f.txt", "1\nnew\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"+"1\n2\n3\n4\n5\n6\n7\n8\n9\n10\nend\n")
	tr.stage("f.txt", "+end")
	check(t, "index", tr.index("f.txt"), ten+ten+"end\n")
}

func TestStageAcrossTwoHunksPartly(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": ten + ten})
	tr.write("f.txt", "1\nA\nB\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"+"1\n2\n3\n4\n5\n6\n7\n8\n9\nC\nD\n")
	tr.stage("f.txt", "+B", "-10", "+C")
	check(t, "index", tr.index("f.txt"), "1\nB\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"+"1\n2\n3\n4\n5\n6\n7\n8\n9\nC\n")
	// What's left unstaged must still be exactly the rest.
	tr.stage("f.txt", "+A", "+D")
	check(t, "index after staging the rest", tr.index("f.txt"), tr.worktree("f.txt"))
}

func TestUnstageOneLine(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\n"})
	tr.write("f.txt", "a\nX\nb\nY\nc\n")
	tr.git("add", "f.txt")
	tr.unstage(git.Entry{Section: git.Staged, Code: 'M', Path: "f.txt"}, "+X")
	check(t, "index", tr.index("f.txt"), "a\nb\nY\nc\n")
	check(t, "worktree", tr.worktree("f.txt"), "a\nX\nb\nY\nc\n")
}

func TestUnstageOneRemoval(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\nd\n"})
	tr.write("f.txt", "a\nd\n")
	tr.git("add", "f.txt")
	tr.unstage(git.Entry{Section: git.Staged, Code: 'M', Path: "f.txt"}, "-b")
	check(t, "index", tr.index("f.txt"), "a\nb\nd\n")
}

func TestRevertOneLineKeepsTheRest(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\n"})
	tr.write("f.txt", "a\nX\nb\nc\nY\n")
	tr.revert("f.txt", "+X")
	check(t, "worktree", tr.worktree("f.txt"), "a\nb\nc\nY\n")
	check(t, "index", tr.index("f.txt"), "a\nb\nc\n")
}

func TestRevertOnlyTouchesUnstagedChanges(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\n"})
	tr.write("f.txt", "a\nS\nb\nc\n")
	tr.git("add", "f.txt")
	tr.write("f.txt", "a\nS\nb\nU\nc\n")
	tr.revert("f.txt", "+U")
	check(t, "worktree", tr.worktree("f.txt"), "a\nS\nb\nc\n")
	check(t, "index", tr.index("f.txt"), "a\nS\nb\nc\n")
}

func TestFileWithoutTrailingNewline(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nlast"})
	tr.write("f.txt", "a\nX\nb\nLAST")
	tr.stage("f.txt", "-last", "+LAST")
	check(t, "index", tr.index("f.txt"), "a\nb\nLAST")
	tr.stage("f.txt", "+X")
	check(t, "index", tr.index("f.txt"), "a\nX\nb\nLAST")
}

func TestLeaveTheMissingNewlineAlone(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nlast"})
	tr.write("f.txt", "a\nX\nb\nLAST")
	tr.stage("f.txt", "+X")
	check(t, "index", tr.index("f.txt"), "a\nX\nb\nlast")
}

func TestStagePartOfADeletion(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\nc\n"})
	if err := os.Remove(filepath.Join(tr.dir, "f.txt")); err != nil {
		t.Fatal(err)
	}
	d := tr.diff(git.Entry{Section: git.Modified, Code: 'D', Path: "f.txt"})
	p, err := Build(d, pick(d, "-b"), Forward)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.r.ApplyToIndex(p, false); err != nil {
		t.Fatalf("%v\n%s", err, p)
	}
	check(t, "index keeps the file, without b", tr.index("f.txt"), "a\nc\n")
}

func TestStageAllOfADeletion(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\n", "keep.txt": "k\n"})
	if err := os.Remove(filepath.Join(tr.dir, "f.txt")); err != nil {
		t.Fatal(err)
	}
	tr.stage("f.txt", "-a", "-b")
	check(t, "status", tr.git("status", "--short", "f.txt"), "D  f.txt\n")
}

func TestUnstagePartOfANewFile(t *testing.T) {
	tr := newRepo(t, map[string]string{"keep.txt": "k\n"})
	tr.write("new.txt", "one\ntwo\nthree\n")
	tr.git("add", "new.txt")
	tr.unstage(git.Entry{Section: git.Staged, Code: 'A', Path: "new.txt"}, "+two")
	check(t, "index keeps the new file, without two", tr.index("new.txt"), "one\nthree\n")
}

func TestUnstageALineOfARenamedFileKeepsTheRename(t *testing.T) {
	tr := newRepo(t, map[string]string{"old.txt": "1\n2\n3\n4\n5\n6\n7\n8\n"})
	tr.git("mv", "old.txt", "new.txt")
	tr.write("new.txt", "1\n2\n3\n4\n5\n6\n7\n8\nnine\n")
	tr.git("add", "new.txt")
	tr.unstage(git.Entry{Section: git.Staged, Code: 'R', Path: "new.txt", OrigPath: "old.txt"}, "+nine")
	check(t, "status", tr.git("status", "--short"), "RM old.txt -> new.txt\n")
	check(t, "index", tr.index("new.txt"), "1\n2\n3\n4\n5\n6\n7\n8\n")
}

func TestPathsWithSpacesAndUnicode(t *testing.T) {
	for _, name := range []string{"dir with space/f.txt", "日本/ファイル.txt", `quote"and\backslash.txt`} {
		t.Run(name, func(t *testing.T) {
			tr := newRepo(t, map[string]string{name: "a\nb\n"})
			tr.write(name, "a\nX\nb\nY\n")
			tr.stage(name, "+Y")
			check(t, "index", tr.index(name), "a\nb\nY\n")
		})
	}
}

// A directory name can hold a newline and what looks like a patch header. git
// quotes such paths in its diff, and the patch keeps the quoting, so staging a
// line can't land in another file.
func TestPathThatLooksLikeAPatchHeader(t *testing.T) {
	crafted := "x\n+++ b/victim.txt"
	name := crafted + "/f.txt"
	tr := newRepo(t, map[string]string{name: "a\nb\n", "victim.txt": "safe\n"})
	tr.write(name, "a\nb\nY\n")
	tr.stage(name, "+Y")
	check(t, "index", tr.index(name), "a\nb\nY\n")
	check(t, "victim", tr.index("victim.txt"), "safe\n")
}

func TestNothingSelected(t *testing.T) {
	tr := newRepo(t, map[string]string{"f.txt": "a\nb\n"})
	tr.write("f.txt", "a\nX\nb\n")
	d := tr.diff(git.Entry{Section: git.Modified, Code: 'M', Path: "f.txt"})
	if _, err := Build(d, pick(d, " a"), Forward); err != ErrNothingSelected {
		t.Errorf("selecting only context: %v", err)
	}
}

func TestPatchText(t *testing.T) {
	d := git.FileDiff{
		Header: []string{"diff --git a/f b/f", "index 1..2 100644", "--- a/f", "+++ b/f"},
		Hunks: []git.Hunk{{
			OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
			Lines: []git.Line{
				{Kind: git.Context, Text: "a"},
				{Kind: git.Removed, Text: "b"},
				{Kind: git.Added, Text: "B"},
				{Kind: git.Removed, Text: "c"},
				{Kind: git.Added, Text: "C"},
			},
		}},
	}
	got, err := Build(d, Selection{0: {1: true, 2: true}}, Forward)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"diff --git a/f b/f", "--- a/f", "+++ b/f",
		"@@ -1,3 +1,3 @@", " a", "-b", "+B", " c", "",
	}, "\n")
	check(t, "forward patch", got, want)

	got, err = Build(d, Selection{0: {1: true, 2: true}}, Reverse)
	if err != nil {
		t.Fatal(err)
	}
	want = strings.Join([]string{
		"diff --git a/f b/f", "--- a/f", "+++ b/f",
		"@@ -1,3 +1,3 @@", " a", "-b", "+B", " C", "",
	}, "\n")
	check(t, "reverse patch", got, want)
}
