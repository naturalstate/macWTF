package cli

import (
	"fmt"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/resolve"
)

// offerBackendInstall offers to install the package managers a plan needs but
// cannot find, and returns a rebuilt plan if any were installed.
//
// Returns (nil, nil) when there is nothing to do, so the caller keeps the plan
// it already has.
func offerBackendInstall(
	plan *install.Plan,
	res *resolve.Result,
	reg backend.Registry,
	ctx *backend.Ctx,
	assumeYes bool,
) (*install.Plan, error) {
	missing := plan.MissingBackends()
	if len(missing) == 0 {
		return nil, nil
	}

	fixable := backend.Provisionable(missing)
	if len(fixable) == 0 {
		return nil, nil
	}

	// Count what is actually being held up, since that is the reason to
	// care rather than the missing tool itself.
	blocked := map[string]int{}
	for _, tp := range plan.Tools {
		if tp.PlanErr == nil {
			continue
		}
		if _, dead := plan.BackendErrs[tp.Tool.Backend]; dead {
			blocked[string(tp.Tool.Backend)]++
		}
	}

	fmt.Println()
	fmt.Println(warnText.Render("Some tools need a package manager you do not have."))
	fmt.Println()
	for _, p := range fixable {
		fmt.Printf("  %-6s %s\n", p.Backend,
			dimText.Render(fmt.Sprintf("%d tool(s) blocked — %s", blocked[string(p.Backend)], p.Note)))
	}
	fmt.Println()
	fmt.Println("macWTF can install them with Homebrew first:")
	fmt.Printf("\n    %s\n\n", backend.ProvisionCommand(fixable))
	fmt.Println(dimText.Render("These are ordinary formulae installed as you, not a privileged installer."))
	fmt.Println()

	if !assumeYes && !confirm("Install them now?") {
		fmt.Println("\nSkipped. Those tools will be reported as not attempted.")
		return nil, nil
	}

	fmt.Println()
	if err := reg.Install(fixable, func(line string) {
		fmt.Printf("  %s\n", dimText.Render(truncateLine(line, terminalWidth()-4)))
	}); err != nil {
		fmt.Println(warnText.Render("Could not install them: " + err.Error()))
		return nil, nil
	}

	// The installed-package snapshots and the preflight results were both
	// taken before these existed, so both have to be discarded.
	ctx.ResetInstalledCache()

	rebuilt, err := install.BuildPlan(res, reg, ctx)
	if err != nil {
		return nil, err
	}

	if still := rebuilt.MissingBackends(); len(still) > 0 {
		fmt.Println()
		fmt.Println(warnText.Render(
			"Installed, but still not on PATH for this run. Open a new terminal and re-run."))
		for _, b := range still {
			fmt.Printf("  %s: %v\n", b, rebuilt.BackendErrs[b])
		}
	}

	todo, _, _ := rebuilt.Counts()
	fmt.Printf("\n%s\n", okMark.Render(fmt.Sprintf("Package managers installed — %d tool(s) now installable.", todo)))
	return rebuilt, nil
}
