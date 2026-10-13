# V1_PLAN.md — build order

The phases for stage v1, in order. Each phase lands as one or more commits on `main`. When a phase is done, its entry here is rewritten to say **DONE**, plus anything that changed from the plan. The reasoning behind those changes goes in `docs/decisions.md`.

`PROJECT_BRIEF.md` is the target, and `AGENTS.md` has the conventions.

## Phase 0 — planning docs — DONE

- `git init -b main`, plus `LICENSE` (GPL-3.0-or-later), `README.md`, `PROJECT_BRIEF.md`, `AGENTS.md`, this file, `.gitignore`.
- `docs/decisions.md`, seeded with the planning conversation's decisions.

**Done when** the human has reviewed the brief. Reviewed 2026-10-12; the human's answers to the open questions are in `docs/decisions.md`.

## Phase 1 — toolchain and terminal layer — DONE (GNOME Terminal check pending)

- `toolchain.sha256` (go1.27.2), `scripts/fetch-go.sh`, `build.sh`, `test.sh`, and `go.mod` (`module stage`, `go 1.27`, no `require`). `test.sh` runs the `gofmt` check, `go vet` and `go test`.
- `term/`:
  - raw mode with cfmakeraw's flags;
  - `VMIN=0, VTIME=1` reads, so the event loop is one goroutine (`docs/decisions.md`);
  - window size, `SIGWINCH`, and SIGTERM/SIGHUP/SIGINT;
  - the alternate screen with auto-wrap off, focus reporting and bracketed paste;
  - suspend and resume;
  - an idempotent `Stop` for every exit path.
- `screen/`: the cell grid (wide characters span two cells, and combining marks join their base), the frame diff, 24-bit colour, and the East Asian Width table.
- `input/`: the key decoder, including Alt as an ESC prefix, xterm modifier parameters, focus events, paste blocks (line breaks normalised to `\n`), and the Esc timeout.
- `ui/`: the palette (brief §4), the layout, borders, and focus with Ctrl+L/K/J, Tab/Shift+Tab, and Esc (from Commit).
- A one-row status bar sits under the panes (`docs/decisions.md`). For now it shows the name of the last key pressed, so the keys GNOME Terminal sends can be checked. Phase 2 replaces that with key hints.

**Checked:**
- `test.sh` passes: decoder tests from VTE's byte sequences, and grid and frame-diff tests.
- In tmux, the panes draw with the focused one in the accent colour, and redraw cleanly on resize, including the "too small" message under 50x12.
- Every key in the brief's table decodes. A paste arrives as one event.
- Ctrl+Q, SIGTERM and a deliberate panic (a throwaway build) each leave the shell in cooked mode on the normal screen. The panic prints its stack trace and exits 2.

**Still to do:** the human runs `./build.sh && ./stage` in GNOME Terminal, checks the keys and the colours, and confirms that a focus change is reported.

## Phase 2 — read-only viewer — DONE (GNOME Terminal check pending)

- `git/`:
  - `Open` finds the top level.
  - Read-only commands run with `GIT_OPTIONAL_LOCKS=0`.
  - `Status` parses `status --porcelain=v2 -z --branch --untracked-files=all`, including renames, unmerged paths, and paths with spaces or newlines.
  - `Diff` gives the staged, unstaged or untracked (`--no-index`) diff with flags that override the human's diff config (`docs/decisions.md`).
  - `parseDiff` reads hunks with old and new line numbers, binary files, the no-newline marker, and combined diffs for unmerged paths.
- `ui/`:
  - the Status pane, grouped under headings, with a coloured status letter per entry and long paths shortened from the left;
  - the Diff pane with old and new line numbers (brief §4, changed in `docs/decisions.md`), hunk headers, the added and removed backgrounds, tabs expanded, long lines wrapped, and the mode, rename and binary lines from the header;
  - the path and "(staged)" in the Diff pane's title;
  - a `▶` cursor in the Diff pane;
  - the movement keys (`j`/`k`, Ctrl+N/P, arrows, `d`/`u`, Ctrl+V/Alt+V, PgUp/PgDn, `gg`/`G`, Alt+`<`/Alt+`>`, Home/End);
  - `n`/`p` between hunks;
  - the branch and counts, or the last error, in the status bar.
- Pulled forward from phases 3 and 4: refresh on focus-in, Ctrl+R and F5, keeping the selection and the diff's cursor; and `n`/`p`.

**Checked:**
- Parser tests pass, including against real temporary repositories and a repository whose config sets `diff.noprefix`, `diff.mnemonicPrefix`, `color.ui=always` and `diff.external`.
- Text helper and layout tests pass.
- In tmux, against a scratch repository, stage shows each of these the way git-cola does:
  - a partly staged file, under both Staged and Modified;
  - a staged rename;
  - a two-hunk file;
  - a binary file;
  - an untracked file in a directory with a space in its name;
  - tabs, Japanese text and a wrapped long line;
  - the end of a 200-line file;
  - a conflicted file (its combined diff, read-only).
- An outside change appears on focus-in. Starting outside a repository prints git's error and exits 1.

**Still to do:** the human checks it in GNOME Terminal on a real repository, and tunes the colours by eye.

## Phase 3 — whole-file actions — DONE

- `git/`:
  - `Stage` (`git add`, which also stages a deletion);
  - `Unstage` (`git restore --staged`, both paths of a rename, and `git rm --cached` before the first commit);
  - `RevertFile` (`git restore --worktree`, which keeps staged changes);
  - `DeleteUntracked` (`git clean --force`, which can't touch a tracked file).
- Every git command runs with `GIT_LITERAL_PATHSPECS=1` (`docs/decisions.md`).
- `ui/`:
  - Ctrl+S stages or unstages the selected file in Status.
  - Ctrl+U asks before reverting a Modified file's unstaged changes.
  - Delete asks before deleting an untracked file.
  - The `y/N` question goes in the status bar, and anything but `y` cancels.
  - The result or git's error stays in the bar until the next key.
  - The bar's hints follow the focused pane.
- After an action the selection stays in the same section, on the next file there (`docs/decisions.md`).
- Refresh on focus, Ctrl+R and F5 landed in phase 2.

**Checked:**
- Action tests against real repositories:
  - staging a modification, a deletion and an untracked file, and unstaging each;
  - unstaging a rename, and unstaging before the first commit;
  - reverting while keeping staged changes;
  - deleting an untracked file, while refusing a tracked one;
  - a file named `a*`. The test fails if `GIT_LITERAL_PATHSPECS` is removed: `git clean` then deletes `ab.txt` too.
- Selection tests.
- In tmux:
  - Delete, then `n`, cancels.
  - Delete, then `y`, deletes.
  - Ctrl+S on an untracked file stages it and selects the next untracked file.
  - Ctrl+U, then `y`, reverts.
  - Ctrl+S on a staged rename unstages both paths.
  - A stale `index.lock` shows git's own error in the bar.

## Phase 4 — hunks and lines — DONE

- `patch/`: `Build` makes a Forward patch (stage) or a Reverse one (unstage, revert) from a selection that may span hunks.
  - It recomputes the hunk starts, so a hunk that was skipped or only partly included before doesn't throw the next one off.
  - It keeps a "no newline" marker only with the line it belongs to.
  - Its header always names the one new path. A rename is never undone by a line patch. A file's creation or deletion is kept only when every change is included.
- `git/`: `ApplyToIndex` (`git apply --cached [-R] --recount --whitespace=nowarn`) and `RevertPatch` (`git apply -R` on the working tree).
- `ui/`:
  - Ctrl+Space sets the mark, and the selection runs from the mark to the cursor.
  - Shift+J/K and Shift+arrows extend the selection.
  - Ctrl+G or Esc clears it.
  - Selected change lines get the brighter backgrounds and a `▌` in the gutter.
  - `s`/Ctrl+S stages the selection, or unstages it from a staged diff. With no mark, they act on the hunk under the cursor. Untracked and binary files go whole.
  - Ctrl+U asks, then reverts the selection or hunk from an unstaged diff.
  - The diff's cursor stays put after the reload.

**Checked:**
- Round-trip tests through real `git apply`:
  - one added line, and one removed line;
  - a one-line replacement among unchanged lines;
  - only the second hunk, and a selection across two hunks (with the rest staged afterwards and checked exactly);
  - unstaging a line and a removal;
  - reverting a line, and reverting without touching staged changes;
  - files with no final newline, both changing and leaving the last line;
  - part of a deletion, and all of one;
  - part of a new file;
  - a line of a renamed file, which stays renamed;
  - paths with a space, Japanese characters, and a quote and backslash;
  - nothing selected.
- The exact patch text, for both directions.
- In tmux: `s` on a hunk; a two-line selection with Ctrl+Space and Shift+J; unstaging a selected line from the staged diff; and a revert from the Diff pane, cancelled with Esc and then confirmed. Each was checked against `git diff --cached` and `git diff`.

## Phase 5 — commit — DONE

- `ui/editor.go`:
  - the summary and a multi-line description;
  - Emacs movement, kill and yank;
  - Enter in the summary moves to the description;
  - a pasted message splits into its summary and description;
  - the cursor starts at the end of the summary.
- `ui/commit_pane.go`: git-cola's placeholders, the summary scrolling sideways, a rule, and the description wrapped and scrolled to keep the cursor in view. The terminal cursor shows there while the pane is focused.
- `git/commit.go`:
  - the draft in `.git/STAGE_MSG`, found with `git rev-parse --git-path` once and cached;
  - `CommitDraft` runs `git commit --file <draft>` on the terminal.
- `ui/app.go`:
  - The draft is saved once per batch of keys.
  - Ctrl+C Ctrl+C commits from any pane.
  - Ctrl+C Ctrl+K clears the message, and Ctrl+Y brings it back.
  - The status is reloaded before committing. An empty summary or nothing staged is caught before the terminal is handed over.
  - On a failed commit, stage waits for Enter.
- `term/`: `Input`, `WaitForEnter`, and SIGINT kept apart from quitting, so Ctrl+C at a hook's prompt only stops git.

**Checked:**
- Editor tests:
  - a message's round trip;
  - typing;
  - Enter in the summary;
  - joining lines;
  - kill and yank;
  - word movement and word kills;
  - pasting part of a message, and a whole message;
  - moving across lines.
- Draft and commit tests against a real repository, including a commit with nothing staged.
- In tmux:
  - a typed message is saved and comes back after quitting;
  - a 100-character description appears within 0.15 s (it first took about 0.5 s, because every key started a `git rev-parse`);
  - Ctrl+C Ctrl+C commits, and `git log -1` shows the exact message;
  - a rejecting `commit-msg` hook shows its output, waits for Enter and keeps the message;
  - a hook that prompts on `/dev/tty` gets its answer;
  - Ctrl+C at that prompt aborts the commit and stage carries on.

## Phase 6 — polish and install

- `install.sh`, the README's usage section and key table, and an audit of the code against `AGENTS.md`.
- The human uses stage on a real repository (`shelf`) in GNOME Terminal, and the colours (brief §4) have been tuned by eye.

**Done when** the human has made real commits with it instead of git-cola.
