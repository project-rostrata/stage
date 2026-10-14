# AGENTS.md — conventions for anyone (human or AI) writing code in this repo

This file covers coding conventions and patterns, the "how" of writing stage's code. The other planning docs each have their own job:
- `PROJECT_BRIEF.md` is the "what": the target design and non-negotiable constraints.
- `V1_PLAN.md` is the "in what order".
- `docs/decisions.md` is the append-only log of *why* each convention below was chosen over the alternatives.

If anything here ever conflicts with the brief, the brief wins. Flag the conflict rather than silently picking one.

stage is the family's first Go project and first CLI. The rules below carry the siblings' Kotlin conventions (small single-purpose types, no DI framework, expected outcomes as values, near-zero dependencies) over into Go idiom. The human is new to Go, so code that reads plainly beats code that is clever.

## Language & style (Go)

- `gofmt` formatting and a clean `go vet ./...` are required. Nothing else lints.
- Plain Go: structs, functions, small interfaces declared by the consumer. Avoid generics unless they remove real duplication, and avoid reflection, `unsafe` (outside `term/`), goroutine tricks and init-time side effects.
- Small, single-purpose types and files. If describing a type's responsibility needs the word "and", split it.
- Minimal exported surface: unexported by default, exported only for what another package actually calls.
- Value types for value objects (`git.FileStatus`, `patch.Hunk`, `input.Key`). Prefer immutability: build a new value rather than mutating one shared by others.
- Comments say *why*, not *what*. Every exported identifier gets a doc comment.

## Packages

One module, `stage`, laid out as in the brief §3. Dependencies point one way:

```
main -> ui -> (screen, input, patch, git)
        term (used by main and ui only)
```

- `patch/` is pure: text in, text out, no I/O. That keeps the risky logic (what gets staged or reverted) fully unit-testable.
- `git/` is the only package that runs processes. Nothing else calls `os/exec`.
- `term/` is the only package that touches termios or writes raw escape sequences outside `screen/`'s frame output.
- `screen/` knows nothing about git. `ui/` knows nothing about termios.

## Composition root

**No globals.** No package-level `var` holding state, and no `init()` with side effects. `main.go` builds every component once, in order, and passes each to what needs it via constructor arguments (`ui.New(term, git, ...)`). Package-level `const` and immutable lookup tables (the width table) are fine.

## Error handling

- Expected outcomes are values: functions return `(T, error)`, and the UI turns an error into a one-line message in the status bar or, for a failed git command, shows git's own stderr. Wrap errors with context (`fmt.Errorf("stage %s: %w", path, err)`).
- `panic` is only for bugs: a broken invariant, never a git failure, bad input or a missing file.
- `main` recovers any panic once, restores the terminal first, and then prints the panic and stack trace. **The terminal must never be left in raw mode or on the alternate screen**, whether stage exits normally, by error, or by panic.

## Running git

- Always run git with an argument list (`exec.Command("git", args...)`), never through a shell, and put `--` before paths.
- Paths come from `-z` output and are passed back byte-for-byte. Never re-quote or "clean" them.
- Every git call runs in the repository root (`git rev-parse --show-toplevel`, found once at startup).
- Patches go to `git apply` on stdin, never through a temp file in the repo.
- Every command runs as `git -c core.fsmonitor=false --literal-pathspecs …` (`guarded` in `git/repo.go`), including `git commit`. The first stops a repository's config from running a program on every refresh. The second means a path never acts as a pattern. They're flags rather than environment variables so the audit trail shows them.
- Files stage keeps in `.git` (the draft and the audit trail) are opened with `openFile`, which never follows a symbolic link.
- Every command that changes something goes through `Repo.write` (or `CommitDraft`), which appends it to the audit trail, `.git/stage.log`. Don't run a write command any other way. A command that worked but couldn't be logged is reported as an error.
- Set `GIT_OPTIONAL_LOCKS=0` for read-only commands (`status`, `diff`, `log`), so a background refresh never contends with the human's own git commands in another terminal.
- Read-only commands may run on every refresh. Write commands (`add`, `restore`, `apply`, `commit`, `rm`, `clean`, `switch`) run only from an explicit keypress, and a destructive one only after its `y/N` prompt.

## Security review

The brief's threat model (§4, "Security") lists what stage trusts. Check every change against it:
- git is run with an argument list, never a shell, through `guarded`.
- Paths come after `--`.
- `diff` and `diff-tree` keep `--no-ext-diff --no-textconv`, and `log` keeps `--no-show-signature`.
- Nothing reaches the terminal except through `screen/`, apart from the suspended `git commit`.
- No file is written or read through a symbolic link that a repository could plant.
- stage still runs no network command and never reads credentials.

A new defence gets a test in `git/hardening_test.go` (or next to the code it protects) that fails without it. Check that by removing the defence once.

## Terminal rules

- Everything drawn goes through `screen/`'s cell grid. Nothing in `ui/` writes to stdout directly.
- Never assume a character is one column wide. Use `screen/`'s width function, and expand tabs before measuring.
- Colours are 24-bit (`CSI 38;2;r;g;b m`), taken from one palette in `ui/` (brief §4). Visibility on the human's dark terminal comes before the family's eel-green branding. Never colour something without checking it reads on dark.
- Escape sequences are written as named constants in `term/` or `screen/`, never inline in UI code.

## Testing

- The standard `testing` package only, no assertion library. Table-driven tests where there are several cases.
- `go test ./...` must pass with no network access and no terminal attached.
- `patch/` tests round-trip through real git: create a temporary repository (`t.TempDir()`), write a change, build a partial patch, apply it with the same `git apply` arguments stage uses, and check `git diff`/`git diff --cached` afterwards. Mocks of git's behaviour don't count.
- `input/` tests feed raw byte sequences (as VTE sends them) and check the decoded keys. `screen/` tests check that a frame diff writes only the changed cells.
- End-to-end checks run stage inside `tmux` against a scratch repository, driven with `tmux send-keys` and checked with `tmux capture-pane`. They are a dev-machine procedure, described in `V1_PLAN.md`; the repo doesn't depend on tmux.

## Scripts

Short, readable bash, `set -euo pipefail`, no logic that belongs in Go:
- `scripts/fetch-go.sh`: download the pinned toolchain into `.toolchain/` and verify it against `toolchain.sha256` before unpacking.
- `build.sh`: `CGO_ENABLED=0 go build -trimpath -o stage .` with the fetched toolchain.
- `test.sh`: `go vet ./...` then `go test ./...`.
- `install.sh`: symlink the built binary into `~/.local/bin/stage`.

## Dependency policy

The standard library only. Any third-party module, `golang.org/x/*` included, needs the human's explicit sign-off before it's added. The Go toolchain itself is pinned in `toolchain.sha256` and fetched by script, never committed. Neither is the built `stage` binary.
