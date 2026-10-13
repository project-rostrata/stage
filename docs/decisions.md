# Decisions log

An append-only record of decisions made while building stage, beyond what `PROJECT_BRIEF.md` and `V1_PLAN.md` already fix. The newest entries are at the bottom. Don't relitigate any of these without flagging it to the human first.

## 2026-10-12 — A terminal app, not a web app, so not Kotlin/VanJS

The human uses git-cola for reviewing, staging and committing, and wanted that workflow in the terminal they already work in. The siblings' stack is a Kotlin backend and a VanJS frontend. The human ruled it out at the start because stage is a CLI, and a JVM's startup time argues against it anyway.

## 2026-10-12 — Go, standard library only, with a hand-written terminal layer

The human's requirements: near-zero dependencies, one executable binary, and minimal build dependencies. They preferred Go or Rust, and were open to Python. The options and why each was set aside are in the brief §4. In short:
- **Python's `curses`** can't produce a single binary.
- **ncurses from any other language** needs cgo or FFI, a C compiler and dynamic linking.
- **Rust** needs `libc` or `unsafe` FFI for termios, and its main TUI attraction (ratatui) wouldn't be used.
- **Ink and ratatui**, the stacks behind Claude Code and Codex CLI, bring large dependency trees.

The human first asked why the language and the UI library were being chosen as pairs. They are separate choices. Once ncurses was ruled out, the question became which language's standard library can drive a terminal by itself, and Go's can.

## 2026-10-12 — git through its command line

Driving git through `git` itself keeps the human's hooks, signing, config and credential helpers working exactly as on the command line, and needs no library. It is also how git-cola works. Partial staging uses `git apply` with a hand-built patch (brief §4).

## 2026-10-12 — v1 scope, agreed with the human

- **In:** reviewing diffs; staging and unstaging whole files and selected lines or hunks; reverting selected lines or hunks; deleting untracked files; committing.
- **Out:** branches, stash, merges and everything else in the brief §6. The human does those outside git-cola today and will keep doing them in the terminal.

## 2026-10-12 — Commit message inline; Ctrl+C Ctrl+C to commit

- **Inline message:** the human chose to write the message in the Commit pane, as summary and description fields like git-cola's, over handing off to `$EDITOR`.
- **Commit key:** git-cola commits with Ctrl+Enter. The human's terminal is GNOME Terminal, which sends Ctrl+Enter as a plain Enter (VTE has no kitty keyboard protocol), so that key can't work. The human chose magit's Ctrl+C Ctrl+C over Alt+Enter.

## 2026-10-12 — Keys: git-cola for actions, Emacs for editing and moving

- **Split:** the human asked for git-cola's action bindings, and Emacs keys for text editing and navigation everywhere, alongside git-cola's own `j`/`k`/`d`/`u`/`gg`/`G`. The full table is in the brief §4.
- **Ctrl+K:** it clashes in the message editor. The human chose Emacs kill-line there, so Tab or Esc leaves the editor, while Ctrl+K still focuses Status from the Diff pane.
- **No mouse.** The human works keyboard-only.

## 2026-10-12 — Refresh on terminal focus, not on a timer

The human chose refresh on focus-in, plus a manual key, over polling or manual-only refresh. It means no background work and no filesystem watching, and a change made in another terminal shows up as soon as stage's window is focused again.

## 2026-10-12 — Name: stage

The human chose `stage` from `sift`, `seam` and `stage`. It names the thing done most in the app, in git's own word, and it's short to type.

## 2026-10-12 — The brief's open questions, answered by the human

- **Colours.** GNOME Terminal is dark. The human said visibility matters more than branding here, so stage needn't use the eel green. It keeps `#93B558` (the eel green's dark-surface token) for the focused pane because it reads well, and picks everything else for contrast. The starting palette is in the brief §4.
- **Hunk navigation:** `n` and `p`, which match the Emacs style.
- **Deleting an untracked file:** git-cola's Ctrl+Backspace was dropped, because what a terminal sends for it depends on the profile. The human asked for something that works in the terminal. It's **Delete** in the Status pane, behind the same y/N prompt. VTE sends Delete as `CSI 3 ~` regardless of the backspace setting. The message editor drops Ctrl+Backspace too, and Alt+Backspace covers deleting the previous word.

## 2026-10-12 — One goroutine: reads time out after a tenth of a second

The terminal is read with `VMIN=0, VTIME=1`, so a read returns after 100 ms even when no key was pressed. The event loop is then a plain loop on one goroutine:
1. read;
2. decode;
3. handle the keys;
4. check for a resize or a quit signal;
5. redraw if anything changed.

The alternative was a reader goroutine feeding a channel. A blocked read can't be cancelled, and that would matter in phase 5: while `git commit` owns the terminal, a background reader would steal keystrokes meant for hooks or pinentry. With timed reads, stage simply stops reading while it is suspended.

The same timeout separates a lone Esc from the start of an escape sequence: an ESC with nothing after it for 100 ms is the Esc key (vim's default `ttimeoutlen` is also 100 ms). An idle stage wakes ten times a second and writes nothing.

## 2026-10-12 — A status bar row under the panes

The brief's layout has no room for messages, but the brief and `AGENTS.md` need one: git's errors and the `y/N` prompts have to go somewhere. The bottom row is a one-line status bar, and the three panes fill the rest. In phase 1 it shows the last key pressed, to check what GNOME Terminal sends.

## 2026-10-12 — Tab and Shift+Tab cycle the panes

The brief only says Tab leaves the message editor. In phase 1, Tab and Shift+Tab move focus Commit → Status → Diff and back, in every pane. Inside the editor Tab can't mean "insert a tab" anyway, and the same key working everywhere is simpler than an exception.

## 2026-10-12 — Pasted line breaks become `\n`

Terminals send a pasted line break as CR, as if Enter had been pressed (both VTE and tmux do). The decoder turns CR and CRLF inside a bracketed paste into LF, so the message editor receives text, not keystrokes.
