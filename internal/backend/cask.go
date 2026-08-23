package backend

import (
	"fmt"
	"os/exec"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Cask installs Homebrew casks — GUI applications and pre-built binaries.
//
// Casks differ from formulae in ways that matter here: they drop .app bundles
// into /Applications, they frequently need an admin password for bundled
// .pkg payloads, and their contents are far more likely to be unsigned and
// therefore quarantined.
type Cask struct{}

func (c *Cask) ID() manifest.Backend { return manifest.BackendCask }

func (c *Cask) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew is not installed or not on PATH")
	}
	return nil
}

func (c *Cask) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("brew", "list", "--cask", "-1").Output()
	if err != nil {
		return nil, fmt.Errorf("brew list --cask: %w", err)
	}
	return lineSet(string(out)), nil
}

// InstalledKey is the package name: this backend's set is package-keyed.
func (c *Cask) InstalledKey(t *manifest.Tool) string { return t.Package }

func (c *Cask) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
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
		Desc: "install cask " + t.Package,
		Name: "brew",
		Args: []string{"install", "--cask", t.Package},
		Kind: KindInstall,
	})
	return commonSteps(steps, t, ctx), nil
}

// RemovePlan forces the uninstall.
//
// A cask interrupted partway through leaves a populated Caskroom directory,
// and a plain uninstall then refuses with "there is already an App at ...",
// making the mess unrepairable by the tool that made it. --force is the
// documented way through, and on a removal the intent is unambiguous.
func (c *Cask) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	return []Step{{
		Desc: "uninstall cask " + t.Package,
		Name: "brew",
		Args: []string{"uninstall", "--cask", "--force", t.Package},
		Kind: KindRemove,
	}}, nil
}
