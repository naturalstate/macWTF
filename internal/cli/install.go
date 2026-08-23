package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/bootstrap"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
	"github.com/naturalstate/macWTF/internal/state"
	"github.com/naturalstate/macWTF/internal/sudo"
)

// stringList collects a repeatable flag, so --tool can be passed more than once.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	profile := fs.String("profile", "", "install a named profile")
	category := fs.String("category", "", "install every tool in a category")
	var tools stringList
	fs.Var(&tools, "tool", "install a single tool (repeatable)")
	dryRun := fs.Bool("dry-run", false, "print every command without executing anything")
	assumeYes := fs.Bool("yes", false, "do not ask for confirmation")
	allowQuarantine := fs.Bool("allow-quarantine-strip", false,
		"permit removing com.apple.quarantine from unsigned apps (waives a Gatekeeper malware check)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}
	if err := cat.Validate(); err != nil {
		return fmt.Errorf("catalogue is invalid, refusing to install:\n%w", err)
	}

	// A dry run only describes what would happen, so it must not require or
	// trigger an install of anything. A real run brings the machine up to
	// scratch first, asking before anything privileged happens.
	if !*dryRun {
		if err := ensurePrerequisites(*assumeYes); err != nil {
			return err
		}
	} else if prefix, adopted := backendAdoptBrew(); adopted {
		fmt.Printf("note: using Homebrew found at %s (not on PATH)\n", prefix)
	}

	ctx, err := backend.NewCtx()
	if err != nil {
		return err
	}
	ctx.DryRun = *dryRun
	ctx.AllowQuarantineStrip = *allowQuarantine

	reg := backend.NewRegistry()
	supported := map[manifest.Backend]bool{}
	for b := range reg {
		supported[b] = true
	}

	res, err := resolve.Resolve(cat, resolve.Request{
		Profile:           *profile,
		Category:          *category,
		Tools:             tools,
		Arch:              ctx.Arch,
		SupportedBackends: supported,
	})
	if err != nil {
		return err
	}

	plan, err := install.BuildPlan(res, reg, ctx)
	if err != nil {
		return err
	}

	// A missing package manager is usually one brew install away. Offering
	// it here, and rebuilding the plan afterwards, means the tools it
	// blocks install in this run rather than after the user goes away,
	// reads an error, installs something and comes back.
	if !*dryRun {
		if newPlan, err := offerBackendInstall(plan, res, reg, ctx, *assumeYes); err != nil {
			return err
		} else if newPlan != nil {
			plan = newPlan
		}
	}

	var out strings.Builder
	plan.Render(&out, *dryRun)
	fmt.Print(out.String())

	if *dryRun {
		return nil
	}

	todo, _, _ := plan.Counts()
	if todo == 0 {
		fmt.Println("Nothing to do.")
		return nil
	}

	// Quarantine stripping waives a Gatekeeper malware check, so it is
	// confirmed separately from the install itself, immediately before
	// anything runs, and never inferred from the user having said yes to
	// installing.
	if pending := plan.PendingQuarantine(); len(pending) > 0 && !*allowQuarantine {
		if !confirmQuarantine(pending, *assumeYes) {
			fmt.Println("Continuing without stripping quarantine.")
			fmt.Println("Those apps will install but Gatekeeper will block first launch.")
		} else {
			ctx.AllowQuarantineStrip = true
			plan, err = install.BuildPlan(res, reg, ctx)
			if err != nil {
				return err
			}
		}
	}

	if !*assumeYes && !confirm(fmt.Sprintf("Install %d tool(s)?", todo)) {
		return fmt.Errorf("cancelled")
	}

	// Ask for the password now, at a predictable moment, rather than
	// letting a cask demand it partway through the run.
	if needs := plan.MayNeedSudo(); len(needs) > 0 && !sudo.Active() {
		fmt.Printf("\n%d cask(s) may need an administrator password.\n",
			len(needs))
		fmt.Println(dimText.Render("Homebrew refuses to run as root, so macOS asks per-package. " +
			"Authorising once now avoids a prompt interrupting the run."))
		fmt.Println()
		if err := sudo.Prime(); err != nil {
			fmt.Println(warnText.Render("Continuing without authorisation — " +
				"anything needing it will prompt or fail."))
		}
	}

	runCtx, cancelKeepAlive := context.WithCancel(context.Background())
	defer cancelKeepAlive()
	sudo.KeepAlive(runCtx)

	st, err := state.Load("")
	if err != nil {
		return err
	}

	what := runLabel(*profile, *category, tools)
	log := install.NewRunLog(ctx.StateDir, what)
	defer log.Close()

	runner := newRunner(todo)
	ex := &install.Executor{Ctx: ctx, State: st, Emit: func(ev install.Event) {
		// The log gets everything; the screen gets what fits.
		log.Event(ev)
		runner.handle(ev)
	}}

	result, err := ex.Run(runCtx, plan)
	runner.finish()
	if err != nil {
		return err
	}

	var summary strings.Builder
	summary.WriteString("\n" + boldText.Render(what))
	result.RenderSummary(&summary)
	fmt.Print(summary.String())

	if p := log.Path(); p != "" {
		fmt.Printf("\nFull output of every command: %s\n", p)
	}

	offerPathFix(result, *assumeYes)

	fmt.Printf("\nState written to %s\n", st.Path())

	if len(result.Failed) > 0 {
		return fmt.Errorf("%d tool(s) failed", len(result.Failed))
	}
	return nil
}

// backendAdoptBrew exposes PATH adoption to the install flow.
func backendAdoptBrew() (string, bool) { return bootstrap.AdoptBrewPath() }

// runLabel describes what a run was asked to do, so the summary and the log
// both say which profile or category this was. Finishing a long run and not
// being told what it was for is a small thing that makes the output useless
// twenty minutes later.
func runLabel(profile, category string, tools []string) string {
	switch {
	case profile != "":
		return "profile: " + profile
	case category != "":
		return "category: " + category
	case len(tools) == 1:
		return "tool: " + tools[0]
	case len(tools) > 1:
		return fmt.Sprintf("%d tools: %s", len(tools), strings.Join(tools, ", "))
	}
	return "install"
}
