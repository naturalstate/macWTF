// Package install turns a resolved selection into an executable plan, and runs
// it.
//
// The separation matters: BuildPlan is pure apart from asking each backend what
// is already installed, so what --dry-run prints is byte-for-byte the same plan
// a real install executes. Dry-run is not a second code path.
package install

import (
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
)

// ToolPlan is the work for a single tool.
type ToolPlan struct {
	Tool  *manifest.Tool
	Steps []backend.Step

	// AlreadyInstalled short-circuits the steps. Re-running a profile on a
	// configured machine must be a no-op, not a reinstall.
	AlreadyInstalled bool

	// PlanErr records a backend that could not produce a plan. Non-fatal:
	// recorded, reported, and the run continues.
	PlanErr error
}

// Plan is a whole run.
type Plan struct {
	Tools   []ToolPlan
	Skipped []resolve.Skipped

	// BackendErrs records backends that failed preflight, keyed by name.
	// Their tools are reported rather than silently dropped.
	BackendErrs map[manifest.Backend]error
}

// Counts summarises a plan for display.
func (p *Plan) Counts() (todo, already, failed int) {
	for _, tp := range p.Tools {
		switch {
		case tp.PlanErr != nil:
			failed++
		case tp.AlreadyInstalled:
			already++
		default:
			todo++
		}
	}
	return
}

// BuildPlan produces the full command sequence for a resolved selection.
//
// A backend that fails preflight does not abort the run: its tools are marked
// with the error and everything else proceeds. One missing package manager
// should not cost the user the other fifty tools.
func BuildPlan(res *resolve.Result, reg backend.Registry, ctx *backend.Ctx) (*Plan, error) {
	p := &Plan{
		Skipped:     res.Skipped,
		BackendErrs: map[manifest.Backend]error{},
	}

	preflighted := map[manifest.Backend]bool{}

	for _, t := range res.Install {
		tp := ToolPlan{Tool: t}

		impl, err := reg.Get(t.Backend)
		if err != nil {
			tp.PlanErr = err
			p.Tools = append(p.Tools, tp)
			continue
		}

		// Preflight each backend once, not once per tool.
		if !preflighted[t.Backend] {
			preflighted[t.Backend] = true
			if err := impl.Preflight(ctx); err != nil {
				p.BackendErrs[t.Backend] = err
			}
		}
		if err, dead := p.BackendErrs[t.Backend]; dead {
			tp.PlanErr = err
			p.Tools = append(p.Tools, tp)
			continue
		}

		installed, err := ctx.InstalledFor(impl)
		if err != nil {
			tp.PlanErr = fmt.Errorf("query installed packages: %w", err)
			p.Tools = append(p.Tools, tp)
			continue
		}
		if installed[t.Package] {
			tp.AlreadyInstalled = true
			p.Tools = append(p.Tools, tp)
			continue
		}

		steps, err := impl.InstallPlan(t, ctx)
		if err != nil {
			tp.PlanErr = err
			p.Tools = append(p.Tools, tp)
			continue
		}
		tp.Steps = steps
		p.Tools = append(p.Tools, tp)
	}

	return p, nil
}

// PendingQuarantine lists tools in the plan that need a quarantine strip which
// has not been authorised. Used to prompt once, up front, rather than
// interrupting the run repeatedly.
func (p *Plan) PendingQuarantine() []*manifest.Tool {
	var out []*manifest.Tool
	for _, tp := range p.Tools {
		if tp.AlreadyInstalled || tp.PlanErr != nil {
			continue
		}
		if !tp.Tool.QuarantineStrip || tp.Tool.AppPath == "" {
			continue
		}
		var planned bool
		for _, s := range tp.Steps {
			if s.Kind == backend.KindQuarantine {
				planned = true
			}
		}
		if !planned {
			out = append(out, tp.Tool)
		}
	}
	return out
}

// Render writes the plan as human-readable text. This is what --dry-run
// prints: every command, in order, exactly as it would run.
func (p *Plan) Render(w *strings.Builder, dryRun bool) {
	todo, already, failed := p.Counts()

	header := "install plan"
	if dryRun {
		header = "dry run — nothing will be executed"
	}
	fmt.Fprintf(w, "\n%s\n%s\n\n", header, strings.Repeat("─", len(header)))

	// A backend that failed preflight would otherwise repeat the same error
	// once per tool. Say it once, at the top, where it can be acted on.
	if len(p.BackendErrs) > 0 {
		for b, err := range p.BackendErrs {
			fmt.Fprintf(w, "  %s backend unavailable: %v\n", b, err)
		}
		fmt.Fprintf(w, "  run `macwtf doctor` to see what is missing\n\n")
	}

	for _, tp := range p.Tools {
		switch {
		case tp.PlanErr != nil:
			if _, dead := p.BackendErrs[tp.Tool.Backend]; dead {
				fmt.Fprintf(w, "  %-14s  blocked (%s unavailable)\n", tp.Tool.ID, tp.Tool.Backend)
				continue
			}
			fmt.Fprintf(w, "  %-14s  cannot plan: %v\n", tp.Tool.ID, tp.PlanErr)
		case tp.AlreadyInstalled:
			fmt.Fprintf(w, "  %-14s  already installed, skipping\n", tp.Tool.ID)
		default:
			fmt.Fprintf(w, "  %s\n", tp.Tool.ID)
			for _, s := range tp.Steps {
				marker := " "
				if s.Kind == backend.KindQuarantine {
					marker = "!"
				}
				fmt.Fprintf(w, "   %s  %s\n", marker, s.String())
			}
		}
	}

	if len(p.Skipped) > 0 {
		fmt.Fprintf(w, "\nskipped\n")
		for _, s := range p.Skipped {
			fmt.Fprintf(w, "  %-14s  %s — %s\n", s.Tool.ID, s.Reason, s.Detail)
		}
	}

	fmt.Fprintf(w, "\n%d to install", todo)
	if already > 0 {
		fmt.Fprintf(w, ", %d already present", already)
	}
	if failed > 0 {
		fmt.Fprintf(w, ", %d cannot be planned", failed)
	}
	if len(p.Skipped) > 0 {
		fmt.Fprintf(w, ", %d skipped", len(p.Skipped))
	}
	w.WriteString("\n")

	if pending := p.PendingQuarantine(); len(pending) > 0 {
		fmt.Fprintf(w, "\n%d tool(s) are unsigned and will not launch until the Gatekeeper\n", len(pending))
		w.WriteString("quarantine attribute is removed:\n")
		for _, t := range pending {
			fmt.Fprintf(w, "  %-14s  xattr -d -r com.apple.quarantine %s\n", t.ID, t.AppPath)
		}
		w.WriteString("\nRe-run with --allow-quarantine-strip to include those steps.\n")
		w.WriteString("This waives a macOS malware check for those specific apps.\n")
	}
}
