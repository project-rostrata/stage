# stage

A small terminal git client for one job: reviewing your changes, staging them by file, hunk
or line, reverting the bits you don't want, and committing. It copies git-cola's three
panes (Commit, Status and Diff) into the terminal you're already working in. That's the
whole program; branches, stashes, merges and pushes stay with plain `git`.

> **Status: planning.** Nothing is built yet. See [`V1_PLAN.md`](V1_PLAN.md).

## Why

stage is built the same way as its sibling projects `shelf`, `larder`, `press`, `proof` and
`list`: only what is necessary. It's one static Go binary using only the standard library: no
ncurses, no TUI framework, no third-party modules. It drives git through the `git` command,
so your hooks, signing and config work exactly as they do on the command line. The only build
dependency is a pinned Go toolchain, fetched and checksum-verified by a script.

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
