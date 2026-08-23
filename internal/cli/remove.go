package cli

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
	"github.com/naturalstate/macWTF/internal/state"
	"github.com/naturalstate/macWTF/internal/sudo"
)

// runStatus reports what macWTF has done to this machine.
func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}
	st, err := state.Load("")
	if err != nil {
		return err
	}

	if len(st.Installed) == 0 {
		fmt.Println("macWTF has not installed anything on this machine.")
		fmt.Printf("(state file: %s)\n", st.Path())
		return nil
	}

	var mine, prior, failed []state.Record
	for _, r := range st.Installed {
		switch {
		case r.Failed:
			failed = append(failed, r)
		case r.Preexisting:
			prior = append(prior, r)
		default:
			mine = append(mine, r)
		}
	}
	for _, s := range [][]state.Record{mine, prior, failed} {
		sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	}

	name := func(r state.Record) string {
		if t, ok := cat.Tool(r.ID); ok {
			return t.Name
		}
		return r.ID
	}

	if len(mine) > 0 {
		fmt.Printf("\ninstalled by macWTF (%d) — these are what `remove` will take\n\n", len(mine))
		for _, r := range mine {
			q := ""
			if r.QuarantineStripped {
				q = warnText.Render("  [quarantine removed]")
			}
			fmt.Printf("  %-22s %-7s %s%s\n", r.ID, r.Backend,
				dimText.Render(name(r)), q)
		}
	}

	// Reported separately and never removed: taking something the user had
	// before any of this ran would be indefensible.
	if len(prior) > 0 {
		fmt.Printf("\nalready present before macWTF ran (%d) — left alone by `remove`\n\n", len(prior))
		for _, r := range prior {
			fmt.Printf("  %-22s %-7s %s\n", r.ID, r.Backend, dimText.Render(name(r)))
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nfailed (%d) — re-running will retry these\n\n", len(failed))
		for _, r := range failed {
			fmt.Printf("  %-22s %-7s %s\n", r.ID, r.Backend, dimText.Render(r.Error))
		}
	}

	fmt.Printf("\n%d installed by macWTF, %d already present, %d failed\n",
		len(mine), len(prior), len(failed))
	fmt.Printf("state: %s\n", st.Path())
	return nil
}

func runRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	profile := fs.String("profile", "", "remove a named profile")
	category := fs.String("category", "", "remove every tool in a category")
	var tools stringList
	fs.Var(&tools, "tool", "remove a single tool (repeatable)")
	all := fs.Bool("all", false, "remove everything macWTF installed")
	dryRun := fs.Bool("dry-run", false, "print every command without executing anything")
	assumeYes := fs.Bool("yes", false, "do not ask for confirmation")
	force := fs.Bool("force", false,
		"remove even tools macWTF did not install, or has no record of")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}
	st, err := state.Load("")
	if err != nil {
		return err
	}
	ctx, err := backend.NewCtx()
	if err != nil {
		return err
	}
	reg := backend.NewRegistry()

	var targets []*manifest.Tool

	if *all {
		// Everything in state that macWTF put there, in reverse
		// dependency order so a tool goes before what it required.
		for _, r := range st.Installed {
			if r.Preexisting && !*force {
				continue
			}
			if t, ok := cat.Tool(r.ID); ok {
				targets = append(targets, t)
			}
		}
		reverse(targets)
	} else {
		supported := map[manifest.Backend]bool{}
		for b := range reg {
			supported[b] = true
		}
		res, err := resolve.Resolve(cat, resolve.Request{
			Profile: *profile, Category: *category, Tools: tools,
			Arch: ctx.Arch, SupportedBackends: supported,
		})
		if err != nil {
			return err
		}
		targets = append(targets, res.Install...)
		reverse(targets)
	}

	if len(targets) == 0 {
		fmt.Println("Nothing selected to remove.")
		return nil
	}

	plan, err := install.BuildRemovePlan(targets, reg, ctx, st, *force)
	if err != nil {
		return err
	}

	var out strings.Builder
	plan.Render(&out, *dryRun)
	fmt.Print(out.String())

	// Clear records for anything already gone, even when there is nothing
	// to run. Otherwise the stale entries persist and every later run
	// reports the same phantom failures.
	if !*dryRun && len(plan.AlreadyGone) > 0 {
		for _, t := range plan.AlreadyGone {
			st.Remove(t.ID)
		}
		if err := st.Save(); err != nil {
			return err
		}
	}

	todo, _ := plan.Counts()
	if *dryRun || todo == 0 {
		if len(plan.AlreadyGone) > 0 {
			fmt.Printf("\nCleared %d stale record(s).\n", len(plan.AlreadyGone))
		}
		return nil
	}

	if !*assumeYes && !confirm(fmt.Sprintf("Remove %d tool(s)?", todo)) {
		return fmt.Errorf("cancelled")
	}

	// Cask removal writes to /Applications and can install or remove launch
	// daemons. Priming covers most of it, but Homebrew invokes sudo itself
	// for some casks and macOS will ask again regardless of our timestamp,
	// so say so rather than letting an unexplained prompt look like a hang.
	if casksInPlan(plan) {
		fmt.Println()
		fmt.Println(dimText.Render(
			"Some casks ask for an administrator password of their own during removal. " +
				"If a prompt appears mid-run, answer it and the run continues."))
		if sudo.Available() && !sudo.Active() {
			_ = sudo.Prime()
		}
	}
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sudo.KeepAlive(runCtx)

	what := "remove — " + runLabel(*profile, *category, tools)
	if *all {
		what = "remove — everything macWTF installed"
	}
	log := install.NewRunLog(ctx.StateDir, what)
	defer log.Close()

	runner := newRunner(todo)
	ex := &install.Executor{Ctx: ctx, State: st, Emit: func(ev install.Event) {
		log.Event(ev)
		runner.handle(ev)
	}}

	result, err := ex.Run(runCtx, plan.AsToolPlans())
	runner.finish()
	if err != nil {
		return err
	}

	// Drop the records for everything that actually went.
	for _, t := range result.Installed {
		st.Remove(t.ID)
	}
	if err := st.Save(); err != nil {
		return err
	}

	fmt.Printf("\n%d removed", len(result.Installed))
	if n := len(result.Failed); n > 0 {
		fmt.Printf(", %d failed", n)
		for _, f := range result.Failed {
			fmt.Printf("\n  %-20s %v", f.Tool.ID, f.Err)
		}
	}
	fmt.Printf("\nState: %s\n", st.Path())
	if p := log.Path(); p != "" {
		fmt.Printf("Full output: %s\n", p)
	}
	return nil
}

// casksInPlan reports whether anything in the plan is a cask.
func casksInPlan(p *install.RemovePlan) bool {
	for _, tp := range p.Tools {
		if tp.Tool.Backend == manifest.BackendCask {
			return true
		}
	}
	return false
}

func reverse(ts []*manifest.Tool) {
	for i, j := 0, len(ts)-1; i < j; i, j = i+1, j-1 {
		ts[i], ts[j] = ts[j], ts[i]
	}
}
