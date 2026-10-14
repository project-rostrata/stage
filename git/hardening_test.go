package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests are the threat model in PROJECT_BRIEF.md, one attack each: a
// repository stage is started in, or a file name in it, must not make stage
// run a program, write outside the repository or read a file it shouldn't.

func exists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Lstat(p)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestRepositoryConfigCantRunAProgram(t *testing.T) {
	r, dir := repo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	run(t, dir, "config", "core.fsmonitor", "touch '"+marker+"'; false")
	write(t, dir, "a.txt", "one\nTWO\nthree\n")
	write(t, dir, "new.txt", "new\n")

	if _, err := r.Status(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Diff(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Stage(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Unstage(Entry{Section: Staged, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RevertFile(Entry{Section: Modified, Code: 'M', Path: "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Stage(Entry{Section: Untracked, Code: '?', Path: "new.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveDraft("Add new\n"); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := r.CommitDraft(strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if exists(t, marker) {
		t.Fatal("a command stage ran started core.fsmonitor")
	}

	// The trap is real: plain git status springs it.
	write(t, dir, "a.txt", "changed\n")
	run(t, dir, "status")
	if !exists(t, marker) {
		t.Fatal("core.fsmonitor didn't run under plain git, so this test proves nothing")
	}
}

func TestDraftIsNeverReadOrWrittenThroughALink(t *testing.T) {
	r, dir := repo(t)
	secret := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, ".git", "STAGE_MSG")); err != nil {
		t.Fatal(err)
	}

	if msg, err := r.LoadDraft(); err == nil || msg != "" {
		t.Errorf("LoadDraft through a link = %q, %v", msg, err)
	}
	if err := r.SaveDraft("Overwrite\n"); err == nil {
		t.Error("SaveDraft wrote through a link")
	}
	write(t, dir, "a.txt", "changed\n")
	run(t, dir, "add", "a.txt")
	var out strings.Builder
	if err := r.CommitDraft(strings.NewReader(""), &out, &out); err == nil {
		t.Errorf("CommitDraft committed from a link: %s", out.String())
	}
	if got := run(t, dir, "log", "-1", "--format=%s"); got != "first\n" {
		t.Errorf("a commit was made: %q", got)
	}
	if b, err := os.ReadFile(secret); err != nil || string(b) != "PRIVATE KEY\n" {
		t.Errorf("the link's target changed: %q, %v", b, err)
	}
}

func TestAuditTrailIsNeverWrittenThroughALink(t *testing.T) {
	r, dir := repo(t)
	target := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(target, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, ".git", "stage.log")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "a.txt", "changed\n")
	err := r.Stage(Entry{Section: Modified, Code: 'M', Path: "a.txt"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("staging with a linked trail: %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "# mine\n" {
		t.Errorf("the link's target changed: %q", b)
	}
	if _, _, err := r.AuditTrail(10); err == nil {
		t.Error("AuditTrail read through a link")
	}
}

// Every path follows "--", so a file named like an option is only a file:
// without it, staging "--all" would stage everything.
func TestPathsThatLookLikeOptions(t *testing.T) {
	r, dir := repo(t)
	write(t, dir, "--all", "x\n")
	write(t, dir, "-n", "y\n")
	write(t, dir, "other.txt", "z\n")
	if err := r.Stage(Entry{Section: Untracked, Code: '?', Path: "--all"}); err != nil {
		t.Fatal(err)
	}
	if got, want := porcelain(t, dir), "A  --all\n?? -n\n?? other.txt"; got != want {
		t.Errorf("after staging --all:\n%s\nwant\n%s", got, want)
	}
	if err := r.Unstage(Entry{Section: Staged, Code: 'A', Path: "--all"}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteUntracked(Entry{Section: Untracked, Code: '?', Path: "-n"}); err != nil {
		t.Fatal(err)
	}
	if got, want := porcelain(t, dir), "?? --all\n?? other.txt"; got != want {
		t.Errorf("after deleting -n:\n%s\nwant\n%s", got, want)
	}
}

// Go won't run a program found through "." in PATH, so a git executable
// planted in the repository is never run in place of the real one.
func TestAGitInTheRepositoryIsNotRun(t *testing.T) {
	_, dir := repo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	write(t, dir, "git", "#!/bin/sh\ntouch '"+marker+"'\n")
	if err := os.Chmod(filepath.Join(dir, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", ".:"+os.Getenv("PATH"))
	if _, err := Open("."); err == nil {
		t.Error("Open ran with ./git first in PATH")
	}
	if exists(t, marker) {
		t.Error("the planted git ran")
	}
}

// A repository's config can ask git log to check every signature it shows,
// with a program it names. A signed commit is only a header in the commit
// object, so a .git from somewhere else can hold one.
func TestHistoryDoesntCheckSignatures(t *testing.T) {
	r, dir := repo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	write(t, dir, ".git/fake-gpg", "#!/bin/sh\ntouch '"+marker+"'\n")
	if err := os.Chmod(filepath.Join(dir, ".git", "fake-gpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "config", "gpg.program", filepath.Join(dir, ".git", "fake-gpg"))
	run(t, dir, "config", "log.showSignature", "true")
	head := strings.TrimSpace(run(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(run(t, dir, "rev-parse", "HEAD^{tree}"))
	signed := "tree " + tree + "\nparent " + head + "\n" +
		"author Test <test@example.com> 1791700000 +0000\ncommitter Test <test@example.com> 1791700000 +0000\n" +
		"gpgsig -----BEGIN PGP SIGNATURE-----\n \n AAAA\n -----END PGP SIGNATURE-----\n\nSigned\n"
	write(t, dir, ".git/signed", signed)
	commit := strings.TrimSpace(run(t, dir, "hash-object", "-t", "commit", "-w", ".git/signed"))
	run(t, dir, "update-ref", "refs/heads/main", commit)

	commits, err := r.Log(10)
	if err != nil || len(commits) != 2 || commits[0].Subject != "Signed" {
		t.Fatalf("Log: %+v, %v", commits, err)
	}
	if _, err := r.CommitFiles(commits[0]); err != nil {
		t.Fatal(err)
	}
	if exists(t, marker) {
		t.Fatal("reading the history ran gpg.program")
	}

	// The trap is real: plain git log springs it.
	run(t, dir, "log", "-1")
	if !exists(t, marker) {
		t.Fatal("gpg.program didn't run under plain git log, so this test proves nothing")
	}
}
