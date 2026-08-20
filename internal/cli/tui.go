package cli

import (
	"flag"
	"fmt"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/tui"
)

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}
	if err := cat.Validate(); err != nil {
		return fmt.Errorf("catalogue is invalid:\n%w", err)
	}

	ctx, err := backend.NewCtx()
	if err != nil {
		return err
	}
	// Preview only: the plan is built against real installed state when
	// Homebrew is present, and against an empty set when it is not, so the
	// interface works on a machine with no brew at all.
	if _, lookErr := backend.NewRegistry().Get(manifest.BackendBrew); lookErr == nil && ctx.BrewPrefix == "" {
		ctx.SeedInstalled(manifest.BackendBrew, map[string]bool{})
		ctx.SeedInstalled(manifest.BackendCask, map[string]bool{})
	}

	return tui.Run(cat, ctx)
}
