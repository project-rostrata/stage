package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuoteWord(t *testing.T) {
	tests := []struct{ in, want string }{
		{"add", "add"},
		{"--literal-pathspecs", "--literal-pathspecs"},
		{"dir/file.go", "dir/file.go"},
		{"dir with space/f.txt", "'dir with space/f.txt'"},
		{"a*", "'a*'"},
		{"it's", `'it'\''s'`},
		{"日本.txt", "'日本.txt'"},
		{"new\nline", `$'new\nline'`},
		{"", "''"},
	}
	for _, tt := range tests {
		if got := quoteWord(tt.in); got != tt.want {
			t.Errorf("quoteWord(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestAuditTrailRecordsChangesOnly(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "a.txt", "changed\n")
	write(t, dir, "it's new.txt", "x\n")
	if _, err := r.Status(); err != nil { // reads are not recorded
		t.Fatal(err)
	}
	if _, err := r.Diff(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if lines, _, err := r.AuditTrail(10); err != nil || len(lines) != 0 {
		t.Fatalf("after only reads: %q, %v", lines, err)
	}

	if err := r.Stage(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Stage(Entry{Section: Untracked, Code: '?', Path: "it's new.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyToIndex("not a patch\n", false); err == nil {
		t.Fatal("a bad patch should fail")
	}
	if err := r.SaveDraft("Change a\n"); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := r.CommitDraft(strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}

	lines, path, err := r.AuditTrail(10)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, ".git", "stage.log") {
		t.Errorf("trail at %s", path)
	}
	want := []string{
		"ok      git -c core.fsmonitor=false --literal-pathspecs add -- a.txt",
		`ok      git -c core.fsmonitor=false --literal-pathspecs add -- 'it'\''s new.txt'`,
		"failed  git -c core.fsmonitor=false --literal-pathspecs apply --cached --recount --whitespace=nowarn -",
		"ok      git -c core.fsmonitor=false --literal-pathspecs commit --file .git/STAGE_MSG",
	}
	if len(lines) != len(want) {
		t.Fatalf("trail:\n%s", strings.Join(lines, "\n"))
	}
	for i, l := range lines {
		// Each line is "YYYY-MM-DD HH:MM:SS  " then the status and the command.
		if len(l) < 21 || l[21:] != want[i] {
			t.Errorf("line %d: %q, want ...%q", i, l, want[i])
		}
	}
}

func TestAuditTrailKeepsTheLastLines(t *testing.T) {
	r, dir := repo(t)
	p := filepath.Join(dir, ".git", "stage.log")
	if err := os.WriteFile(p, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, _, err := r.AuditTrail(2)
	if err != nil || strings.Join(lines, ",") != "two,three" {
		t.Errorf("last two lines: %q, %v", lines, err)
	}
}
