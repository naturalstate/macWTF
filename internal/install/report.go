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

	// Name what was installed rather than only counting it. After a long
	// run "37 already present" says nothing about what the machine now has.
	if len(r.Installed) > 0 {
		byCat := map[string][]string{}
		var order []string
		for _, t := range r.Installed {
			if _, seen := byCat[t.Category]; !seen {
				order = append(order, t.Category)
			}
			byCat[t.Category] = append(byCat[t.Category], t.ID)
		}
		for _, c := range order {
			fmt.Fprintf(w, "  %-16s %s\n", c, strings.Join(byCat[c], " "))
		}
		w.WriteString("\n")
	}

	if n := len(r.Installed); n > 0 {
		fmt.Fprintf(w, "  %d installed\n", n)
	}
	if n := len(r.Skipped); n > 0 {
		fmt.Fprintf(w, "  %d already present\n", n)
	}
	if n := len(r.Failed); n > 0 {
		fmt.Fprintf(w, "  %d failed\n", n)
	}
	if n := len(r.Blocked); n > 0 {
		fmt.Fprintf(w, "  %d not attempted\n", n)
	}

	if len(r.Failed) > 0 {
		w.WriteString("\nfailed\n")
		for _, f := range r.Failed {
			fmt.Fprintf(w, "  %-14s %v\n", f.Tool.ID, f.Err)
		}
		w.WriteString("\nThese were attempted and did not succeed. Re-running will retry them.\n")
	}

	// Reported apart from failures, and grouped by what is missing, so the
	// fix is one install rather than a list of names to investigate.
	if len(r.Blocked) > 0 {
		byBackend := map[manifest.Backend][]string{}
		for _, b := range r.Blocked {
			byBackend[b.Tool.Backend] = append(byBackend[b.Tool.Backend], b.Tool.ID)
		}
		w.WriteString("\nnot attempted — the package manager they need is missing\n")
		for be, ids := range byBackend {
			fmt.Fprintf(w, "  %-6s %s\n", be, strings.Join(ids, ", "))
			if fix := backendFix(be); fix != "" {
				fmt.Fprintf(w, "         %s\n", fix)
			}
		}
		w.WriteString("\nNothing was installed for these, and nothing was broken by them.\n")
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

// backendFix is the command that makes a package manager available.
func backendFix(b manifest.Backend) string {
	switch b {
	case manifest.BackendPipx:
		return "brew install pipx"
	case manifest.BackendGo:
		return "brew install go"
	case manifest.BackendCargo:
		return "brew install rustup-init && rustup-init"
	case manifest.BackendNPM:
		return "brew install node"
	case manifest.BackendMAS:
		return "brew install mas"
	}
	return ""
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
