// Package term puts the terminal into raw mode on the alternate screen and
// always puts it back. It is the only package that touches termios.
package term

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

// Sequences written when stage takes over the terminal and when it hands it back.
// Auto-wrap is off while stage runs, so drawing the bottom-right cell never scrolls.
const (
	altScreenOn     = "\x1b[?1049h"
	altScreenOff    = "\x1b[?1049l"
	autoWrapOff     = "\x1b[?7l"
	autoWrapOn      = "\x1b[?7h"
	focusEventsOn   = "\x1b[?1004h"
	focusEventsOff  = "\x1b[?1004l"
	bracketPasteOn  = "\x1b[?2004h"
	bracketPasteOff = "\x1b[?2004l"
	cursorHide      = "\x1b[?25l"
	cursorShow      = "\x1b[?25h"
	styleReset      = "\x1b[0m"

	enterSequence = altScreenOn + autoWrapOff + focusEventsOn + bracketPasteOn + cursorHide
	leaveSequence = bracketPasteOff + focusEventsOff + styleReset + autoWrapOn + cursorShow + altScreenOff
)

// Terminal is the controlling terminal on stdin and stdout.
type Terminal struct {
	in     int
	out    *os.File
	saved  syscall.Termios
	active bool
	winch  chan os.Signal
	quit   chan os.Signal
	intr   chan os.Signal
}

// Open checks that stdin and stdout are a terminal and remembers its current
// settings so Stop can restore them. It doesn't change anything yet.
func Open() (*Terminal, error) {
	in := int(os.Stdin.Fd())
	var saved syscall.Termios
	if err := ioctl(in, syscall.TCGETS, unsafe.Pointer(&saved)); err != nil {
		return nil, errors.New("stdin is not a terminal")
	}
	var probe syscall.Termios
	if err := ioctl(int(os.Stdout.Fd()), syscall.TCGETS, unsafe.Pointer(&probe)); err != nil {
		return nil, errors.New("stdout is not a terminal")
	}
	t := &Terminal{
		in:    in,
		out:   os.Stdout,
		saved: saved,
		winch: make(chan os.Signal, 1),
		quit:  make(chan os.Signal, 1),
		intr:  make(chan os.Signal, 1),
	}
	signal.Notify(t.winch, syscall.SIGWINCH)
	// Raw mode turns off the keys that send these, so they only come from outside
	// (kill, a closed window). stage still has to restore the terminal for them.
	signal.Notify(t.quit, syscall.SIGTERM, syscall.SIGHUP)
	// SIGINT is kept apart: while suspended, Ctrl+C at a hook's prompt sends it
	// to stage as well as to git. That aborts the commit, not stage, so Resume
	// forgets it.
	signal.Notify(t.intr, syscall.SIGINT)
	return t, nil
}

// Start switches to raw mode and the alternate screen, with focus reporting and
// bracketed paste on.
func (t *Terminal) Start() error {
	raw := t.saved
	// The same flags as cfmakeraw(3). With IXON and ISIG off, Ctrl+S, Ctrl+Q,
	// Ctrl+C and Ctrl+Z arrive as keys instead of controlling the terminal.
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	// A read returns after a tenth of a second even with no input. That lets the
	// single-threaded event loop notice resizes, and tell a lone Esc from the
	// start of an escape sequence, without a second goroutine.
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := ioctl(t.in, syscall.TCSETS, unsafe.Pointer(&raw)); err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	t.active = true
	_, err := t.out.WriteString(enterSequence)
	return err
}

// Stop restores the terminal exactly as Open found it. It is safe to call more
// than once, and from a deferred panic handler.
func (t *Terminal) Stop() error {
	if !t.active {
		return nil
	}
	t.active = false
	_, werr := t.out.WriteString(leaveSequence)
	if err := ioctl(t.in, syscall.TCSETS, unsafe.Pointer(&t.saved)); err != nil {
		return fmt.Errorf("restore terminal: %w", err)
	}
	return werr
}

// Suspend hands the terminal back to the shell's normal state, so a program
// stage runs (git commit, its hooks, pinentry) can use it.
func (t *Terminal) Suspend() error { return t.Stop() }

// Resume takes the terminal back after Suspend. The caller must redraw everything.
func (t *Terminal) Resume() error {
	select {
	case <-t.intr:
	default:
	}
	return t.Start()
}

// Input is the terminal's input, for a program run while suspended.
func (t *Terminal) Input() *os.File { return os.Stdin }

// WaitForEnter prints a prompt and waits for a line, while suspended.
func (t *Terminal) WaitForEnter(prompt string) error {
	if _, err := t.out.WriteString(prompt); err != nil {
		return err
	}
	_, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return err
}

// Size returns the terminal's width and height in cells.
func (t *Terminal) Size() (int, int, error) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	if err := ioctl(int(t.out.Fd()), syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err != nil {
		return 0, 0, fmt.Errorf("window size: %w", err)
	}
	return int(ws.Col), int(ws.Row), nil
}

// Read waits up to a tenth of a second for input. It returns 0 if none arrived.
func (t *Terminal) Read(buf []byte) (int, error) {
	for {
		n, err := syscall.Read(t.in, buf)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		return n, nil
	}
}

// Write sends bytes to the terminal.
func (t *Terminal) Write(p []byte) (int, error) { return t.out.Write(p) }

// Resized reports whether the window has changed size since the last call.
func (t *Terminal) Resized() bool {
	select {
	case <-t.winch:
		return true
	default:
		return false
	}
}

// QuitRequested reports whether stage was asked to exit by a signal.
func (t *Terminal) QuitRequested() bool {
	select {
	case <-t.quit:
		return true
	case <-t.intr:
		return true
	default:
		return false
	}
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
