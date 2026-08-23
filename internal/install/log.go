package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RunLog records everything a run did, to a file.
//
// The screen cannot be the only record. Progress output is transient by design
// — a 400-tool run would otherwise scroll thousands of lines past — so when
// something fails the evidence is already gone by the time anyone looks. The
// log keeps the full output of every command regardless of what was displayed,
// which is the difference between "exit status 1" and knowing why.
type RunLog struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// NewRunLog opens a timestamped log under the state directory.
//
// Failure to open one is deliberately not fatal: not being able to write a log
// is a worse reason to refuse to install than to proceed without it.
func NewRunLog(stateDir, what string) *RunLog {
	dir := filepath.Join(stateDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &RunLog{}
	}
	name := fmt.Sprintf("%s-%s.log", time.Now().Format("2006-01-02-150405"), sanitise(what))
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return &RunLog{}
	}

	l := &RunLog{f: f, path: f.Name()}
	l.Printf("macwtf run: %s", what)
	l.Printf("started: %s", time.Now().Format(time.RFC3339))
	l.Printf("%s", strings.Repeat("─", 60))
	return l
}

func sanitise(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		case r == ' ', r == '_', r == '/':
			return '-'
		}
		return -1
	}, s)
	if s == "" {
		return "run"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// Path is where the log is being written, or empty if there is none.
func (l *RunLog) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Printf writes a line.
func (l *RunLog) Printf(format string, args ...any) {
	if l == nil || l.f == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.f, format+"\n", args...)
}

// Event records a progress event, including the command output that the
// display throws away.
func (l *RunLog) Event(ev Event) {
	if l == nil || l.f == nil {
		return
	}
	switch ev.Kind {
	case EventToolStart:
		l.Printf("\n=== %s (%s) ===", ev.Tool.ID, ev.Tool.Backend)
	case EventStepStart:
		l.Printf("$ %s", ev.Step.String())
	case EventOutput:
		l.Printf("  %s", ev.Line)
	case EventToolDone:
		if ev.Err != nil {
			l.Printf("FAILED after %s: %v", ev.Elapsed.Round(time.Millisecond), ev.Err)
		} else {
			l.Printf("ok in %s", ev.Elapsed.Round(time.Millisecond))
		}
	case EventRunDone:
		l.Printf("\n%s\nfinished in %s", strings.Repeat("─", 60),
			ev.Elapsed.Round(time.Millisecond))
	}
}

// Close finishes the log.
func (l *RunLog) Close() {
	if l == nil || l.f == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f.Close()
	l.f = nil
}
