package backend

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Provision describes how to make a backend available.
//
// Most package managers macWTF drives are themselves ordinary Homebrew
// formulae, so a missing one is a single `brew install` away rather than
// something the user has to go and research. This is materially different from
// the Homebrew bootstrap question: that means running someone else's privileged
// installer as root, whereas these are plain formulae installed as you.
type Provision struct {
	Backend manifest.Backend

	// Formula is the Homebrew formula that provides it.
	Formula string

	// Note explains anything the install alone does not finish.
	Note string
}

// provisions maps backends to the formula that supplies them. A backend absent
// here cannot be fixed automatically and is reported as-is.
var provisions = map[manifest.Backend]Provision{
	manifest.BackendPipx: {
		Backend: manifest.BackendPipx, Formula: "pipx",
		Note: "Python CLI tools, each in its own virtualenv",
	},
	manifest.BackendGo: {
		Backend: manifest.BackendGo, Formula: "go",
		Note: "builds Go tools from source",
	},
	manifest.BackendNPM: {
		Backend: manifest.BackendNPM, Formula: "node",
		Note: "provides npm",
	},
	manifest.BackendMAS: {
		Backend: manifest.BackendMAS, Formula: "mas",
		Note: "Mac App Store CLI; needs you signed into the App Store",
	},
	manifest.BackendCargo: {
		Backend: manifest.BackendCargo, Formula: "rustup-init",
		Note: "installs the Rust toolchain installer; run rustup-init afterwards to get cargo",
	},
}

// ProvisionFor returns how to supply a backend, if macWTF knows.
func ProvisionFor(b manifest.Backend) (Provision, bool) {
	p, ok := provisions[b]
	return p, ok
}

// Provisionable filters a set of unavailable backends to those that can be
// fixed by installing a Homebrew formula.
func Provisionable(backends []manifest.Backend) []Provision {
	var out []Provision
	for _, b := range backends {
		if p, ok := provisions[b]; ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Formula < out[j].Formula })
	return out
}

// ProvisionCommand is the single command that installs them all.
func ProvisionCommand(ps []Provision) string {
	if len(ps) == 0 {
		return ""
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Formula)
	}
	return "brew install " + strings.Join(names, " ")
}

// Install installs the formulae that supply these backends.
//
// One brew invocation rather than one per formula: brew resolves shared
// dependencies once, and a single command is what the user would have typed.
func (r Registry) Install(ps []Provision, out func(string)) error {
	if len(ps) == 0 {
		return nil
	}
	args := []string{"install"}
	for _, p := range ps {
		args = append(args, p.Formula)
	}

	cmd := exec.Command("brew", args...)
	combined, err := cmd.CombinedOutput()
	if out != nil {
		for _, line := range strings.Split(string(combined), "\n") {
			if strings.TrimSpace(line) != "" {
				out(line)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("brew install failed: %w", err)
	}
	return nil
}
