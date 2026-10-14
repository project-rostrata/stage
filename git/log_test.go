package git

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	hashA = "1111111111111111111111111111111111111111"
	hashB = "2222222222222222222222222222222222222222"
	hashC = "3333333333333333333333333333333333333333"
)

func TestParseLog(t *testing.T) {
	out := strings.Join([]string{
		hashC + "\x00" + hashB + " " + hashA + "\x001791900000\x00Merge feature",
		hashB + "\x00" + hashA + "\x001791800000\x00Subject with \x01 in it",
		hashA + "\x00\x001791700000\x00",
	}, "\n") + "\n"
	got, err := parseLog(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []Commit{
		{Hash: hashC, Parent: hashB, Time: time.Unix(1791900000, 0), Subject: "Merge feature"},
		{Hash: hashB, Parent: hashA, Time: time.Unix(1791800000, 0), Subject: "Subject with \x01 in it"},
		{Hash: hashA, Time: time.Unix(1791700000, 0)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseLog:\n got %+v\nwant %+v", got, want)
	}
	if got[0].Short() != "3333333" {
		t.Errorf("Short: %q", got[0].Short())
	}

	for _, bad := range []string{
		hashA + "\x00only three\x001",
		"--output=x\x00\x001791700000\x00Not a hash",
		hashA + "\x00--output=x\x001791700000\x00Parent not a hash",
		hashA + "\x00\x00soon\x00Bad time",
	} {
		if _, err := parseLog(bad + "\n"); err == nil {
			t.Errorf("parseLog(%q) should be an error", bad)
		}
	}
}

func TestParseNameStatus(t *testing.T) {
	out := "M\x00a.txt\x00R087\x00old name\x00new\nname\x00A\x00dir/b.txt\x00D\x00gone.txt\x00"
	got, err := parseNameStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := []FileChange{
		{Code: 'M', Path: "a.txt"},
		{Code: 'R', Path: "new\nname", OrigPath: "old name"},
		{Code: 'A', Path: "dir/b.txt"},
		{Code: 'D', Path: "gone.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseNameStatus:\n got %+v\nwant %+v", got, want)
	}
	if got, err := parseNameStatus(nil); err != nil || got != nil {
		t.Errorf("no output: %+v, %v", got, err)
	}
	for _, bad := range []string{"M\x00", "R100\x00only one\x00"} {
		if _, err := parseNameStatus([]byte(bad)); err == nil {
			t.Errorf("parseNameStatus(%q) should be an error", bad)
		}
	}
}

func TestHistoryAgainstGit(t *testing.T) {
	r, dir := repo(t)
	// Settings that would change the shape of the output if stage didn't override them.
	run(t, dir, "config", "diff.noprefix", "true")
	run(t, dir, "config", "color.ui", "always")
	run(t, dir, "mv", "old.txt", "new.txt")
	write(t, dir, "a.txt", "one\nTWO\nthree\n")
	run(t, dir, "commit", "-q", "-am", "Rename and change")
	run(t, dir, "switch", "-q", "-c", "side")
	write(t, dir, "side.txt", "side\n")
	run(t, dir, "add", "side.txt")
	run(t, dir, "commit", "-q", "-m", "Side work")
	run(t, dir, "switch", "-q", "main")
	write(t, dir, "a.txt", "one\nTWO\nthree\nfour\n")
	run(t, dir, "commit", "-q", "-am", "Main work")
	run(t, dir, "merge", "-q", "--no-edit", "--no-ff", "side")

	commits, err := r.Log(10)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, c := range commits {
		subjects = append(subjects, c.Subject)
	}
	// Newest first; the order of the two branches' commits follows their times.
	if len(commits) != 5 || subjects[0] != "Merge branch 'side'" || subjects[4] != "first" {
		t.Fatalf("Log: %q", subjects)
	}
	if commits[4].Parent != "" || commits[0].Parent != strings.TrimSpace(run(t, dir, "rev-parse", "HEAD^1")) {
		t.Errorf("parents: %+v", commits)
	}
	if two, err := r.Log(2); err != nil || len(two) != 2 {
		t.Errorf("Log(2): %d commits, %v", len(two), err)
	}

	files := func(c Commit) []FileChange {
		t.Helper()
		f, err := r.CommitFiles(c)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	byHash := map[string]Commit{}
	for _, c := range commits {
		byHash[c.Subject] = c
	}
	// A merge shows what it brought into the branch, from its first parent.
	if got, want := files(byHash["Merge branch 'side'"]), []FileChange{{Code: 'A', Path: "side.txt"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("merge files: %+v", got)
	}
	if got, want := files(byHash["first"]), []FileChange{{Code: 'A', Path: "a.txt"}, {Code: 'A', Path: "old.txt"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("root files: %+v", got)
	}
	rename := byHash["Rename and change"]
	want := []FileChange{{Code: 'M', Path: "a.txt"}, {Code: 'R', Path: "new.txt", OrigPath: "old.txt"}}
	got := files(rename)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rename files: %+v", got)
	}

	d, err := r.CommitDiff(rename, got[0])
	if err != nil {
		t.Fatal(err)
	}
	if d.Header[0] != "diff --git a/a.txt b/a.txt" || len(d.Hunks) != 1 {
		t.Errorf("diff of a.txt: %+v", d)
	}
	var changed []string
	for _, l := range d.Hunks[0].Lines {
		if l.Kind != Context {
			changed = append(changed, string(l.Kind)+l.Text)
		}
	}
	if !reflect.DeepEqual(changed, []string{"-two", "+TWO"}) {
		t.Errorf("diff of a.txt changes: %q", changed)
	}
	if d, err := r.CommitDiff(rename, got[1]); err != nil || !strings.Contains(strings.Join(d.Header, "\n"), "rename from old.txt") {
		t.Errorf("diff of the rename: %+v, %v", d, err)
	}
	if d, err := r.CommitDiff(byHash["first"], FileChange{Code: 'A', Path: "a.txt"}); err != nil || len(d.Hunks) != 1 || d.Hunks[0].NewCount != 3 {
		t.Errorf("diff in the root commit: %+v, %v", d, err)
	}
}

func TestLogBeforeTheFirstCommit(t *testing.T) {
	_, dir := repo(t)
	run(t, dir, "switch", "-q", "--orphan", "empty")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.Log(10); err != nil || got != nil {
		t.Errorf("Log on an unborn branch: %+v, %v", got, err)
	}
}
