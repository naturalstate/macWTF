package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/naturalstate/macWTF/internal/state"
)

// runReset is the development loop: put the machine back so a scenario can be
// tested again from a known starting point.
//
// Exists because testing an installer means installing repeatedly, and doing
// that by hand means guessing at what the last run left behind. It is a thin
// wrapper over `remove --all` plus discarding the state file, but having it as
// one obvious command is the difference between testing and messing about.
func runReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	dryRun := fs.Bool("dry-run", false, "print every command without executing anything")
	assumeYes := fs.Bool("yes", false, "do not ask for confirmation")
	keepState := fs.Bool("keep-state", false, "remove the tools but leave the state file")
	stateOnly := fs.Bool("state-only", false,
		"forget everything without uninstalling, for when the machine was reset another way")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := state.Load("")
	if err != nil {
		return err
	}

	removable := st.Removable()
	if *stateOnly {
		if !*assumeYes && !confirm(
			fmt.Sprintf("Forget %d record(s) without uninstalling anything?", len(st.Installed))) {
			return fmt.Errorf("cancelled")
		}
		if err := os.Remove(st.Path()); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("Removed %s. Nothing was uninstalled.\n", st.Path())
		return nil
	}

	if len(removable) == 0 {
		fmt.Println("Nothing to reset: macWTF has not installed anything.")
		return nil
	}

	fmt.Printf("\nReset will uninstall %d tool(s) macWTF installed.\n", len(removable))
	fmt.Println(dimText.Render("Tools that were already present before macWTF ran are left alone."))

	// Hand off to remove, which owns the safety rules about what may be
	// taken. Duplicating them here would mean two places to get it wrong.
	removeArgs := []string{"--all"}
	if *dir != "" {
		removeArgs = append(removeArgs, "--manifest-dir", *dir)
	}
	if *dryRun {
		removeArgs = append(removeArgs, "--dry-run")
	}
	if *assumeYes {
		removeArgs = append(removeArgs, "--yes")
	}
	if err := runRemove(removeArgs); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}

	if !*keepState {
		if err := os.Remove(st.Path()); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("Forgot %s — the next run starts from scratch.\n", st.Path())
	}

	// Removing a package does not undo everything installing it did, and a
	// developer resetting between runs needs to know which of those will
	// still be true next time.
	fmt.Println()
	fmt.Println(warnText.Render("Not undone by a reset:"))
	fmt.Println("  · Homebrew dependencies pulled in automatically — brew autoremove")
	fmt.Println("  · permissions granted in System Settings")
	fmt.Println("  · configuration in your home directory, such as ~/.mitmproxy")
	fmt.Println("  · PATH lines added to your shell profile")
	fmt.Println("  · system preferences changed by the system-tweaks category")
	fmt.Println()
	fmt.Println(dimText.Render("For a genuinely clean slate, revert the VM to a snapshot."))
	return nil
}
