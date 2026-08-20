package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/naturalstate/macWTF/internal/bootstrap"
)

// confirm asks a yes/no question. Defaults to no: anything privileged should
// require a deliberate "yes", never an accidental RETURN.
func confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func runBootstrap(args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return ensurePrerequisites(*yes)
}

// ensurePrerequisites brings the machine up to the point where installs can
// run, asking before anything privileged happens. Returns nil once every
// required prerequisite is satisfied.
func ensurePrerequisites(assumeYes bool) error {
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

	needsBrew := false
	for _, s := range rep.Blocking() {
		switch s.Name {
		case "Homebrew":
			needsBrew = true
		case "Apple Silicon":
			// Not fixable. macWTF targets Apple Silicon only.
			return fmt.Errorf("%s: %s", s.Name, s.Detail)
		}
	}

	if !needsBrew {
		return fmt.Errorf("missing prerequisites cannot be installed automatically")
	}

	fmt.Println()
	fmt.Println("macWTF can install Homebrew for you. This will:")
	fmt.Println("  · download and execute Homebrew's official installer script")
	fmt.Println("  · ask for your administrator password")
	fmt.Println("  · install the Xcode Command Line Tools if they are missing")
	fmt.Println("  · write to /opt/homebrew")
	fmt.Println()
	fmt.Println("The exact command:")
	fmt.Printf("\n    %s\n\n", bootstrap.InstallCommand)

	if !assumeYes && !confirm("Install Homebrew now?") {
		fmt.Println("\nSkipped. Run the command above yourself, then re-run macwtf.")
		return fmt.Errorf("Homebrew is required")
	}

	fmt.Println("\nRunning Homebrew's installer. It will prompt for your password.")
	fmt.Println()
	if err := bootstrap.InstallHomebrew(); err != nil {
		return fmt.Errorf("Homebrew installation failed: %w", err)
	}

	prefix, _ := bootstrap.AdoptBrewPath()
	if prefix == "" {
		prefix = "/opt/homebrew"
	}

	fmt.Println()
	fmt.Println("Homebrew installed.")
	fmt.Printf("\nAdd this to your shell profile so brew stays on your PATH:\n\n    %s\n\n",
		bootstrap.ShellEnvHint(prefix))

	// Re-check rather than assuming success.
	if rep := bootstrap.Check(); !rep.OK() {
		var b strings.Builder
		rep.Render(&b)
		fmt.Print(b.String())
		return fmt.Errorf("prerequisites still unmet after installing Homebrew")
	}
	return nil
}
