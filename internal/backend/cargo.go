package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Cargo installs Rust binaries from crates.io.
type Cargo struct{}

func (c *Cargo) ID() manifest.Backend { return manifest.BackendCargo }

func (c *Cargo) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("cargo"); err != nil {
		return fmt.Errorf("cargo is not installed (brew install rustup-init, then rustup-init)")
	}
	return nil
}

// Installed parses `cargo install --list`, which prints a crate header line
// like "ripgrep v14.1.0:" followed by indented binary names.
func (c *Cargo) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("cargo", "install", "--list").Output()
	if err != nil {
		return map[string]bool{}, nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // indented lines are the binaries, not the crate
		}
		name := strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), ":"))
		if len(name) > 0 {
			set[name[0]] = true
		}
	}
	return set, nil
}

// InstalledKey is the crate name: this backend's set is crate-keyed.
func (c *Cargo) InstalledKey(t *manifest.Tool) string { return t.Package }

func (c *Cargo) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	steps := []Step{{
		Desc: "cargo install " + t.Package,
		Name: "cargo",
		Args: []string{"install", t.Package},
		Kind: KindInstall,
	}}
	return commonSteps(steps, t, ctx), nil
}

func (c *Cargo) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	return []Step{{
		Desc: "cargo uninstall " + t.Package,
		Name: "cargo",
		Args: []string{"uninstall", t.Package},
		Kind: KindRemove,
	}}, nil
}
