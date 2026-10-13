package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// porcelain is `git status --short` with its lines sorted as git prints them, for comparing.
func porcelain(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimRight(run(t, dir, "status", "--short", "--untracked-files=all"), "\n")
}

func TestStageAndUnstage(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "a.txt", "one\nTWO\nthree\n")
	write(t, dir, "untracked.txt", "new\n")
	if err := os.Remove(filepath.Join(dir, "old.txt")); err != nil {
		t.Fatal(err)
	}

	for _, e := range []Entry{
		{Section: Modified, Code: 'M', Path: "a.txt"},
		{Section: Modified, Code: 'D', Path: "old.txt"},
		{Section: Untracked, Code: '?', Path: "untracked.txt"},
	} {
		if err := r.Stage(e); err != nil {
			t.Fatalf("Stage(%s): %v", e.Path, err)
		}
	}
	if got, want := porcelain(t, dir), "M  a.txt\nD  old.txt\nA  untracked.txt"; got != want {
		t.Errorf("after staging:\n%s\nwant\n%s", got, want)
	}

	for _, e := range []Entry{
		{Section: Staged, Code: 'M', Path: "a.txt"},
		{Section: Staged, Code: 'D', Path: "old.txt"},
		{Section: Staged, Code: 'A', Path: "untracked.txt"},
	} {
		if err := r.Unstage(e); err != nil {
			t.Fatalf("Unstage(%s): %v", e.Path, err)
		}
	}
	if got, want := porcelain(t, dir), " M a.txt\n D old.txt\n?? untracked.txt"; got != want {
		t.Errorf("after unstaging:\n%s\nwant\n%s", got, want)
	}
}

func TestUnstageRenameRestoresBothPaths(t *testing.T) {
	r, dir := repo(t)
	run(t, dir, "mv", "old.txt", "new.txt")
	if err := r.Unstage(Entry{Section: Staged, Code: 'R', Path: "new.txt", OrigPath: "old.txt"}); err != nil {
		t.Fatal(err)
	}
	if got, want := porcelain(t, dir), " D old.txt\n?? new.txt"; got != want {
		t.Errorf("after unstaging a rename:\n%s\nwant\n%s", got, want)
	}
}

func TestUnstageBeforeTheFirstCommit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "first.txt", "hello\n")
	run(t, dir, "add", "first.txt")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Unstage(Entry{Section: Staged, Code: 'A', Path: "first.txt"}); err != nil {
		t.Fatal(err)
	}
	if got := porcelain(t, dir); got != "?? first.txt" {
		t.Errorf("after unstaging with no commits: %q", got)
	}
}

func TestRevertFileKeepsStagedChanges(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "a.txt", "one\nTWO\nthree\n")
	run(t, dir, "add", "a.txt")
	write(t, dir, "a.txt", "one\nTWO\nthree\nfour\n")
	if err := r.RevertFile(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\nTWO\nthree\n" || porcelain(t, dir) != "M  a.txt" {
		t.Errorf("after revert: %q, status %q", got, porcelain(t, dir))
	}
}

func TestDeleteUntracked(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "junk/tmp.txt", "x\n")
	if err := r.DeleteUntracked(Entry{Section: Untracked, Code: '?', Path: "junk/tmp.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "junk/tmp.txt")); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
	// git clean never deletes a tracked file, even if asked to by mistake.
	if err := r.DeleteUntracked(Entry{Section: Untracked, Code: '?', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Errorf("tracked file was deleted: %v", err)
	}
}

// Without GIT_LITERAL_PATHSPECS, git clean treats "a*" as a pattern and deletes ab.txt too.
func TestPathsAreLiteral(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "a*", "star\n")
	write(t, dir, "ab.txt", "other\n")
	if err := r.Stage(Entry{Section: Untracked, Code: '?', Path: "a*"}); err != nil {
		t.Fatal(err)
	}
	if got := porcelain(t, dir); got != "A  a*\n?? ab.txt" {
		t.Errorf("staging a file named a* staged more than it:\n%s", got)
	}
	if err := r.DeleteUntracked(Entry{Section: Untracked, Code: '?', Path: "a*"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ab.txt")); err != nil {
		t.Errorf("deleting a* deleted ab.txt too: %v", err)
	}
}

func TestActionsRefuseTheWrongSection(t *testing.T) {
	r, _ := repo(t)
	if err := r.Stage(Entry{Section: Unmerged, Code: 'U', Path: "a.txt"}); err == nil {
		t.Error("staging an unmerged path should be refused")
	}
	if err := r.RevertFile(Entry{Section: Untracked, Code: '?', Path: "a.txt"}); err == nil {
		t.Error("reverting an untracked file should be refused")
	}
	if err := r.DeleteUntracked(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err == nil {
		t.Error("deleting a tracked file should be refused")
	}
}
