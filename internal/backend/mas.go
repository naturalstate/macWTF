package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// MAS installs from the Mac App Store.
//
// Two caveats worth knowing, both upstream limitations rather than macWTF's:
// it needs an Apple ID signed into the App Store, and it installs reliably only
// for apps already in that account's purchase history. A first-time install of
// an app never previously acquired often fails. Entries using this backend
// therefore carry a manual fallback in their notes.
//
// It is also the one backend that cannot be tested in a VM: Apple Silicon
// guests cannot sign into Apple services at all.
type MAS struct{}

func (m *MAS) ID() manifest.Backend { return manifest.BackendMAS }

func (m *MAS) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("mas"); err != nil {
		return fmt.Errorf("mas is not installed (brew install mas)")
	}
	return nil
}

// Installed parses `mas list`, whose lines start with the numeric app id.
func (m *MAS) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("mas", "list").Output()
	if err != nil {
		// Not signed in, or nothing installed. Neither is a failure
		// worth aborting a run over.
		return map[string]bool{}, nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			set[f[0]] = true
		}
	}
	return set, nil
}

// InstalledKey is the numeric App Store id.
func (m *MAS) InstalledKey(t *manifest.Tool) string { return t.Package }

func (m *MAS) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	if strings.TrimLeft(t.Package, "0123456789") != "" {
		return nil, fmt.Errorf("mas backend needs a numeric App Store id, got %q", t.Package)
	}
	steps := []Step{{
		Desc: "mas install " + t.Package,
		Name: "mas",
		Args: []string{"install", t.Package},
		Kind: KindInstall,
	}}
	return commonSteps(steps, t, ctx), nil
}

func (m *MAS) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	// `mas uninstall` needs root because it deletes from /Applications.
	return []Step{{
		Desc: "mas uninstall " + t.Package,
		Name: "mas",
		Args: []string{"uninstall", t.Package},
		Kind: KindRemove,
		Sudo: true,
	}}, nil
}
