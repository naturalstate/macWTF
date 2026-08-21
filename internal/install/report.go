package install

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// ManualStep is one thing a human has to do that no installer can.
type ManualStep struct {
	Tool *manifest.Tool

	// Pane is the System Settings location for a TCC permission, empty for
	// a free-form step.
	Pane manifest.Pane

	// Text is the instruction.
	Text string
}

// ManualSteps collects everything the run could not do automatically.
//
// This is the highest-value output macWTF produces. macOS will not let any
// installer grant Full Disk Access, Screen Recording, Accessibility or Input
// Monitoring — without MDM a human has to click them — so the alternative to
// this report is the user discovering, one broken app at a time, that the tool
// they installed does nothing.
//
// Collected only for tools that actually installed: telling someone to grant
// permissions to an app that failed to install would be noise.
func (r *Result) ManualSteps() []ManualStep {
	var out []ManualStep

	for _, t := range r.Installed {
		for _, p := range t.TCCPermissions {
			out = append(out, ManualStep{
				Tool: t,
				Pane: p.Pane(),
				Text: fmt.Sprintf("Grant %s in %s", t.Name, p.Pane().Name),
			})
		}
		for _, s := range t.ManualSteps {
			out = append(out, ManualStep{Tool: t, Text: s})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		// TCC permissions first: they are the ones that make an app
		// silently useless until granted.
		iTCC := out[i].Pane.Name != ""
		jTCC := out[j].Pane.Name != ""
		if iTCC != jTCC {
			return iTCC
		}
		return out[i].Tool.ID < out[j].Tool.ID
	})
	return out
}

// RenderSummary writes the end-of-run report.
func (r *Result) RenderSummary(w *strings.Builder) {
	w.WriteString("\n")
	w.WriteString(strings.Repeat("─", 64) + "\n")
	fmt.Fprintf(w, "Done in %s\n\n", roundDuration(r.Elapsed))

	if n := len(r.Installed); n > 0 {
		fmt.Fprintf(w, "  %d installed\n", n)
	}
	if n := len(r.Skipped); n > 0 {
		fmt.Fprintf(w, "  %d already present\n", n)
	}
	if n := len(r.Failed); n > 0 {
		fmt.Fprintf(w, "  %d failed\n", n)
	}

	if len(r.Failed) > 0 {
		w.WriteString("\nfailed\n")
		for _, f := range r.Failed {
			fmt.Fprintf(w, "  %-14s %v\n", f.Tool.ID, f.Err)
		}
		w.WriteString("\nThese were skipped and the run continued. Re-running will retry them.\n")
	}

	if len(r.QuarantineStripped) > 0 {
		w.WriteString("\nGatekeeper quarantine was removed from:\n")
		for _, t := range r.QuarantineStripped {
			fmt.Fprintf(w, "  %-14s %s\n", t.ID, t.AppPath)
		}
		w.WriteString("These apps are unsigned and now bypass macOS malware checking.\n")
	}

	steps := r.ManualSteps()
	if len(steps) == 0 {
		return
	}

	w.WriteString("\n")
	w.WriteString(strings.Repeat("═", 64) + "\n")
	fmt.Fprintf(w, "%d MANUAL STEPS REMAIN\n", len(steps))
	w.WriteString(strings.Repeat("═", 64) + "\n\n")
	w.WriteString("macOS does not allow an installer to grant these. Until you do,\n")
	w.WriteString("the affected tools will launch but not work.\n\n")

	for i, s := range steps {
		fmt.Fprintf(w, "  %2d. %s\n", i+1, s.Text)
		if s.Pane.URL != "" {
			fmt.Fprintf(w, "      open '%s'\n", s.Pane.URL)
		}
		w.WriteString("\n")
	}
}

// roundDuration trims a duration to something worth reading. Nobody needs
// nanoseconds after a package install.
func roundDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(100 * time.Millisecond).String()
	default:
		return d.Round(time.Second).String()
	}
}

// ProgressBar renders a fixed-width bar. Used by the CLI runner and the TUI so
// both show the same thing.
func ProgressBar(done, total, width int) string {
	if total <= 0 {
		return strings.Repeat("░", width)
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
