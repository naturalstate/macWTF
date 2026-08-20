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
)

const usage = `macwtf — install the tooling macOS leaves out

usage:
  macwtf <command> [flags]

commands:
  validate    check the catalogue for schema and referential integrity errors
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
		fmt.Printf(usage, manifest.ManifestDirEnv)
		return nil
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "validate":
		return runValidate(rest)
	case "list":
		return runList(rest)
	case "install":
		return runInstall(rest)
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
		if len(p.Includes) > 0 {
			fmt.Printf("             includes: %s\n", strings.Join(p.Includes, ", "))
		}
		fmt.Printf("             %d tool(s) declared directly\n\n", len(p.Tools))
	}
	return nil
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
	if t.LinuxOnly {
		f = append(f, "!linux-only")
	}
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
