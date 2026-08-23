package install

import (
	"fmt"
	"sort"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/state"
)

// RemoveSkip explains why a tool will not be removed.
type RemoveSkip struct {
	Tool   *manifest.Tool
	Reason string
}

// RemovePlan is a set of tools to uninstall.
type RemovePlan struct {
	Tools   []ToolPlan
	Skipped []RemoveSkip

	// AlreadyGone is tools that are not installed. Recorded so their state
	// entries can still be cleared: an interrupted removal leaves records
	// for things that are no longer there, and leaving them behind makes
	// every later run report phantom failures.
	AlreadyGone []*manifest.Tool

	BackendErrs map[manifest.Backend]error
}

// Counts summarises the plan.
func (p *RemovePlan) Counts() (todo, skipped int) {
	return len(p.Tools), len(p.Skipped)
}

// BuildRemovePlan works out what can be uninstalled.
//
// The governing rule is that macWTF removes only what macWTF installed. State
// records, per tool, whether the package was already present beforehand; a tool
// the user had for years before running any of this must survive a remove, or
// the tool is dangerous rather than merely useful. Force overrides that, and
// exists because a state file can be lost or predate a change.
func BuildRemovePlan(
	tools []*manifest.Tool,
	reg backend.Registry,
	ctx *backend.Ctx,
	st *state.State,
	force bool,
) (*RemovePlan, error) {
	p := &RemovePlan{BackendErrs: map[manifest.Backend]error{}}
	preflighted := map[manifest.Backend]bool{}

	for _, t := range tools {
		rec, known := st.Record(t.ID)

		switch {
		case !known && !force:
			p.Skipped = append(p.Skipped, RemoveSkip{t,
				"macWTF has no record of installing this"})
			continue
		case known && rec.Preexisting && !force:
			p.Skipped = append(p.Skipped, RemoveSkip{t,
				"was already installed before macWTF ran; left alone"})
			continue
		case known && rec.Failed && !force:
			p.Skipped = append(p.Skipped, RemoveSkip{t,
				"the install failed, so there is nothing to remove"})
			continue
		}

		impl, err := reg.Get(t.Backend)
		if err != nil {
			p.Skipped = append(p.Skipped, RemoveSkip{t, err.Error()})
			continue
		}

		if !preflighted[t.Backend] && !ctx.AssumeBackendsReady {
			preflighted[t.Backend] = true
			if err := impl.Preflight(ctx); err != nil {
				p.BackendErrs[t.Backend] = err
			}
		}
		if err, dead := p.BackendErrs[t.Backend]; dead {
			p.Skipped = append(p.Skipped, RemoveSkip{t, err.Error()})
			continue
		}

		// Removing something already gone is success, not failure: the
		// wanted end state is that it is absent. Without this an
		// interrupted removal is unrepeatable — the second attempt
		// reports every tool the first one removed as an error, which
		// is exactly what happened when a password prompt stopped a
		// reset partway through.
		present, err := backend.IsInstalled(impl, t, ctx)
		if err == nil && !present {
			p.AlreadyGone = append(p.AlreadyGone, t)
			continue
		}

		steps, err := impl.RemovePlan(t, ctx)
		if err != nil {
			p.Skipped = append(p.Skipped, RemoveSkip{t, err.Error()})
			continue
		}
		p.Tools = append(p.Tools, ToolPlan{Tool: t, Steps: steps})
	}

	sort.SliceStable(p.Skipped, func(i, j int) bool {
		return p.Skipped[i].Tool.ID < p.Skipped[j].Tool.ID
	})
	return p, nil
}

// Render writes the plan as text.
func (p *RemovePlan) Render(w *strings.Builder, dryRun bool) {
	header := "these commands will run"
	if dryRun {
		header = "dry run — nothing will be executed"
	}
	fmt.Fprintf(w, "\n%s\n%s\n\n", header, strings.Repeat("─", len(header)))

	for _, tp := range p.Tools {
		fmt.Fprintf(w, "  %s\n", tp.Tool.ID)
		for _, s := range tp.Steps {
			fmt.Fprintf(w, "      %s\n", s.String())
		}
	}

	if len(p.AlreadyGone) > 0 {
		fmt.Fprintf(w, "\nalready gone (%d) — nothing to do, records cleared\n", len(p.AlreadyGone))
		var ids []string
		for _, t := range p.AlreadyGone {
			ids = append(ids, t.ID)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(ids, " "))
	}

	if len(p.Skipped) > 0 {
		w.WriteString("\nleft alone\n")
		for _, s := range p.Skipped {
			fmt.Fprintf(w, "  %-20s %s\n", s.Tool.ID, s.Reason)
		}
	}

	todo, skipped := p.Counts()
	fmt.Fprintf(w, "\n%d to remove", todo)
	if skipped > 0 {
		fmt.Fprintf(w, ", %d left alone", skipped)
	}
	w.WriteString("\n")

	// Removing a package does not undo everything it did. Saying so is the
	// difference between an honest uninstall and one that quietly leaves
	// the machine changed.
	if todo > 0 {
		w.WriteString("\nThis removes packages only. It does not undo:\n")
		w.WriteString("  · dependencies pulled in automatically (see: brew autoremove)\n")
		w.WriteString("  · permissions granted in System Settings\n")
		w.WriteString("  · configuration and data in your home directory\n")
		w.WriteString("  · launch daemons a cask installed, such as Wireshark's ChmodBPF\n")
	}
}

// AsToolPlans exposes the removals for the executor.
func (p *RemovePlan) AsToolPlans() *Plan {
	return &Plan{Tools: p.Tools, BackendErrs: p.BackendErrs}
}
