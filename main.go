// stage is a terminal git client for reviewing, staging and committing,
// laid out like git-cola's Commit, Status and Diff panes.
//
// Copyright (C) 2026 Casey Watson. Licensed under GPL-3.0-or-later; see LICENSE.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"stage/git"
	"stage/screen"
	"stage/term"
	"stage/ui"
)

func main() {
	os.Exit(run())
}

// run is the composition root: it builds each component once and wires them
// together. The terminal is restored on every way out, including a panic.
func run() (code int) {
	repo, err := git.Open(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "stage:", err)
		return 1
	}
	t, err := term.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stage:", err)
		return 1
	}
	app := ui.New(t, screen.New(t), repo)

	defer func() {
		if r := recover(); r != nil {
			t.Stop()
			fmt.Fprintf(os.Stderr, "stage: panic: %v\n\n%s", r, debug.Stack())
			code = 2
		}
	}()
	if err := t.Start(); err != nil {
		t.Stop()
		fmt.Fprintln(os.Stderr, "stage:", err)
		return 1
	}
	err = app.Run()
	if stopErr := t.Stop(); err == nil {
		err = stopErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "stage:", err)
		return 1
	}
	return 0
}
