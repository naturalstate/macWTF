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
	return tui.Run(cat, ctx)
}
