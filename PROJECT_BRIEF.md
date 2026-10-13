# stage — a terminal git client for reviewing, staging and committing — project brief

This is a handoff document from a planning conversation, written the same way the siblings' (`shelf`, `larder`, `press`, `proof`, `list`) were. It records the decisions already made, why they were made, and what's still open.
- Treat section 2 as hard constraints.
- Treat section 4 as decisions not to relitigate without flagging it to the human first.
- Treat section 5 as things to check with the human before building.
- Everything else is a reasonable default that can be refined.

## 1. What this is

A full-screen terminal program for the one part of the git workflow the human still leaves the terminal for: reviewing changes, staging them (whole files, hunks, or individual lines), throwing away the unwanted ones, and committing. For years that has meant switching to git-cola. stage brings git-cola's three panes into the terminal, laid out the way the human has git-cola configured:

```
┌ Commit ──────────────┐┌ Diff ───────────────────────────────────────────┐
│ Commit summary       ││    @@ -0,0 +1,61 @@                             │
│ Extended description ││ 01 +# Builds the Android debug APK in a ...      │
│                      ││ 02 +# Mirrors android-build.yml's CI job ...     │
└──────────────────────┘│ 03 +# and reuses the same checksummed-down...    │
┌ Status ──────────────┐│ ...                                             │
│ Untracked            ││                                                 │
│   android/.docker... ││                                                 │
│ > android/Dockerfile ││                                                 │
│   android/build-wi...││                                                 │
└──────────────────────┘└─────────────────────────────────────────────────┘
```

It is the sixth sibling project in this workspace, and the first that isn't a web app. The family's ethos still applies in full: **only what is necessary.** A feature goes in because the human's workflow needs it, never because git-cola, tig, lazygit or magit has it. Branches, stashes, merges, rebases, pushes and pulls happen in the terminal with plain `git`, and stay there.

The user is the human alone, on Ubuntu, in GNOME Terminal. There is no packaging for anyone else.

## 2. Non-negotiable constraints

- **Language: Go, standard library only.** `go.mod` has no `require` lines. Any third-party module, including `golang.org/x/*`, needs the human's approval first. Ask before adding it, don't just add it.
- **One static binary.** It is built with `CGO_ENABLED=0 go build`. No cgo, no C compiler, and no shared libraries at runtime beyond what the kernel provides.
- **No ncurses and no TUI library.** The terminal layer (raw mode, the screen buffer, key decoding) is hand-written on `syscall`, `os/signal` and escape sequences (section 4).
- **Git is driven through the `git` command, never a library** (not libgit2, not go-git). That way the human's hooks, signing, config and credential helpers all behave exactly as they do on the command line.
- **The only build dependency is the Go toolchain**, pinned to an exact release:
  - It is fetched by a script into a gitignored `.toolchain/`.
  - It is checked against a git-tracked `toolchain.sha256` before use, the same discipline as the siblings' `DEPENDENCIES.sha1`.
  - Neither the toolchain nor the built binary is ever committed.
  - Pinned release: **go1.27.2**, `go1.27.2.linux-amd64.tar.gz`, sha256 `ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5`.
- **No hidden build-tool magic.** `go build`, `go test` and `go vet`, called from short hand-written scripts. No Makefile generators, no goreleaser.
- **v1 scope is fixed** (section 4). Anything else goes in section 6 until the human asks for it.
- **License: GPL-3.0-or-later**, the same as the siblings.

## 3. Architecture

```
GNOME Terminal (VTE)
   ^  escape sequences: alternate screen, colours, cursor, focus events, bracketed paste
   |  key bytes in
   v
stage (one Go process, single-threaded event loop)
  term/    raw mode (termios via syscall), window size (TIOCGWINSZ), SIGWINCH,
           restore on exit or panic, suspend/resume around `git commit`
  input/   bytes -> key events (CSI/SS3 sequences, Alt as an ESC prefix,
           paste blocks, focus in/out, two-key chords like Ctrl+C Ctrl+C)
  screen/  cell grid -> diff against the last frame -> write only changed cells;
           character width table (East Asian wide characters, emoji)
  ui/      Commit, Status and Diff panes, focus, y/N prompt, the message editor
  patch/   hunk + selected lines -> partial patch text
  git/     os/exec wrapper; parses status and diff output
   |
   v
git (the human's own install, config and hooks)
  status --porcelain=v2 -z      diff / diff --cached / diff --no-index
  apply --cached [-R] --recount apply -R --recount     add / restore --staged
  commit -F -
```

`main.go` is the composition root. It builds every component once and passes it by argument. There are no package-level variables holding state.

The repository is the only state. stage keeps nothing of its own except an unsent commit message (section 4), so quitting and restarting loses nothing.

## 4. Key design decisions already made (with rationale)

**Go with the standard library, and a hand-written terminal layer.** The human wanted a single executable, minimal build dependencies, and near-zero third-party code. They were open to Go, Rust or Python. Options weighed:
- *Python with the stdlib `curses` module.* Zero dependencies, but it can't be a single binary without PyInstaller.
- *Anything on ncurses* (Python, C, Go through cgo, Rust's `ncurses` crate). ncurses is a C library, so outside Python it means cgo or FFI, a C compiler and `libncurses-dev` at build time, and a dynamically linked binary. That throws away Go's main advantage.
- *Rust.* Its std has no termios, so it needs the `libc` crate or hand-written `unsafe` FFI. The toolchain is also larger and compiles more slowly. The usual reason to pick Rust for a TUI is ratatui, which this project wouldn't use.
- *The stacks behind Claude Code (TypeScript, React, Ink) and Codex CLI (Rust, ratatui, crossterm).* Polished, but each brings a large dependency tree. Both are also mostly inline, chat-style UIs, while stage is a fixed full-screen layout.
- *Go, stdlib only.* `syscall` has termios and `TIOCGWINSZ` on Linux, `os/signal` delivers `SIGWINCH`, `os/exec` runs git, and `CGO_ENABLED=0` gives one static binary. **Chosen.**

What stage writes by hand instead of importing is ratatui's model, kept small:
- each frame is drawn into a grid of cells;
- the grid is compared with the previous frame;
- only the changed cells are written.

For three panes that is a few hundred lines. The human hasn't used Go before (`shelf`'s brief). It was chosen anyway as the plainest of the three to review. Keep the code boring (`AGENTS.md`).

**Git through its command line.** It follows the human's own git setup with no translation layer, and it is how git-cola works.
- Status comes from `git status --porcelain=v2 -z`, which is stable, machine-readable, and safe for any filename.
- Diffs come from `git diff` (unstaged) and `git diff --cached` (staged). An untracked file is shown with `git diff --no-index -- /dev/null <file>`.

**v1 scope, agreed with the human:**
- **Review:** browse changed files and read their diffs.
- **Stage and unstage** whole files, hunks, and selected lines.
- **Revert** selected lines or hunks, or all of a file's unstaged edits, in the working tree.
- **Delete** an untracked file.
- **Commit**, with the message written in the Commit pane.

Branches, stash, merge, rebase, amend, push and pull are deliberately left to the terminal (section 6).

**Layout: the human's git-cola layout.**
- The left column, about a quarter of the width, holds **Commit** at the top (a few rows) and **Status** below it.
- **Diff** fills the rest.
- Status groups files under headings, in this order: **Staged**, **Modified**, **Untracked**, and **Unmerged** if there are any. Unmerged files can be viewed but have no actions, because merges are resolved outside stage.
- Diff shows:
  - each hunk header;
  - the old and new line numbers down the left edge, as git-cola does (a removed line has only an old number, an added line only a new one);
  - added lines on a green background and removed lines on red;
  - tabs expanded;
  - long lines wrapped, not cut off.

**Colours: chosen for a dark GNOME Terminal profile, and visibility comes first.** The human's terminal is dark. They said readability matters more here than the family's branding, so stage doesn't have to use the eel green.
- It starts with the eel green's dark-surface token, `#93B558`, for the focused pane's border and title, because that reads well on dark.
- Everything else is picked for contrast.
- stage uses 24-bit colour, which VTE supports. It doesn't detect a light background.

Starting values, to be tuned by eye in GNOME Terminal during phases 1 and 2:

| Element | Colour |
|---|---|
| Added line, background | `#1E3A1E` |
| Removed line, background | `#4A1E1E` |
| Selected added line, background | `#2F6B2F` |
| Selected removed line, background | `#7A2C2C` |
| Hunk header, text | `#7FB4CA` |
| Line numbers and unfocused borders | `#6C6C6C` |
| Cursor line in Status and Diff, background | `#3A3A3A` |
| Focused pane border and title | `#93B558` |

Text on the coloured lines stays the terminal's own foreground. Selected lines also get a `▌` in the gutter, so a selection never depends on colour alone.

**Partial staging builds a patch and hands it to `git apply`.** This is git-cola's approach (and `git add -p`'s), written here from the algorithm rather than copied. git-cola is GPL, but its exact version hasn't been confirmed. Given a hunk and a set of selected lines:
- **Stage** works from the unstaged diff:
  - unselected `+` lines are dropped;
  - unselected `-` lines become context;
  - the result is applied with `git apply --cached --recount`.
- **Unstage** works from the staged diff, applied in reverse with `git apply --cached -R --recount`. For a reverse apply the roles swap:
  - unselected `-` lines are dropped;
  - unselected `+` lines become context.
- **Revert** works from the unstaged diff, built the same way as unstage and applied with `git apply -R --recount` to the working tree.
- With no lines selected, the action applies to the hunk under the cursor.
- `--recount` lets git fix up the hunk headers' line counts, so `patch/` only has to get the lines right.

Two cases support whole-file actions only:
- **Untracked files** are staged with `git add`. Line-level staging of a new file is deferred.
- **Binary files.**

Whole files are staged with `git add -- <path>` and unstaged with `git restore --staged -- <path>`.

**Destructive actions ask first.** Revert and delete each show a one-line `y/N` prompt naming what will be lost, as git-cola's dialogs do. Anything other than `y` cancels.

**The commit message is written inline, in the Commit pane.** It has two fields, like git-cola's: a one-line summary and a multi-line extended description. On commit they are joined as `summary`, a blank line, then the description.
- An empty summary refuses to commit.
- An unsent message is saved to `.git/STAGE_MSG` as it's typed, restored on the next start, and deleted after a successful commit (git-cola keeps `GIT_COLA_MSG` the same way).
- Commit runs `git commit -F -` with the message on stdin, **with the TUI suspended**: cooked mode and the normal screen, so hooks, GPG/SSH signing and pinentry can use the terminal.
  - stage then shows git's output and waits for a key before redrawing.
  - A failed commit keeps the message.

**Keys: git-cola's bindings for actions, Emacs for editing and moving.** These come from git-cola's `cola/hotkeys.py`, adjusted for what a terminal can send. Keys are scoped by the focused pane.

| Action | Key | Notes |
|---|---|---|
| Stage or unstage the selection, hunk or file | `s` (Diff), Ctrl+S | Stages from the unstaged side, unstages from the staged side |
| Revert lines or hunk (Diff), or the file's unstaged edits (Status) | Ctrl+U | Asks y/N |
| Delete an untracked file (Status) | Delete | Asks y/N. git-cola uses Ctrl+Backspace, which terminals don't send reliably. |
| Commit | Ctrl+C Ctrl+C | magit's key. GNOME Terminal sends Ctrl+Enter as a plain Enter, so git-cola's binding can't work. |
| Clear the message | Ctrl+C Ctrl+K | |
| Focus Commit / Status / Diff | Ctrl+L / Ctrl+K / Ctrl+J | git-cola's keys. Inside the message editor Ctrl+K is kill-line, so Tab or Esc leaves the editor. |
| Move | `j`/`k`, Ctrl+N/Ctrl+P, arrows | |
| Page | `d`/`u`, Ctrl+V/Alt+V, PgDn/PgUp | |
| Top / bottom | `gg`/`G`, Alt+`<`/Alt+`>` | |
| Select diff lines | Ctrl+Space sets the mark, then move; Shift+J/Shift+K extend | Ctrl+G clears the mark |
| Next / previous hunk | `n` / `p` | Emacs-style, alongside Ctrl+N/Ctrl+P for lines |
| Refresh | Ctrl+R, F5, and automatically on focus-in | |
| Quit | Ctrl+Q | |

The message editor takes the usual Emacs keys:
- Ctrl+A/E/F/B/D/K/Y/W
- Alt+F/B/D and Alt+Backspace
- Delete and Backspace

Raw mode turns off XON/XOFF and signal keys, so Ctrl+S, Ctrl+Q, Ctrl+C and Ctrl+Z reach stage. Pasted text arrives as a bracketed paste and goes into the message verbatim, never treated as keys.

**Refresh on focus, not on a timer.** The human edits files in other terminals while stage is open. stage turns on the terminal's focus reporting (`CSI ?1004h`), and re-reads status and the current diff whenever its window regains focus. Ctrl+R and F5 do the same by hand. There is no polling and no filesystem watching. A refresh keeps the selected file and scroll position where it can.

## 5. Open questions

None at the start of v1. Raise anything that comes up here before building it.

## 6. Deliberately out of scope

These were considered and left out of v1. Each could come back only if the human's workflow needs it.
- Branches, checkout, stash, merge, rebase, cherry-pick, reset, amend, push, pull and fetch. These are done in the terminal.
- Log and history browsing, and blame.
- Mouse support.
- Search within a diff.
- Menus, a settings screen, and configurable key bindings.
- Line-level staging of untracked files (they're staged whole).
- Opening a file in `$EDITOR` from stage.
- Submodule handling beyond showing the submodule as a changed path.
- Packaging (`.deb`, Homebrew) and platforms other than Linux on amd64.
