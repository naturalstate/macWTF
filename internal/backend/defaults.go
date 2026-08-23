package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Defaults writes macOS preferences.
//
// Not a package manager: these entries change how the system behaves rather
// than installing anything. That difference drives two rules.
//
// First, every entry must declare how to undo itself. A tweak nobody can
// reverse is a trap, and "reinstall macOS" is not a revert. Validation rejects
// an entry without one.
//
// Second, the key and value are separate fields rather than a command string,
// so the write can be verified by reading the value back, and so the revert is
// expressible as data rather than as another opaque command.
type Defaults struct{}

func (d *Defaults) ID() manifest.Backend { return manifest.BackendDefaults }

// Preflight has nothing to check: defaults is part of macOS.
func (d *Defaults) Preflight(ctx *Ctx) error { return nil }

// Installed reads each domain's current state.
//
// Unlike a package, a preference is never simply absent — it has whatever value
// the system defaults to. So "installed" means "already set to the value this
// entry wants", which is read per tool rather than snapshotted in one call.
func (d *Defaults) Installed(ctx *Ctx) (map[string]bool, error) {
	return map[string]bool{}, nil
}

// InstalledKey is unused: applied state is determined per tool by IsApplied,
// because there is no list of preferences to snapshot.
func (d *Defaults) InstalledKey(t *manifest.Tool) string { return "" }

// IsApplied reports whether the preference already holds the wanted value.
func (d *Defaults) IsApplied(t *manifest.Tool) bool {
	if t.Package == "" || t.Key == "" {
		return false
	}
	out, err := exec.Command("defaults", "read", t.Package, t.Key).Output()
	if err != nil {
		return false // key not set at all
	}
	got := strings.TrimSpace(string(out))
	want := strings.TrimSpace(t.Value)

	// `defaults read` prints booleans as 0 and 1 whatever spelling was
	// written, so compare in those terms.
	return got == want || boolish(got) == boolish(want)
}

func boolish(s string) string {
	switch strings.ToLower(s) {
	case "1", "true", "yes":
		return "1"
	case "0", "false", "no":
		return "0"
	}
	return s
}

func (d *Defaults) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	if t.Package == "" || t.Key == "" {
		return nil, fmt.Errorf("defaults backend needs a domain in package and a key")
	}

	args := []string{"write", t.Package, t.Key}
	if vt := t.ValueType; vt != "" {
		args = append(args, "-"+vt)
	}
	args = append(args, t.Value)

	steps := []Step{{
		Desc: fmt.Sprintf("set %s %s to %s", t.Package, t.Key, t.Value),
		Name: "defaults",
		Args: args,
		Kind: KindInstall,
	}}

	// Read the value back rather than trusting the exit status: `defaults
	// write` reports success for a domain that does not exist.
	steps = append(steps, Step{
		Desc:  "verify " + t.ID,
		Args:  []string{fmt.Sprintf("defaults read %q %q >/dev/null", t.Package, t.Key)},
		Kind:  KindVerify,
		Shell: true,
	})

	return commonSteps(steps, t, ctx), nil
}

// RemovePlan restores the previous behaviour, either by writing the declared
// revert value or by deleting the key so the system default applies again.
func (d *Defaults) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	if strings.EqualFold(t.Revert, "delete") || t.Revert == "" {
		return []Step{{
			Desc: fmt.Sprintf("unset %s %s", t.Package, t.Key),
			Name: "defaults",
			Args: []string{"delete", t.Package, t.Key},
			Kind: KindRemove,
		}}, nil
	}

	args := []string{"write", t.Package, t.Key}
	if vt := t.ValueType; vt != "" {
		args = append(args, "-"+vt)
	}
	args = append(args, t.Revert)

	return []Step{{
		Desc: fmt.Sprintf("restore %s %s to %s", t.Package, t.Key, t.Revert),
		Name: "defaults",
		Args: args,
		Kind: KindRemove,
	}}, nil
}
