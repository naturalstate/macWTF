package backend

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// NPM installs global Node CLI packages.
type NPM struct{}

func (n *NPM) ID() manifest.Backend { return manifest.BackendNPM }

func (n *NPM) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm is not installed (brew install node)")
	}
	return nil
}

// Installed reads the global package list as JSON. Parsing npm's tree output
// by eye is fragile; --json is stable.
func (n *NPM) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("npm", "list", "-g", "--depth=0", "--json").Output()
	if err != nil && len(out) == 0 {
		return map[string]bool{}, nil
	}
	var doc struct {
		Dependencies map[string]any `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return map[string]bool{}, nil
	}
	set := map[string]bool{}
	for name := range doc.Dependencies {
		set[name] = true
	}
	return set, nil
}

// InstalledKey is the package name: this backend's set is package-keyed.
func (n *NPM) InstalledKey(t *manifest.Tool) string { return t.Package }

func (n *NPM) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	steps := []Step{{
		Desc: "npm install -g " + t.Package,
		Name: "npm",
		Args: []string{"install", "-g", t.Package},
		Kind: KindInstall,
	}}
	return commonSteps(steps, t, ctx), nil
}

func (n *NPM) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	return []Step{{
		Desc: "npm uninstall -g " + t.Package,
		Name: "npm",
		Args: []string{"uninstall", "-g", t.Package},
		Kind: KindRemove,
	}}, nil
}
