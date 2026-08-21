package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/state"
)

// eventMsg carries one executor progress event into the bubbletea loop.
type eventMsg install.Event

// doneMsg ends the run.
type doneMsg struct {
	result *install.Result
	err    error
}

// tickMsg drives the spinner while a command is running with no output.
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// waitForEvent blocks on the executor's event channel. Re-issued after every
// event so the stream keeps flowing.
func waitForEvent(ch <-chan install.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(ev)
	}
}

// startInstall builds the plan for the current selection and runs it.
//
// The executor runs on its own goroutine and reports through a channel, so the
// interface stays responsive and can show output as it arrives rather than
// freezing until the run finishes.
func (m *Model) startInstall() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		m.runErr = errNothingSelected
		return nil
	}

	st, err := state.Load("")
	if err != nil {
		m.runErr = err
		return nil
	}

	// Rebuild the plan against the consent just given, so an approved
	// quarantine strip actually appears in the steps.
	m.ctx.AllowQuarantineStrip = m.allowQuarantine
	plan, err := m.resolvePlan()
	if err != nil {
		m.runErr = err
		return nil
	}

	todo, _, _ := plan.Counts()
	m.runTotal = todo
	m.runDone = 0
	m.runLog = nil
	m.runStatus = ""
	m.screen = screenProgress
	m.startedAt = time.Now()

	ch := make(chan install.Event, 64)
	m.events = ch

	runCtx, cancel := context.WithCancel(context.Background())
	m.cancelRun = cancel

	ex := &install.Executor{
		Ctx:   m.ctx,
		State: st,
		Emit:  func(ev install.Event) { ch <- ev },
	}

	go func() {
		res, err := ex.Run(runCtx, plan)
		close(ch)
		m.doneCh <- doneMsg{result: res, err: err}
	}()

	return tea.Batch(waitForEvent(ch), tick(), waitForDone(m.doneCh))
}

func waitForDone(ch <-chan doneMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// resolvePlan builds an install plan from the current selection.
func (m *Model) resolvePlan() (*install.Plan, error) {
	reg := backend.NewRegistry()
	supported := map[manifest.Backend]bool{}
	for b := range reg {
		supported[b] = true
	}

	res, err := resolveSelection(m, supported)
	if err != nil {
		return nil, err
	}
	return install.BuildPlan(res, reg, m.ctx)
}

// handleEvent folds an executor event into the model.
func (m *Model) handleEvent(ev install.Event) {
	switch ev.Kind {
	case install.EventToolStart:
		m.runCurrent = ev.Tool.ID
		m.runRecent = m.runRecent[:0]

	case install.EventStepStart:
		m.runStatus = ev.Step.Desc

	case install.EventOutput:
		m.runStatus = ev.Line
		m.runRecent = append(m.runRecent, ev.Line)
		if len(m.runRecent) > 12 {
			m.runRecent = m.runRecent[len(m.runRecent)-12:]
		}

	case install.EventToolDone:
		m.runDone++
		entry := logEntry{tool: ev.Tool.ID, elapsed: ev.Elapsed}
		if ev.Err != nil {
			entry.err = ev.Err
			entry.output = append([]string(nil), m.runRecent...)
		}
		m.runLog = append(m.runLog, entry)
		m.runStatus = ""
	}
}

// logEntry is one finished tool, kept as a permanent record on the progress
// screen while transient command output scrolls past.
type logEntry struct {
	tool    string
	elapsed time.Duration
	err     error
	output  []string
}
