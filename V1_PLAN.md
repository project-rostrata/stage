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

## Phase 2 — read-only viewer

- `git/`: find the repository root, and parse `status --porcelain=v2 -z` into staged, modified, untracked and unmerged entries, including renames and paths with spaces or newlines.
- `git/`: parse unified diffs into files, hunks and lines. Untracked files get the `--no-index` diff, and binary files are recognised.
- `ui/`:
  - the Status pane, grouped and navigable;
  - the Diff pane for the selected entry (line numbers, hunk headers, colours, tab expansion, wrapping);
  - focus keys and movement keys (brief §4).

**Done when:** parser tests pass, and stage shows a real repository's changes the way git-cola does, including an untracked file, a rename, a binary file, and a file with a tab and a wide character.

## Phase 3 — whole-file actions and refresh

- Stage and unstage a whole file from Status (`git add` / `git restore --staged`, or `git rm --cached` before the first commit).
- Revert a file's unstaged edits (`git restore`), and delete an untracked file, each behind the `y/N` prompt.
- Refresh on focus-in, Ctrl+R and F5, keeping the selection and scroll where possible.
- Errors from git are shown, never swallowed.

**Done when:** each action matches what the same command does at the prompt, checked in a tmux session against a scratch repository, and a change made in another terminal appears on focus.

## Phase 4 — hunks and lines

- `patch/`: build the forward patch (stage) and the reverse form (unstage, revert) from a hunk and a set of selected lines (brief §4).
- `ui/`:
  - the mark and selection in the Diff pane;
  - `s`/Ctrl+S and Ctrl+U on a selection or on the hunk under the cursor;
  - hunk navigation;
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
