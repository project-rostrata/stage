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

## 2026-10-12 — Old and new line numbers, as git-cola shows them

The brief said "a running line number down the left edge, as git-cola does". Looking at the human's screenshot again, git-cola's gutter has two columns: the old file's line number and the new file's. The top row of a hunk shows `.. ..` in both. A new file only fills the second column, which is why the screenshot looks like a single running count. stage does the same: context lines show both numbers, removed lines only the old one, and added lines only the new one. They are the real line numbers in the file, which helps when going back to an editor. The brief has been updated.

## 2026-10-12 — Diff flags override the human's diff config

stage passes these to every `git diff`, because the diff text has to be parseable and, from phase 4, has to apply with `git apply`:
- `--no-color` and `--no-ext-diff`;
- `--no-textconv`;
- `--no-relative`;
- `--src-prefix=a/` and `--dst-prefix=b/`.

Settings such as `diff.noprefix`, `diff.mnemonicPrefix`, `color.ui=always` or an external diff tool in the human's `~/.gitconfig` would otherwise break parsing or patches. A test runs against a repository with all four set. Settings that only change which lines differ (`diff.algorithm`, `diff.context`) are kept, so stage shows the same hunks as `git diff` at the prompt.

## 2026-10-12 — Untracked files are listed one by one

`git status` normally lists a new directory as a single entry. stage passes `--untracked-files=all`, as git-cola's Status list does (the human's screenshot shows `android/Dockerfile` and its siblings separately), so each untracked file can be read and staged on its own.

## 2026-10-12 — Refresh and hunk keys arrived early

Refresh (focus-in, Ctrl+R, F5) and `n`/`p` were planned for phases 3 and 4. Each was a few lines once the viewer existed, and the viewer is much less useful without refresh, so they landed in phase 2.

## 2026-10-12 — Paths are always literal (GIT_LITERAL_PATHSPECS=1)

Git reads a path argument as a pattern. `git add` happens to check for an exact filename first, but `git clean -- 'a*'` deletes every untracked file starting with `a`. Checked by hand, and pinned by `TestPathsAreLiteral`, which fails without the setting. Every git command stage runs sets `GIT_LITERAL_PATHSPECS=1`, so a path from `git status` only ever means that one file.

## 2026-10-12 — Deleting an untracked file uses git clean

Delete runs `git clean --force -- <path>` rather than removing the file directly. git clean refuses to delete anything tracked, so a stale Status list can never delete committed work. Empty directories are left behind; git doesn't track them, so they don't show up in Status.

## 2026-10-12 — After an action, the next file in the same section is selected

Once a file is staged, unstaged, reverted or deleted, it leaves its section. Keeping the same position in the whole list made the selection jump: staging an untracked file selected a Modified file. stage keeps the same position within the file's old section instead, so the next file in that list is selected, as in git-cola, and Ctrl+S can be pressed repeatedly to stage files one after another. Only when that section is now empty does it fall back to the same position in the whole list.

## 2026-10-12 — Partly staging a block puts the staged lines after it

Within one block of removed lines followed by added lines, a removed line that isn't selected stays where it is, as context, ahead of the block's added lines. So staging only `+ONE` and `+TWO` from

```
-one
-two
-three
+ONE
+TWO
+THREE
```

leaves `one two three ONE TWO` in the index. git add -p's edit mode, git-gui and git-cola all build these patches the same way, and a selection in stage is a contiguous range, as in git-cola. The usual case, replacing one line in the middle of unchanged lines, comes out exactly right. `TestPartOfABlockGoesAfterItsRemainingLines` records the behaviour on purpose. Pairing the removed and added lines one-to-one was considered and left out: it guesses the human's intent, and would surprise anyone used to git-cola.

## 2026-10-12 — A line patch never renames, creates or deletes, unless everything is included

A line patch's header always names one path, the new one. Unstaging a line of a staged rename therefore changes that file's content in the index, and the rename stays (Ctrl+S in Status unstages the rename itself). "new file mode" and "deleted file mode" are kept only when every change in the file is included. Staging part of a deletion leaves a shorter file in the index rather than deleting it, and unstaging part of a new file leaves a shorter new file.

## 2026-10-12 — Shift+arrows and Esc in the Diff pane

The brief's selection keys were Ctrl+Space, Shift+J/K and Ctrl+G. Shift+Up/Down also extend the selection, and Esc also clears the mark, because both are what a terminal user reaches for first and neither key had another meaning in the Diff pane.

## 2026-10-12 — git apply --whitespace=nowarn

A partial patch is the human's own lines, so stage applies it as written. Without this flag, an `apply.whitespace=fix` or `=error` setting would rewrite or refuse lines with trailing whitespace that `git add` would have staged unchanged.

## 2026-10-12 — Commit from the draft file, not stdin

The brief said `git commit -F -`. With the message on stdin, git's stdin is a pipe, so a hook that reads its input can't reach the terminal (pinentry opens `/dev/tty` itself, but hooks don't always). stage already keeps the message in `.git/STAGE_MSG`, so it runs `git commit --file <that path>` and leaves stdin on the terminal. git's default cleanup for a message that isn't edited is `whitespace`: it trims surrounding blank lines and trailing spaces, and keeps `#` lines.

## 2026-10-12 — After a commit, wait only on failure

The brief said stage would always show git's output and wait for a key. Waiting after every successful commit is an extra keypress for nothing. git's output is copied to the terminal and also kept by stage. On success, stage returns straight away and shows the `[main abc1234] Summary` line in the status bar. On failure it waits for Enter on the normal screen, where the hook's or git's output is, and keeps the message.

## 2026-10-12 — Ctrl+C during a commit stops git, not stage

While suspended, Ctrl+C sends SIGINT to the whole foreground process group, stage included. SIGINT is therefore kept on its own channel and forgotten on resume: the commit is aborted, and stage comes back with the message kept. A SIGINT from outside while stage is running normally still quits cleanly, as SIGTERM and SIGHUP do.

## 2026-10-12 — Message editor details

- **Ctrl+W deletes the word before the cursor, as in readline and the shell.** The editor has no mark, so Emacs's kill-region would have nothing to kill. Ctrl+Backspace does the same, if the terminal sends it as `^H`.
- **Ctrl+C Ctrl+K needs no confirmation.** The cleared text becomes the editor's last kill, so Ctrl+Y in the empty summary restores it.
- **Commit works from any pane:** stage something in Status or Diff, then Ctrl+C Ctrl+C. Committing doesn't require going back to the message.
- **A restored draft puts the cursor at the end of the summary.**
- **The draft is saved once per batch of keys.** Its path is looked up once. Saving per key, with a lookup each time, delayed a 100-character paste by about half a second.

## 2026-10-12 — An audit trail of the git commands stage runs

The human asked for a record of the git commands stage runs, since stage only drives the ordinary git command line. Their choices:
- **Commands that change something only:** `add`, `restore`, `rm --cached`, `clean`, `apply`, `commit`. The reads run on every refresh and selection, and would bury them.
- **A file plus an in-app view:** `.git/stage.log`, next to git's own logs, which is per repository and readable with `less` or `tail -f`. Ctrl+O shows it in place of the Diff pane. git-cola has a console view but no key for it, and Ctrl+O was free in every pane.
- **The command line only, not a partial patch's text.**

Each line is the time, `ok` or `failed`, and the command quoted for a POSIX shell, with control characters in `$'...'` form so every entry stays on one line. A command that worked but couldn't be logged is reported as an error, so the trail is never silently incomplete.

## 2026-10-12 — --literal-pathspecs replaces GIT_LITERAL_PATHSPECS=1

The literal-paths protection (above) moved from the environment variable to git's equivalent global flag. Both behave the same, and `TestPathsAreLiteral` still pins it. But an environment variable is invisible in a recorded command line, and the trail should show exactly what ran: `git --literal-pathspecs add -- 'a*'` can be pasted and behaves the same.

## 2026-10-13 — Basic hardening, no sandbox

The human asked what security audit stage needs. They don't want stage ever to run code from a repository, to be open to git command injection, or to expose git credentials.

The finding: a Go binary on Linux has no sandbox. stage runs as the human, so the useful question is what a repository, or a file name in it, can make stage or git do. `git status` runs a repository's `core.fsmonitor` command, which was confirmed in the sandbox, and stage reads status on every refresh.

**Decided:**
- **`git commit` from stage behaves exactly as plain `git commit`.** Hooks, including ones that run scripts committed in the repository, signing and filters all run.
- **No sandbox.** Landlock, bubblewrap and AppArmor were considered. Landlock is the only one that fits a single binary without root, but it confines git and the hooks too, which breaks signing and hooks that need more than the repository.
- **Every command runs with `-c core.fsmonitor=false`.** fsmonitor only speeds up status in very large trees, and no result changes. The flag is on `git commit` too: commit only uses fsmonitor to refresh the index, so its behaviour is unchanged. A human who uses git's built-in fsmonitor daemon gets slightly slower status from stage, and their own git keeps it.
- **The draft and the audit trail are opened with `O_NOFOLLOW`, and the commit checks the draft is a plain file.** A planted link would otherwise load a private key into the message, commit it, or append to a file outside the repository.
- **Filters are left alone.** A clean or smudge filter in a planted `.git/config` still runs, but turning filters off would break git-lfs. This is recorded as accepted in the brief.

Already in place: argument lists, never a shell; `--` before paths; `--literal-pathspecs`; `--no-ext-diff --no-textconv`; control characters drawn as `�`; Go's refusal to run `./git` through `.` in PATH; `GOTOOLCHAIN=local`; no third-party modules. Each now has a test named in the brief's threat model.

The audit trail's lines grow by `-c core.fsmonitor=false`. They stay exactly what ran.

## 2026-10-13 — A branch list, and switching branches

The human asked for a view of the most recent branches: whether each is behind its remote, a shortened last commit message, and its local time. A key in the bottom bar switches between it and the three panes. stage opens on it when the tree is clean, and on the three panes when it's dirty. This reverses v1's "branch selection happens in the terminal" for switching only; creating and deleting branches stay out.

- **Order: the latest commit, newest first.** One `for-each-ref --sort=-committerdate`, which matches the time column. The alternative, most recently checked out (from the HEAD reflog), was turned down.
- **Behind dates from the last fetch.** Fetching would mean the network and credential helpers, which stage has never touched (the threat model's "Credentials"). The title shows `FETCH_HEAD`'s time so the staleness is visible. Only "behind" is shown, as asked; "ahead" isn't.
- **Enter switches, with `git switch -- <name>`.**
  - `switch` only takes branches, so a tag of the same name can't be picked.
  - A branch named like an option can exist in a planted `.git`, and `for-each-ref` lists it, which was checked. `--` makes it only a name.
  - With uncommitted changes, stage asks first, because they come along. git refuses a switch that would overwrite them.
  - stage then returns to the three panes.
- **Ctrl+T toggles the views,** from anywhere. It was free in every pane, VTE sends it, and Emacs's transpose-chars isn't implemented.
- **Clean means nothing staged, modified, untracked or unmerged.** Untracked files count, because they're something to stage.
- **A refactor:** `gitPath` replaces the separate draft and audit-trail path lookups, now that `FETCH_HEAD` needs one too.

## 2026-10-13 — A commit that leaves the tree clean moves to the branch list

The human asked for this. It's the startup rule applied again: with nothing left to stage, the next thing is usually another branch. Only a successful commit does it, and only when nothing staged, modified, untracked or unmerged remains, so a partial commit stays on the three panes. Reverting or deleting the last change doesn't move to the list; the human asked only about commits.
