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

## Phase 4 — hunks and lines

- `patch/`: build the forward patch (stage) and the reverse form (unstage, revert) from a hunk and a set of selected lines (brief §4).
- `ui/`:
  - the mark and selection in the Diff pane;
  - `s`/Ctrl+S and Ctrl+U on a selection or on the hunk under the cursor;
  - hunk navigation (already done in phase 2);
  - the cursor stays near the same place after the diff reloads.

**Done when** round-trip tests against real temporary repositories pass for:
- stage, unstage and revert of single lines, mixed `+`/`-` selections, whole hunks, the first and last hunks, and a file without a trailing newline;
- the same flows in a tmux session, checked against `git diff` and `git diff --cached`.

## Phase 5 — commit

- The Commit pane's summary and description editor with the Emacs keys, paste, and line wrapping.
- The draft saved to `.git/STAGE_MSG` and restored at startup.
- Ctrl+C Ctrl+C commits through `git commit -F -` with the TUI suspended, shows git's output, keeps the message on failure, and clears it and the draft on success. Ctrl+C Ctrl+K clears the message.

**Done when:**
- editor tests pass;
- a commit made from stage runs a `commit-msg` hook that prompts on the terminal;
- `git log -1` shows the exact message typed.

## Phase 6 — polish and install

- `install.sh`, the README's usage section and key table, and an audit of the code against `AGENTS.md`.
- The human uses stage on a real repository (`shelf`) in GNOME Terminal, and the colours (brief §4) have been tuned by eye.

**Done when** the human has made real commits with it instead of git-cola.
