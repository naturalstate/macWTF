package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Brew installs Homebrew formulae — command-line tools and libraries.
type Brew struct{}

func (b *Brew) ID() manifest.Backend { return manifest.BackendBrew }

func (b *Brew) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew is not installed or not on PATH")
	}
	return nil
}

// Installed lists installed formulae in a single call. Names are returned
// without their tap prefix, matching how manifests refer to them.
func (b *Brew) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("brew", "list", "--formula", "-1").Output()
	if err != nil {
		return nil, fmt.Errorf("brew list --formula: %w", err)
	}
	return lineSet(string(out)), nil
}

// InstalledKey is the package name: this backend's set is package-keyed.
func (b *Brew) InstalledKey(t *manifest.Tool) string { return t.Package }

func (b *Brew) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	var steps []Step
	if t.Tap != "" {
		steps = append(steps, Step{
			Desc: "tap " + t.Tap,
			Name: "brew",
			Args: []string{"tap", t.Tap},
			Kind: KindTap,
		})
	}
	steps = append(steps, Step{
		Desc: "install formula " + t.Package,
		Name: "brew",
		Args: []string{"install", "--formula", t.Package},
		Kind: KindInstall,
	})
	return commonSteps(steps, t, ctx), nil
}

func (b *Brew) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	return []Step{{
		Desc: "uninstall formula " + t.Package,
		Name: "brew",
		Args: []string{"uninstall", "--formula", t.Package},
		Kind: KindRemove,
	}}, nil
}

// lineSet turns newline-delimited command output into a set, ignoring blanks.
func lineSet(out string) map[string]bool {
	set := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Tapped formulae list as owner/tap/name; manifests use the
		// bare name, so index both.
		set[line] = true
		if i := strings.LastIndex(line, "/"); i >= 0 {
			set[line[i+1:]] = true
		}
	}
	return set
}
