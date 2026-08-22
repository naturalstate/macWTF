package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/bootstrap"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/pathenv"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	rep := bootstrap.Check()
	var b strings.Builder
	rep.Render(&b)
	fmt.Print(b.String())

	// Report bin directories that exist but are unreachable, for every
	// backend macWTF can install through — not just the ones used so far.
	all := map[manifest.Backend]bool{
		manifest.BackendBrew: true, manifest.BackendCask: true,
		manifest.BackendPipx: true, manifest.BackendCargo: true,
		manifest.BackendGo: true,
	}
	if missing := pathenv.Missing(pathenv.Detect(all)); len(missing) > 0 {
		fmt.Println("\nInstalled tools that are not on your PATH:")
		for _, e := range missing {
			fmt.Printf("  %-22s %s\n", e.Dir, "("+string(e.Backend)+")")
			fmt.Printf("    %s\n", e.Line)
		}
	}

	if !rep.OK() {
		return fmt.Errorf("%d prerequisite(s) missing", len(rep.Blocking()))
	}
	return nil
}
