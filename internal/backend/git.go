package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Git clones repositories.
//
// Much of what a pentest setup needs is not software at all — SecLists,
// PayloadsAllTheThings, rockyou are data, and no package manager carries them.
// Some tools also only distribute as a repository to clone and run in place.
//
// Clones live under the macWTF data directory rather than /opt: /opt is a Linux
// convention that clutters macOS, and a per-user directory needs no root.
type Git struct{}

func (g *Git) ID() manifest.Backend { return manifest.BackendGit }

func (g *Git) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed (it ships with the Xcode Command Line Tools)")
	}
	return nil
}

// dest is where a repository is cloned to.
func (g *Git) dest(t *manifest.Tool, ctx *Ctx) string {
	name := t.Dest
	if name == "" {
		name = t.ID
	}
	return filepath.Join(ctx.DataDir, name)
}

// Installed lists the directories already cloned.
func (g *Git) Installed(ctx *Ctx) (map[string]bool, error) {
	set := map[string]bool{}
	entries, err := os.ReadDir(ctx.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, fmt.Errorf("read %s: %w", ctx.DataDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// A directory without .git is a half-finished clone, and
		// treating it as installed would leave the user with an empty
		// wordlist directory and no error.
		if _, err := os.Stat(filepath.Join(ctx.DataDir, e.Name(), ".git")); err == nil {
			set[e.Name()] = true
		}
	}
	return set, nil
}

// InstalledKey is the clone directory name.
func (g *Git) InstalledKey(t *manifest.Tool) string {
	if t.Dest != "" {
		return t.Dest
	}
	return t.ID
}

func (g *Git) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	if !strings.Contains(t.Package, "://") && !strings.HasPrefix(t.Package, "git@") {
		return nil, fmt.Errorf("git backend needs a repository URL, got %q", t.Package)
	}
	dest := g.dest(t, ctx)

	steps := []Step{{
		Desc: "clone " + t.Package,
		Args: []string{fmt.Sprintf("mkdir -p %q && git clone --depth 1 %q %q",
			filepath.Dir(dest), t.Package, dest)},
		Kind:  KindInstall,
		Shell: true,
	}}
	return commonSteps(steps, t, ctx), nil
}

func (g *Git) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	return []Step{{
		Desc:  "remove " + g.dest(t, ctx),
		Args:  []string{fmt.Sprintf("rm -rf %q", g.dest(t, ctx))},
		Kind:  KindRemove,
		Shell: true,
	}}, nil
}
