// Package bootstrap checks and installs the prerequisites macwtf itself needs.
//
// A fresh macOS install has neither the Xcode Command Line Tools nor Homebrew,
// so a tool whose job is installing your toolchain cannot assume a toolchain.
// Everything here is detection first: nothing installs without being asked.
package bootstrap

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Status is the result of one prerequisite check.
type Status struct {
	Name string

	// OK reports whether the prerequisite is satisfied.
	OK bool

	// Detail describes what was found, or what is missing.
	Detail string

	// Fix is the command a user can run to satisfy it, if there is one.
	Fix string

	// Required marks a prerequisite that blocks installs entirely, as
	// opposed to one that only limits some backends.
	Required bool
}

// Report is the full prerequisite picture.
type Report struct {
	Statuses []Status
}

// Blocking returns the unsatisfied prerequisites that prevent installing.
func (r *Report) Blocking() []Status {
	var out []Status
	for _, s := range r.Statuses {
		if s.Required && !s.OK {
			out = append(out, s)
		}
	}
	return out
}

// OK reports whether everything required is satisfied.
func (r *Report) OK() bool { return len(r.Blocking()) == 0 }

// Check probes the machine. Read-only: it never installs or modifies anything.
func Check() *Report {
	return &Report{Statuses: []Status{
		checkArch(),
		checkCLT(),
		checkBrew(),
	}}
}

func checkArch() Status {
	s := Status{Name: "Apple Silicon", Required: true}
	if runtime.GOARCH == "arm64" {
		s.OK = true
		s.Detail = "arm64"
		return s
	}
	s.Detail = fmt.Sprintf("this machine is %s; macWTF targets Apple Silicon only", runtime.GOARCH)
	return s
}

func checkCLT() Status {
	s := Status{
		Name:     "Xcode Command Line Tools",
		Required: true,
		Fix:      "xcode-select --install",
	}
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		s.Detail = "not installed — required by Homebrew and by anything that compiles"
		return s
	}
	s.OK = true
	s.Detail = strings.TrimSpace(string(out))
	return s
}

func checkBrew() Status {
	s := Status{
		Name:     "Homebrew",
		Required: true,
		Fix:      InstallCommand,
	}
	if _, err := exec.LookPath("brew"); err != nil {
		s.Detail = "not installed — the brew and cask backends need it"
		return s
	}
	prefix, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		s.Detail = "found on PATH but `brew --prefix` failed"
		return s
	}
	s.OK = true
	s.Detail = strings.TrimSpace(string(prefix))
	return s
}

// InstallCommand is Homebrew's official installer. Quoted verbatim so the user
// can see exactly what would run, and can run it themselves instead.
const InstallCommand = `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`

// Render writes a human-readable prerequisite report.
func (r *Report) Render(w *strings.Builder) {
	w.WriteString("\nprerequisites\n─────────────\n\n")
	for _, s := range r.Statuses {
		mark := "✗"
		if s.OK {
			mark = "✓"
		}
		fmt.Fprintf(w, "  %s  %-26s %s\n", mark, s.Name, s.Detail)
	}

	blocking := r.Blocking()
	if len(blocking) == 0 {
		w.WriteString("\nEverything macWTF needs is present.\n")
		return
	}

	fmt.Fprintf(w, "\n%d prerequisite(s) missing. Installs will not run until they are met.\n\n",
		len(blocking))
	for _, s := range blocking {
		if s.Fix == "" {
			continue
		}
		fmt.Fprintf(w, "  %s\n    %s\n\n", s.Name, s.Fix)
	}
	w.WriteString("Homebrew's installer needs an administrator password and will\n")
	w.WriteString("install the Command Line Tools itself if they are missing.\n")
}
