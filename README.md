# stage

A small terminal git client for one job: reviewing your changes, staging them by file, hunk
or line, reverting the bits you don't want, and committing. It copies git-cola's three
panes (Commit, Status and Diff) into the terminal you're already working in. That's the
whole program; branches, stashes, merges and pushes stay with plain `git`.

> **Status: v1 built, awaiting its first real use in GNOME Terminal.** See [`V1_PLAN.md`](V1_PLAN.md).

## Why

stage is built the same way as its sibling projects `shelf`, `larder`, `press`, `proof` and
`list`: only what is necessary. It's one static Go binary using only the standard library: no
ncurses, no TUI framework, no third-party modules. It drives git through the `git` command,
so your hooks, signing and config work exactly as they do on the command line. The only build
dependency is a pinned Go toolchain, fetched and checksum-verified by a script.

## Install

```
./install.sh
```

This fetches the pinned Go toolchain into `.toolchain/` (checked against `toolchain.sha256`),
builds the static `stage` binary, and links it into `~/.local/bin`. Then run `stage` anywhere
inside a git repository. `./test.sh` runs the formatting check, `go vet` and the tests.

## Using it

The left column is **Commit** (top) and **Status** (below); **Diff** fills the right. Status
lists files under Staged, Modified and Untracked; Diff shows the selected file's diff. Moving
to another terminal window and back refreshes everything.

| Do | Keys |
|---|---|
| Move between panes | Tab / Shift+Tab, or Ctrl+L (Commit), Ctrl+K (Status), Ctrl+J (Diff) |
| Move | `j`/`k`, Ctrl+N/Ctrl+P, arrows; `d`/`u` half a page; Ctrl+V/Alt+V or PgDn/PgUp a page; `gg`/`G`, Alt+`<`/Alt+`>` the ends |
| Stage or unstage a file | Ctrl+S in Status |
| Revert a file's unstaged changes | Ctrl+U in Status (asks first) |
| Delete an untracked file | Delete in Status (asks first) |
| Jump between hunks | `n` / `p` in Diff |
| Select lines | Ctrl+Space in Diff sets the mark, then move; Shift+J/Shift+K or Shift+arrows extend; Ctrl+G or Esc clears |
| Stage or unstage the selection, or the hunk under the cursor | `s` or Ctrl+S in Diff |
| Revert the selection, or the hunk under the cursor | Ctrl+U in Diff (asks first) |
| Write the message | Ctrl+L, then type: Enter moves from the summary to the description; Emacs keys (Ctrl+A/E/F/B/N/P/D/K/Y/W, Alt+F/B/D, Alt+Backspace) edit |
| Leave the message | Tab or Esc |
| Commit | Ctrl+C Ctrl+C, from any pane |
| Clear the message | Ctrl+C Ctrl+K (Ctrl+Y in the summary brings it back) |
| Refresh | Ctrl+R or F5 |
| Quit | Ctrl+Q |

The message is saved in `.git/STAGE_MSG` as you type, so quitting doesn't lose it. A commit
runs `git commit` on the normal screen, so hooks and signing prompts work as usual.

## Documentation

- [`PROJECT_BRIEF.md`](PROJECT_BRIEF.md): the target design and non-negotiable constraints
- [`V1_PLAN.md`](V1_PLAN.md): the phased build plan
- [`docs/decisions.md`](docs/decisions.md): an append-only log of design decisions and why they were made
- [`AGENTS.md`](AGENTS.md): coding conventions for anyone (human or AI) working on the code

## License

Copyright (C) 2026 Casey Watson.

stage is free software: you can redistribute it and/or modify it under the terms of the GNU
General Public License as published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version. See [`LICENSE`](LICENSE) for the full text.

stage is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even
the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
