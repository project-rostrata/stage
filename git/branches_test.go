package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseBranches(t *testing.T) {
	out := strings.Join([]string{
		"*\x00main\x00\x001791900000\x00Harden stage",
		" \x00feature/x\x00behind 3\x001791800000\x00Add a thing",
		" \x00both\x00ahead 1, behind 2\x001791700000\x00",
		" \x00ahead\x00ahead 4\x001791600000\x00Subject with \x01 in it",
		" \x00old\x00gone\x001791500000\x00Old",
	}, "\n") + "\n"
	got, err := parseBranches(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []Branch{
		{Name: "main", Current: true, Time: time.Unix(1791900000, 0), Subject: "Harden stage"},
		{Name: "feature/x", Behind: 3, Time: time.Unix(1791800000, 0), Subject: "Add a thing"},
		{Name: "both", Behind: 2, Time: time.Unix(1791700000, 0)},
		{Name: "ahead", Time: time.Unix(1791600000, 0), Subject: "Subject with \x01 in it"},
		{Name: "old", Gone: true, Time: time.Unix(1791500000, 0), Subject: "Old"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseBranches:\n got %+v\nwant %+v", got, want)
	}
	if _, err := parseBranches("main\x00only two\n"); err == nil {
		t.Error("a short line should be an error")
	}
}

// commitAt makes an empty commit with a fixed committer time, so the order is certain.
func commitAt(t *testing.T, dir, msg string, unix int64) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-q", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+time.Unix(unix, 0).Format(time.RFC3339))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestBranchesAgainstGit(t *testing.T) {
	r, dir := repo(t)
	run(t, dir, "switch", "-q", "-c", "old")
	commitAt(t, dir, "Old work", 1791500000)
	run(t, dir, "switch", "-q", "-c", "feature")
	commitAt(t, dir, "Feature work", 1791600000)
	run(t, dir, "switch", "-q", "main")
	commitAt(t, dir, "Main work", 1791700000)
	// feature tracks main, which has a commit feature lacks; old tracks a
	// branch that no longer exists.
	run(t, dir, "config", "branch.feature.remote", ".")
	run(t, dir, "config", "branch.feature.merge", "refs/heads/main")
	run(t, dir, "config", "branch.old.remote", ".")
	run(t, dir, "config", "branch.old.merge", "refs/heads/deleted")

	got, err := r.Branches()
	if err != nil {
		t.Fatal(err)
	}
	want := []Branch{
		{Name: "main", Current: true, Time: time.Unix(1791700000, 0), Subject: "Main work"},
		{Name: "feature", Behind: 1, Time: time.Unix(1791600000, 0), Subject: "Feature work"},
		{Name: "old", Gone: true, Time: time.Unix(1791500000, 0), Subject: "Old work"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Branches:\n got %+v\nwant %+v", got, want)
	}
}

func TestSwitch(t *testing.T) {
	r, dir := repo(t)
	run(t, dir, "branch", "feature")
	if err := r.Switch("feature"); err != nil {
		t.Fatal(err)
	}
	if got := run(t, dir, "branch", "--show-current"); got != "feature\n" {
		t.Errorf("on %q after switching", got)
	}
	lines, _, err := r.AuditTrail(1)
	if err != nil || len(lines) != 1 || !strings.HasSuffix(lines[0], "ok      git -c core.fsmonitor=false --literal-pathspecs switch -- feature") {
		t.Errorf("trail: %q, %v", lines, err)
	}

	// git refuses a switch that would overwrite uncommitted work.
	run(t, dir, "switch", "-q", "-c", "other")
	write(t, dir, "a.txt", "on other\n")
	run(t, dir, "commit", "-q", "-am", "Change a on other")
	run(t, dir, "switch", "-q", "main")
	write(t, dir, "a.txt", "mine, uncommitted\n")
	if err := r.Switch("other"); err == nil {
		t.Error("switching over uncommitted work should fail")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "mine, uncommitted\n" {
		t.Errorf("uncommitted work changed: %q", b)
	}
}

// A .git from somewhere else can hold a branch named like an option. Without
// "--", git switch --detach would detach HEAD instead of switching to it.
func TestSwitchToABranchNamedLikeAnOption(t *testing.T) {
	r, dir := repo(t)
	head := strings.TrimSpace(run(t, dir, "rev-parse", "HEAD"))
	write(t, dir, ".git/refs/heads/--detach", head+"\n")
	if err := r.Switch("--detach"); err != nil {
		t.Fatal(err)
	}
	if got := run(t, dir, "branch", "--show-current"); got != "--detach\n" {
		t.Errorf("on %q, want the branch named --detach", got)
	}
}

func TestLastFetch(t *testing.T) {
	r, dir := repo(t)
	if got, err := r.LastFetch(); err != nil || !got.IsZero() {
		t.Errorf("before any fetch: %v, %v", got, err)
	}
	when := time.Unix(1791800000, 0)
	p := filepath.Join(dir, ".git", "FETCH_HEAD")
	write(t, dir, ".git/FETCH_HEAD", "")
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
	if got, err := r.LastFetch(); err != nil || !got.Equal(when) {
		t.Errorf("after a fetch: %v, %v", got, err)
	}
}
