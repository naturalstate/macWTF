package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Golang installs tools with `go install`.
//
// A large part of the modern recon toolchain — the ProjectDiscovery suite,
// gowitness, kerbrute — ships this way and nothing else.
type Golang struct{}

func (g *Golang) ID() manifest.Backend { return manifest.BackendGo }

func (g *Golang) Preflight(ctx *Ctx) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go is not installed (brew install go)")
	}
	return nil
}

// binDir is where `go install` puts binaries: GOBIN if set, else GOPATH/bin.
func (g *Golang) binDir() string {
	if out, err := exec.Command("go", "env", "GOBIN").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return filepath.Join(s, "bin")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin")
	}
	return ""
}

// Installed lists binaries in the Go bin directory.
//
// Go keeps no package database — once installed, a binary is just a file with
// no record of the module it came from. So the check is by binary name, which
// is why BinaryName has to undo the module path.
func (g *Golang) Installed(ctx *Ctx) (map[string]bool, error) {
	dir := g.binDir()
	if dir == "" {
		return map[string]bool{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	set := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		set[e.Name()] = true
	}

	return set, nil
}

// BinaryName derives the installed binary name from a Go module path.
//
//	github.com/ffuf/ffuf/v2@latest      -> ffuf
//	github.com/projectdiscovery/...@v1  -> the last non-version element
//
// The /vN major-version suffix is part of the module path but never part of
// the binary name, so it has to be stripped or nothing would ever be detected
// as installed.
func BinaryName(pkg string) string {
	p := pkg
	if i := strings.Index(p, "@"); i >= 0 {
		p = p[:i]
	}
	p = strings.TrimSuffix(p, "/")
	parts := strings.Split(p, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if seg == "" {
			continue
		}
		// Skip a major-version suffix such as v2.
		if len(seg) > 1 && seg[0] == 'v' && isAllDigits(seg[1:]) {
			continue
		}
		if seg == "cmd" || seg == "..." {
			continue
		}
		return seg
	}
	return p
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// InstalledKey reduces a module path to the binary it produces, because that
// is all Go leaves behind to detect.
func (g *Golang) InstalledKey(t *manifest.Tool) string { return BinaryName(t.Package) }

func (g *Golang) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	pkg := t.Package
	// go install requires an explicit version suffix in module-aware mode.
	if !strings.Contains(pkg, "@") {
		pkg += "@latest"
	}

	steps := []Step{{
		Desc: "go install " + pkg,
		Name: "go",
		Args: []string{"install", pkg},
		Kind: KindInstall,
	}}
	return commonSteps(steps, t, ctx), nil
}

// RemovePlan deletes the binary. Go has no uninstall: `go install` copies a
// file into place and records nothing, so removing it is removing the file.
func (g *Golang) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	dir := g.binDir()
	if dir == "" {
		return nil, fmt.Errorf("cannot determine the Go bin directory")
	}
	bin := filepath.Join(dir, BinaryName(t.Package))
	return []Step{{
		Desc: "remove " + bin,
		Name: "rm",
		Args: []string{"-f", bin},
		Kind: KindRemove,
	}}, nil
}
