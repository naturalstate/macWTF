package cli

import (
	"fmt"

	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/pathenv"
)

// offerPathFix tells the user when freshly installed tools are not reachable,
// and offers to fix it.
//
// Opt-in by construction: editing a shell profile is a stated non-goal unless
// explicitly asked for, so this shows the exact lines, defaults to no, and
// backs the file up before touching it. Declining leaves the machine exactly as
// the install found it.
func offerPathFix(result *install.Result, assumeYes bool) {
	if len(result.Installed) == 0 {
		return
	}

	used := map[manifest.Backend]bool{}
	for _, t := range result.Installed {
		used[t.Backend] = true
	}

	missing := pathenv.Missing(pathenv.Detect(used))
	if len(missing) == 0 {
		return
	}

	profile, err := pathenv.ProfilePath()
	if err != nil {
		return
	}

	fmt.Println()
	fmt.Println(warnText.Render("Some tools you just installed are not on your PATH."))
	fmt.Println("Running them by name will fail until this is fixed:")
	fmt.Println()
	for _, e := range missing {
		fmt.Printf("  %-22s %s\n", e.Dir, dimText.Render("("+string(e.Backend)+")"))
	}
	fmt.Println()
	fmt.Printf("macWTF can append these to %s:\n\n", profile)
	for _, e := range missing {
		fmt.Printf("    %s\n", e.Line)
	}
	fmt.Println()
	fmt.Println(dimText.Render("The file is backed up first, and lines already present are not repeated."))
	fmt.Println()

	if !assumeYes && !confirm(fmt.Sprintf("Add these to %s?", profile)) {
		fmt.Println("\nLeft alone. Add the lines above by hand whenever you like.")
		return
	}

	added, backup, err := pathenv.Append(profile, missing)
	if err != nil {
		fmt.Printf("could not update %s: %v\n", profile, err)
		return
	}
	if len(added) == 0 {
		fmt.Println("Already present — nothing to add.")
		return
	}

	fmt.Printf("\nAdded %d line(s) to %s\n", len(added), profile)
	if backup != "" {
		fmt.Printf("Backup: %s\n", backup)
	}
	fmt.Println(dimText.Render("Open a new terminal, or run: source " + profile))
}
