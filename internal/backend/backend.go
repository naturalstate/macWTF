// Package backend turns catalogue entries into concrete commands.
//
// The central rule here: backends build plans, they never execute. Every
// method returns a []Step describing what would happen; a separate executor
// decides whether to actually spawn anything. This is what makes --dry-run
// trustworthy — it is not a parallel code path that can drift out of sync with
// a real install, it is the same plan with execution declined.
package backend

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Kind classifies a step so the executor and the UI can treat categories of
// work differently — notably so that security-relevant steps can be surfaced
// and confirmed rather than buried in a wall of output.
type Kind int

const (
	KindInstall Kind = iota
	KindTap
	KindPostInstall
	KindQuarantine // strips com.apple.quarantine; always requires consent
	KindVerify
	KindRemove
)

func (k Kind) String() string {
	switch k {
	case KindInstall:
		return "install"
	case KindTap:
		return "tap"
	case KindPostInstall:
		return "post-install"
	case KindQuarantine:
		return "quarantine"
	case KindVerify:
		return "verify"
	case KindRemove:
		return "remove"
	}
	return "unknown"
}

// Step is a single command, fully resolved and ready to run.
type Step struct {
	Desc string
	Name string
	Args []string
	Kind Kind

	// Sudo marks a step that needs elevation. Never applied silently: the
	// executor surfaces these before running them.
	Sudo bool

	// Shell marks a step whose Args[0] is a shell snippet from the manifest
	// (post_install, verify_cmd) rather than an argv vector.
	Shell bool
}

// String renders the step the way a user would type it, which is what
// --dry-run prints and therefore what they can copy and run by hand.
func (s Step) String() string {
	var b strings.Builder
	if s.Sudo {
		b.WriteString("sudo ")
	}
	if s.Shell {
		b.WriteString(s.Args[0])
		return b.String()
	}
	b.WriteString(s.Name)
	for _, a := range s.Args {
		b.WriteByte(' ')
		if strings.ContainsAny(a, " \t\"'$") {
			fmt.Fprintf(&b, "%q", a)
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// Cmd builds an exec.Cmd for the step. Called only by the executor.
func (s Step) Cmd() *exec.Cmd {
	name, args := s.Name, s.Args
	if s.Shell {
		name, args = "/bin/sh", []string{"-c", s.Args[0]}
	}
	if s.Sudo {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	return exec.Command(name, args...)
}

// Backend installs tools through one specific mechanism. Homebrew is one
// implementation among many, deliberately not a privileged default.
type Backend interface {
	// ID is the manifest backend name this implements.
	ID() manifest.Backend

	// Preflight reports whether the backend can run at all — usually
	// whether its driving binary exists. Failure disables every tool using
	// this backend rather than aborting the whole run.
	Preflight(ctx *Ctx) error

	// Installed snapshots everything this backend has already installed, in
	// one call rather than one per tool.
	Installed(ctx *Ctx) (map[string]bool, error)

	// InstalledKey is the key to look up in the Installed set for a given
	// tool. Usually the package name, but not always: Go keeps no package
	// database, so its set is keyed by binary name and a module path has to
	// be reduced to the binary it produces.
	InstalledKey(t *manifest.Tool) string

	// InstallPlan returns the steps that would install the tool.
	InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error)

	// RemovePlan returns the steps that would remove it.
	RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error)
}

// Registry maps backend names to implementations.
type Registry map[manifest.Backend]Backend

// NewRegistry returns the backends implemented so far. Tools whose backend is
// absent are reported as unsupported rather than silently skipped.
func NewRegistry() Registry {
	return Registry{
		manifest.BackendBrew:  &Brew{},
		manifest.BackendCask:  &Cask{},
		manifest.BackendPipx:  &Pipx{},
		manifest.BackendGo:    &Golang{},
		manifest.BackendCargo: &Cargo{},
		manifest.BackendNPM:   &NPM{},

		manifest.BackendCurl:     &Curl{},
		manifest.BackendGit:      &Git{},
		manifest.BackendMAS:      &MAS{},
		manifest.BackendDefaults: &Defaults{},
	}
}

// Get returns the backend for a tool, or an error naming what is missing.
func (r Registry) Get(b manifest.Backend) (Backend, error) {
	impl, ok := r[b]
	if !ok {
		return nil, fmt.Errorf("backend %q is not implemented yet", b)
	}
	return impl, nil
}

// commonSteps appends the post_install and verify steps shared by every
// backend, plus the quarantine strip when one is called for. Keeping this in
// one place means a new backend cannot forget to honour a manifest field.
func commonSteps(steps []Step, t *manifest.Tool, ctx *Ctx) []Step {
	for _, cmd := range t.PostInstall {
		steps = append(steps, Step{
			Desc:  "post-install: " + t.ID,
			Args:  []string{cmd},
			Kind:  KindPostInstall,
			Shell: true,
		})
	}

	// Quarantine stripping waives a Gatekeeper malware check. It is only
	// ever planned when the user has explicitly allowed it, and it is
	// tagged so the executor can confirm before running.
	if t.QuarantineStrip && t.AppPath != "" && ctx.AllowQuarantineStrip {
		steps = append(steps, Step{
			Desc: "remove Gatekeeper quarantine from " + t.AppPath,
			Name: "xattr",
			Args: []string{"-d", "-r", "com.apple.quarantine", t.AppPath},
			Kind: KindQuarantine,
		})
	}

	if t.VerifyCmd != "" {
		steps = append(steps, Step{
			Desc:  "verify " + t.ID,
			Args:  []string{t.VerifyCmd},
			Kind:  KindVerify,
			Shell: true,
		})
	}
	return steps
}
