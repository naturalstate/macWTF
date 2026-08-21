package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/bootstrap"
)

func runBootstrap(args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "reserved; prerequisites are never installed automatically")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return ensurePrerequisites(*yes)
}

// ensurePrerequisites verifies the machine can run installs. It reports what is
// missing and how to fix it, but never installs anything itself.
func ensurePrerequisites(_ bool) error {
	// Homebrew may already be installed but absent from PATH — the state
	// immediately after installing it in the same session.
	if prefix, adopted := bootstrap.AdoptBrewPath(); adopted {
		fmt.Printf("Found Homebrew at %s (not on your PATH).\n", prefix)
		fmt.Printf("Add this to your shell profile to make that permanent:\n\n    %s\n\n",
			bootstrap.ShellEnvHint(prefix))
	}

	rep := bootstrap.Check()
	var b strings.Builder
	rep.Render(&b)
	fmt.Print(b.String())

	if rep.OK() {
		return nil
	}

	// Installing Homebrew is the user's responsibility. macWTF reports what
	// is missing and the exact command that fixes it, then stops. Wrapping
	// somebody else's privileged installer is not worth owning the failure
	// modes, and the user should decide what runs as root on their machine.
	return fmt.Errorf("%d prerequisite(s) missing — see above, then re-run", len(rep.Blocking()))
}
