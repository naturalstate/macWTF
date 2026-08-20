package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
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

	var out strings.Builder
	plan.Render(&out, *dryRun)
	fmt.Print(out.String())

	if *dryRun {
		return nil
	}

	return fmt.Errorf("real installs are not wired up yet — re-run with --dry-run")
}
