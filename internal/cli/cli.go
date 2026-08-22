// Package cli implements macwtf's command dispatch.
package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
)

const usage = `macwtf — install the tooling macOS leaves out

usage:
  macwtf <command> [flags]

commands:
  tui         launch the interactive interface (default with no arguments)
  bootstrap   install the prerequisites macwtf needs (Homebrew, Xcode CLT)
  doctor      check that the prerequisites macwtf needs are present
  validate    check the catalogue for schema and referential integrity errors
  check       verify every package name still resolves upstream (needs network)
  list        list tools, categories and profiles
  install     install a profile, a category, or individual tools
  version     print the version

global flags:
  --manifest-dir <path>   read the catalogue from a checkout instead of the
                          copy embedded in this binary (or set %s)

Run "macwtf <command> -h" for command-specific flags.
`

// Run dispatches a command. It returns an error rather than exiting so that
// main owns the process lifecycle and tests can call it directly.
func Run(args []string, version string) error {
	if len(args) == 0 {
		return runTUI(nil)
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "validate":
		return runValidate(rest)
	case "check":
		return runCheck(rest)
	case "list":
		return runList(rest)
	case "install":
		return runInstall(rest)
	case "tui":
		return runTUI(rest)
	case "doctor":
		return runDoctor(rest)
	case "bootstrap":
		return runBootstrap(rest)
	case "version", "--version", "-v":
		fmt.Printf("macwtf %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return nil
	case "help", "--help", "-h":
		fmt.Printf(usage, manifest.ManifestDirEnv)
		return nil
	default:
		return fmt.Errorf("unknown command %q — run \"macwtf help\"", cmd)
	}
}

// manifestDirFlag registers the shared --manifest-dir flag on a flag set.
func manifestDirFlag(fs *flag.FlagSet) *string {
	return fs.String("manifest-dir", "", "read the catalogue from this directory instead of the embedded copy")
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	quiet := fs.Bool("quiet", false, "print nothing on success")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}

	if err := cat.Validate(); err != nil {
		var ps manifest.Problems
		if asProblems(err, &ps) {
			fmt.Fprintf(os.Stderr, "%d problem(s) found:\n\n", len(ps))
			for _, p := range ps {
				fmt.Fprintln(os.Stderr, "  ✗ "+p.Error())
			}
			fmt.Fprintln(os.Stderr)
			return fmt.Errorf("catalogue is invalid")
		}
		return err
	}

	if !*quiet {
		fmt.Printf("✓ catalogue is valid — %d tools across %d categories, %d profiles\n",
			len(cat.Tools), len(cat.Categories()), len(cat.Profiles))
		if cat.OtherPlatform > 0 {
			fmt.Printf("  %d shared entries have no macOS block and are not part of this catalogue\n",
				cat.OtherPlatform)
		}
	}
	return nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	category := fs.String("category", "", "list only tools in this category")
	profiles := fs.Bool("profiles", false, "list profiles instead of tools")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}

	if *profiles {
		return listProfiles(cat)
	}
	return listTools(cat, *category)
}

func listProfiles(cat *manifest.Catalogue) error {
	ps := append([]*manifest.Profile(nil), cat.Profiles...)
	sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })

	for _, p := range ps {
		fmt.Printf("%-12s %s\n", p.ID, p.Name)
		if p.Description != "" {
			fmt.Printf("             %s\n", p.Description)
		}
		if c := profileComposition(p); c != "" {
			fmt.Printf("             %s\n", c)
		}

		// The resolved count, not the declared one. A profile composed
		// from categories declares no tools at all, and reporting that
		// as "0 tools" is worse than saying nothing.
		res, err := resolve.Resolve(cat, resolve.Request{Profile: p.ID})
		if err != nil {
			fmt.Printf("             (cannot resolve: %v)\n\n", err)
			continue
		}
		fmt.Printf("             %d tools\n\n", len(res.Install))
	}
	return nil
}

// profileComposition renders a profile the way the design table reads:
// "Baseline + Recon + Web", rather than listing every id it happens to pull in.
func profileComposition(p *manifest.Profile) string {
	var parts []string
	parts = append(parts, p.Includes...)
	parts = append(parts, p.Categories...)
	if n := len(p.Tools); n > 0 {
		parts = append(parts, fmt.Sprintf("%d named tool(s)", n))
	}
	if len(parts) == 0 {
		return ""
	}
	out := strings.Join(parts, " + ")
	if len(p.Excludes) > 0 {
		out += " − " + strings.Join(p.Excludes, ", ")
	}
	return out
}

func listTools(cat *manifest.Catalogue, only string) error {
	cats := cat.Categories()
	if only != "" {
		found := false
		for _, c := range cats {
			if c == only {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown category %q — known: %s", only, strings.Join(cats, ", "))
		}
		cats = []string{only}
	}

	for _, c := range cats {
		tools := cat.InCategory(c)
		fmt.Printf("\n%s (%d)\n", c, len(tools))
		for _, t := range tools {
			flags := toolFlags(t)
			fmt.Printf("  %-14s %-9s %s%s\n", t.ID, t.Backend, t.Description, flags)
		}
	}
	fmt.Println()
	return nil
}

// toolFlags renders the short markers the catalogue uses for tools that need
// something beyond a plain install.
func toolFlags(t *manifest.Tool) string {
	var f []string
	if t.QuarantineStrip {
		f = append(f, "Q")
	}
	if len(t.TCCPermissions) > 0 {
		f = append(f, "T")
	}
	if t.RequiresRosetta {
		f = append(f, "R")
	}
	if t.License == manifest.LicensePaid || t.License == manifest.LicenseFreemium {
		f = append(f, string(t.License))
	}
	if len(f) == 0 {
		return ""
	}
	return "  [" + strings.Join(f, " ") + "]"
}

// asProblems is errors.As specialised to manifest.Problems, which is a slice
// type and therefore needs a value rather than pointer target.
func asProblems(err error, target *manifest.Problems) bool {
	if ps, ok := err.(manifest.Problems); ok {
		*target = ps
		return true
	}
	return false
}
