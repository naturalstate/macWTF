package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/naturalstate/macWTF/internal/check"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
)

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	dir := manifestDirFlag(fs)
	concurrency := fs.Int("concurrency", 8, "parallel requests")
	timeout := fs.Duration("timeout", 20*time.Second, "per-request timeout")
	quiet := fs.Bool("quiet", false, "only print problems")
	strict := fs.Bool("strict", false, "treat deprecations as failures")
	asJSON := fs.Bool("json", false, "emit results as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cat, err := manifest.Load(*dir)
	if err != nil {
		return err
	}
	if err := cat.Validate(); err != nil {
		return fmt.Errorf("catalogue is invalid, fix that first:\n%w", err)
	}

	if !*asJSON {
		fmt.Printf("Checking %d package name(s) against upstream…\n\n", len(cat.Tools))
	}

	tty := isTerminal(os.Stdout) && !*asJSON
	rep, err := check.Run(context.Background(), cat, check.Options{
		Concurrency: *concurrency,
		Timeout:     *timeout,
		Progress: func(done, total int, r check.Result) {
			if !tty {
				return
			}
			fmt.Printf("\r\033[2K  %s %d/%d  %s",
				install.ProgressBar(done, total, 24), done, total, r.Tool.ID)
		},
	})
	if err != nil {
		return err
	}
	if tty {
		fmt.Printf("\r\033[2K")
	}

	counts := rep.Counts()
	problems := rep.Problems()
	errs := rep.Errors()

	if *asJSON {
		type row struct {
			ID          string `json:"id"`
			File        string `json:"file"`
			Backend     string `json:"backend"`
			Package     string `json:"package"`
			Verdict     string `json:"verdict"`
			Detail      string `json:"detail"`
			Suggestion  string `json:"suggestion,omitempty"`
			Description string `json:"description,omitempty"`
		}
		out := make([]row, 0, len(rep.Results))
		for _, r := range rep.Results {
			out = append(out, row{
				ID: r.Tool.ID, File: r.Tool.SourceFile,
				Backend: string(r.Tool.Backend), Package: r.Tool.Package,
				Verdict: r.Verdict.String(), Detail: r.Detail,
				Suggestion: r.Suggestion, Description: r.Description,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		if err := enc.Encode(out); err != nil {
			return err
		}
		if len(problems) > 0 {
			return fmt.Errorf("%d package name(s) no longer resolve", len(problems))
		}
		return nil
	}

	if !*quiet {
		for _, r := range rep.Results {
			if r.Verdict != check.VerdictOK {
				continue
			}
			fmt.Printf("  %s %-14s %s\n", okMark.Render("✓"), r.Tool.ID, dimText.Render(r.Detail))
		}
	}

	for _, r := range rep.Warnings() {
		fmt.Printf("  %s %-14s %s\n", warnText.Render("!"), r.Tool.ID, r.Detail)
		fmt.Printf("      %s\n", dimText.Render(r.Tool.SourceFile))
	}

	for _, r := range problems {
		fmt.Printf("  %s %-14s %s\n", failMark.Render("✗"), r.Tool.ID, r.Detail)
		fmt.Printf("      %s\n", dimText.Render(r.Tool.SourceFile))
		if r.Suggestion != "" {
			fmt.Printf("      %s\n", warnText.Render("fix: "+r.Suggestion))
		}
	}

	// Checks that could not run are reported apart from real problems: a
	// flaky network is not a broken catalogue, and conflating the two
	// teaches people to ignore a red build.
	for _, r := range errs {
		fmt.Printf("  %s %-14s could not check: %s\n",
			warnText.Render("?"), r.Tool.ID, r.Detail)
	}

	fmt.Printf("\n%d ok", counts[check.VerdictOK])
	if n := counts[check.VerdictSkipped]; n > 0 {
		fmt.Printf(", %d skipped (backend not checkable yet)", n)
	}
	if n := len(rep.Warnings()); n > 0 {
		fmt.Printf(", %d deprecated", n)
	}
	if len(problems) > 0 {
		fmt.Printf(", %d need fixing", len(problems))
	}
	if len(errs) > 0 {
		fmt.Printf(", %d could not be checked", len(errs))
	}
	fmt.Println()

	if len(problems) > 0 {
		return fmt.Errorf("%d package name(s) no longer resolve", len(problems))
	}
	if *strict && len(rep.Warnings()) > 0 {
		return fmt.Errorf("%d deprecated package(s) and --strict given", len(rep.Warnings()))
	}
	return nil
}
