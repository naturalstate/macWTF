package install

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/pathenv"
	"github.com/naturalstate/macWTF/internal/state"
)

// EventKind classifies progress events.
type EventKind int

const (
	EventRunStart EventKind = iota
	EventToolStart
	EventStepStart
	EventOutput // a line of stdout/stderr from a running command
	EventToolDone
	EventRunDone
)

// Event is one thing that happened during a run.
//
// The executor reports progress rather than printing it, so the CLI and the TUI
// render the same run in their own way without the executor knowing which is
// watching.
type Event struct {
	Kind EventKind
	Tool *manifest.Tool
	Step backend.Step

	// Index and Total position this tool in the run, for a progress bar.
	Index int
	Total int

	// Line carries command output for EventOutput.
	Line string

	// Err is set on EventToolDone when the tool failed.
	Err error

	// Elapsed is set on EventToolDone and EventRunDone.
	Elapsed time.Duration
}

// Result summarises a finished run.
type Result struct {
	Installed []*manifest.Tool

	// Failed is tools that were attempted and did not succeed.
	Failed []FailedTool

	// Blocked is tools that were never attempted, because the backend they
	// need is not available. Kept apart from Failed deliberately: reporting
	// them together makes a missing package manager look like nine broken
	// installs, which sends the user hunting for the wrong cause.
	Blocked []FailedTool

	Skipped []*manifest.Tool // already present

	// QuarantineStripped lists tools whose Gatekeeper quarantine attribute
	// was removed, for the end-of-run report.
	QuarantineStripped []*manifest.Tool

	Elapsed time.Duration
}

// FailedTool is a tool that did not install, and why.
type FailedTool struct {
	Tool *manifest.Tool
	Err  error
}

// Executor runs a plan.
type Executor struct {
	Ctx   *backend.Ctx
	State *state.State

	// Emit receives progress events. Never nil in practice; guarded anyway.
	Emit func(Event)

	mu      sync.Mutex
	envOnce sync.Once
	env     []string
}

func (e *Executor) emit(ev Event) {
	if e.Emit == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Emit(ev)
}

// Run executes a plan, tool by tool.
//
// A failing tool does not abort the run. One dead cask must not cost the user
// the other fifty tools, so failures are recorded and reported at the end.
// State is saved after every tool rather than once at the end, so an interrupted
// run still knows what it managed to install.
func (e *Executor) Run(ctx context.Context, p *Plan) (*Result, error) {
	start := time.Now()
	res := &Result{}

	var todo []ToolPlan
	for _, tp := range p.Tools {
		switch {
		case tp.AlreadyInstalled:
			res.Skipped = append(res.Skipped, tp.Tool)
			// Record it as preexisting so a later remove leaves it
			// alone: macWTF did not put it there.
			if _, known := e.State.Record(tp.Tool.ID); !known {
				e.State.Put(state.Record{
					ID:          tp.Tool.ID,
					Backend:     tp.Tool.Backend,
					Package:     tp.Tool.Package,
					Preexisting: true,
					InstalledAt: time.Now().UTC(),
				})
			}
		case tp.PlanErr != nil:
			if _, dead := p.BackendErrs[tp.Tool.Backend]; dead {
				res.Blocked = append(res.Blocked, FailedTool{tp.Tool, tp.PlanErr})
			} else {
				res.Failed = append(res.Failed, FailedTool{tp.Tool, tp.PlanErr})
			}
		default:
			todo = append(todo, tp)
		}
	}

	e.emit(Event{Kind: EventRunStart, Total: len(todo)})

	for i, tp := range todo {
		if err := ctx.Err(); err != nil {
			return res, err
		}

		e.emit(Event{Kind: EventToolStart, Tool: tp.Tool, Index: i, Total: len(todo)})
		toolStart := time.Now()

		err := e.runTool(ctx, tp, i, len(todo))
		elapsed := time.Since(toolStart)

		rec := state.Record{
			ID:          tp.Tool.ID,
			Backend:     tp.Tool.Backend,
			Package:     tp.Tool.Package,
			InstalledAt: time.Now().UTC(),
		}

		if err != nil {
			res.Failed = append(res.Failed, FailedTool{tp.Tool, err})
			rec.Failed = true
			rec.Error = err.Error()
		} else {
			res.Installed = append(res.Installed, tp.Tool)
			for _, s := range tp.Steps {
				if s.Kind == backend.KindQuarantine {
					rec.QuarantineStripped = true
					res.QuarantineStripped = append(res.QuarantineStripped, tp.Tool)
				}
			}
		}

		e.State.Put(rec)
		// Save after every tool: an interrupted run must not lose the
		// record of what it already installed.
		if saveErr := e.State.Save(); saveErr != nil {
			e.emit(Event{Kind: EventOutput,
				Line: "warning: could not write state: " + saveErr.Error()})
		}

		e.emit(Event{
			Kind: EventToolDone, Tool: tp.Tool, Index: i, Total: len(todo),
			Err: err, Elapsed: elapsed,
		})
	}

	res.Elapsed = time.Since(start)
	e.emit(Event{Kind: EventRunDone, Total: len(todo), Elapsed: res.Elapsed})
	return res, nil
}

// runTool executes every step for one tool, stopping at the first failure.
func (e *Executor) runTool(ctx context.Context, tp ToolPlan, index, total int) error {
	for _, step := range tp.Steps {
		e.emit(Event{Kind: EventStepStart, Tool: tp.Tool, Step: step, Index: index, Total: total})

		if err := e.runStep(ctx, step, tp.Tool, index, total); err != nil {
			// A failed verification is worth reporting distinctly:
			// the install command succeeded but the tool is not
			// where the manifest says it should be.
			if step.Kind == backend.KindVerify {
				return fmt.Errorf("installed, but verification failed: %s", step.String())
			}
			return fmt.Errorf("%s: %w", step.Kind, err)
		}
	}
	return nil
}

// stepEnv is the environment steps run under: the caller's, with every
// backend's bin directory prepended to PATH.
//
// Without this, verification lies. `go install` puts a binary in ~/go/bin and
// pipx puts entry points in ~/.local/bin, neither of which is on the default
// PATH — so a tool that installed perfectly fails `command -v` and gets
// reported as broken. macWTF knows where each backend writes, so it should look
// there rather than depend on the user's shell being configured first.
func (e *Executor) stepEnv() []string {
	e.envOnce.Do(func() {
		dirs := pathenv.Detect(map[manifest.Backend]bool{
			manifest.BackendBrew: true, manifest.BackendCask: true,
			manifest.BackendPipx: true, manifest.BackendCargo: true,
			manifest.BackendGo: true, manifest.BackendNPM: true,
		})
		var prefix []string
		for _, d := range dirs {
			if d.Exists && !d.OnPath {
				prefix = append(prefix, d.Dir)
			}
		}
		env := os.Environ()
		if len(prefix) > 0 {
			env = append(env, "PATH="+strings.Join(prefix, ":")+":"+os.Getenv("PATH"))
		}
		e.env = env
	})
	return e.env
}

// runStep runs one command, streaming its output as events.
func (e *Executor) runStep(ctx context.Context, step backend.Step, tool *manifest.Tool, index, total int) error {
	cmd := step.Cmd()
	cmd.Env = e.stepEnv()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	stream := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r\n")
			if line == "" {
				continue
			}
			e.emit(Event{
				Kind: EventOutput, Tool: tool, Step: step,
				Index: index, Total: total, Line: line,
			})
		}
	}
	wg.Add(2)
	go stream(stdout)
	go stream(stderr)

	// Kill the command if the context is cancelled, so ctrl-c does not
	// leave a brew install running detached.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()

	wg.Wait()
	err = cmd.Wait()
	close(done)
	return err
}
