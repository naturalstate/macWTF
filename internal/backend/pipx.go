package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Pipx installs Python CLI applications, each into its own virtualenv.
//
// pipx rather than pip on purpose: a large part of the security catalogue is
// Python, and installing those into one shared environment guarantees
// dependency conflicts — impacket and a scanner that pins a different urllib3
// cannot coexist. pipx gives each tool its own venv and links only the
// entry points.
type Pipx struct{}

func (p *Pipx) ID() manifest.Backend { return manifest.BackendPipx }

func (p *Pipx) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("pipx"); err != nil {
		return fmt.Errorf("pipx is not installed (brew install pipx)")
	}
	return nil
}

// Installed parses `pipx list --short`, which prints "name version" per line.
func (p *Pipx) Installed(ctx *Ctx) (map[string]bool, error) {
	out, err := exec.Command("pipx", "list", "--short").Output()
	if err != nil {
		// pipx exits non-zero when nothing is installed at all, which is
		// a legitimate state rather than a failure.
		return map[string]bool{}, nil
	}

	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		set[name] = true
		// PyPI treats underscores and hyphens as equivalent, and
		// manifests may use either spelling.
		set[strings.ReplaceAll(name, "_", "-")] = true
		set[strings.ReplaceAll(name, "-", "_")] = true
	}
	return set, nil
}

// InstalledKey is the name pipx registers the application under.
//
// Not the install spec. A package installed from git has a spec like
// "git+https://github.com/laramies/theHarvester" but pipx lists it as
// "theHarvester", so keying on the spec meant it was never detected as
// installed — reinstalled on every run — and `pipx uninstall <spec>` failed
// because uninstall takes the name.
func (p *Pipx) InstalledKey(t *manifest.Tool) string { return pipxName(t) }

// pipxName resolves the registered name from an explicit binary, or by taking
// the repository name out of a VCS spec.
func pipxName(t *manifest.Tool) string {
	if t.Binary != "" {
		return t.Binary
	}
	pkg := t.Package
	if i := strings.Index(pkg, "+"); i >= 0 && strings.Contains(pkg[:i], "git") {
		pkg = pkg[i+1:]
	}
	if strings.Contains(pkg, "://") {
		pkg = strings.TrimSuffix(pkg, "/")
		pkg = strings.TrimSuffix(pkg, ".git")
		if i := strings.LastIndex(pkg, "/"); i >= 0 {
			pkg = pkg[i+1:]
		}
		if i := strings.Index(pkg, "@"); i >= 0 {
			pkg = pkg[:i]
		}
	}
	return pkg
}

func (p *Pipx) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	steps := []Step{{
		Desc: "pipx install " + t.Package,
		Name: "pipx",
		Args: []string{"install", t.Package},
		Kind: KindInstall,
	}}
	return commonSteps(steps, t, ctx), nil
}

// RemovePlan uninstalls by registered name: pipx uninstall does not accept an
// install spec.
func (p *Pipx) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	name := pipxName(t)
	return []Step{{
		Desc: "pipx uninstall " + name,
		Name: "pipx",
		Args: []string{"uninstall", name},
		Kind: KindRemove,
	}}, nil
}
