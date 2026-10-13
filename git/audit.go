package git

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// auditName is the file in the git directory that records every git command
// stage ran that changed the repository, one line each, next to git's own logs.
const auditName = "stage.log"

// auditPath asks git where the audit trail lives, once.
func (r *Repo) auditPath() (string, error) {
	if r.audit != "" {
		return r.audit, nil
	}
	out, err := r.read(nil, "rev-parse", "--git-path", auditName)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(string(out), "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	r.audit = p
	return p, nil
}

// record appends a command to the audit trail: when it ran, whether it
// succeeded, and the command line exactly as run, quoted so it can be pasted
// into a shell. args excludes the leading "git".
func (r *Repo) record(args []string, runErr error) error {
	p, err := r.auditPath()
	if err != nil {
		return err
	}
	status := "ok"
	if runErr != nil {
		status = "failed"
	}
	line := fmt.Sprintf("%s  %-6s  %s\n", time.Now().Format("2006-01-02 15:04:05"), status,
		shellQuote(append([]string{"git"}, args...)))
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", auditName, err)
	}
	if _, err := f.WriteString(line); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", auditName, err)
	}
	return f.Close()
}

// AuditTrail returns the last n lines of the audit trail, oldest first, and
// the file's path. A repository stage has never changed has no trail yet.
func (r *Repo) AuditTrail(n int) ([]string, string, error) {
	p, err := r.auditPath()
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, p, nil
	}
	if err != nil {
		return nil, p, fmt.Errorf("read %s: %w", auditName, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, p, fmt.Errorf("read %s: %w", auditName, err)
	}
	return lines, p, nil
}

// shellQuote joins a command line the way a POSIX shell would read it back:
// plain words as they are, anything else in single quotes, and control
// characters (a newline in a file name) in bash's $'...' form, so each
// entry stays on one line.
func shellQuote(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = quoteWord(a)
	}
	return strings.Join(quoted, " ")
}

func quoteWord(s string) string {
	if s == "" {
		return "''"
	}
	plain, control := true, false
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f:
			control = true
		case !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:@%+,", r)):
			plain = false
		}
	}
	switch {
	case control:
		var b strings.Builder
		b.WriteString("$'")
		for _, r := range s {
			switch {
			case r == '\'' || r == '\\':
				b.WriteByte('\\')
				b.WriteRune(r)
			case r == '\n':
				b.WriteString(`\n`)
			case r == '\t':
				b.WriteString(`\t`)
			case r < 0x20 || r == 0x7f:
				fmt.Fprintf(&b, `\x%02x`, r)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('\'')
		return b.String()
	case plain:
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
