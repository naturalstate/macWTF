// Package bootstrap checks and installs the prerequisites macwtf itself needs.
//
// A fresh macOS install has neither the Xcode Command Line Tools nor Homebrew,
// so a tool whose job is installing your toolchain cannot assume a toolchain.
// Everything here is detection first: nothing installs without being asked.
package bootstrap

import (
	"fmt"
	"os"
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

	// Fix is the command that satisfies an unmet prerequisite.
	Fix string

	// Advice is guidance for a prerequisite that is satisfied but not
	// ideal — chiefly Homebrew installed outside PATH, which works for this
	// run but will confuse every future shell. Distinct from Fix, which
	// answers "how do I make this work at all".
	Advice string

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
		checkAdmin(),
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
	s := Status{Name: "Xcode Command Line Tools", Required: true}
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		s.Detail = "not installed — required by Homebrew and by anything that compiles"
		s.Fix = "xcode-select --install"
		return s
	}
	s.OK = true
	s.Detail = strings.TrimSpace(string(out))
	return s
}

// BrewPrefixes are the standard Homebrew locations. Apple Silicon uses
// /opt/homebrew; Intel used /usr/local. Never hardcoded past this point —
// everything downstream asks `brew --prefix`.
var BrewPrefixes = []string{"/opt/homebrew", "/usr/local"}

// checkBrew looks for Homebrew on PATH and, failing that, in the standard
// prefixes. The distinction matters: /opt/homebrew is not on the default PATH
// on Apple Silicon, so a perfectly good install looks missing in any shell that
// has not sourced brew shellenv — a fresh terminal, a non-login SSH session, or
// a script. Reporting that as "not installed" sends the user to reinstall
// something they already have.
func checkBrew() Status {
	s := Status{Name: "Homebrew", Required: true}

	if _, err := exec.LookPath("brew"); err == nil {
		prefix, err := exec.Command("brew", "--prefix").Output()
		if err != nil {
			s.Detail = "found on PATH but `brew --prefix` failed"
			return s
		}
		s.OK = true
		s.Detail = strings.TrimSpace(string(prefix))
		return s
	}

	for _, prefix := range BrewPrefixes {
		if _, err := os.Stat(prefix + "/bin/brew"); err != nil {
			continue
		}
		s.OK = true
		s.Detail = prefix + " (installed, but not on this shell's PATH)"
		s.Advice = ShellEnvHint(prefix)
		return s
	}

	s.Detail = "not installed — the brew and cask backends need it"
	s.Fix = InstallCommand
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
		for _, s := range r.Statuses {
			if s.Advice != "" {
				fmt.Fprintf(w, "\n%s works for this run, but add this to your shell\nprofile so it works in every future shell:\n\n    %s\n",
					s.Name, s.Advice)
			}
		}
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
	w.WriteString("Install these yourself and re-run macwtf. Homebrew's installer\n")
	w.WriteString("needs an administrator password and will install the Command Line\n")
	w.WriteString("Tools itself if they are missing.\n")
}

// InstallHomebrew runs Homebrew's official installer.
//
// This is a genuinely privileged action: it downloads and executes a shell
// script that calls sudo. It never runs without explicit consent, and the exact
// command is shown first so the user can read it or run it themselves.
//
// Two details matter and are easy to get wrong:
//
// The command substitution form is required. Piping curl into bash makes the
// script's stdin the pipe rather than the terminal, so sudo cannot read a
// password and the install dies. `bash -c "$(curl ...)"` keeps stdin attached
// to the terminal.
//
// NONINTERACTIVE must only be set when there is genuinely no terminal. It does
// not merely skip the "press RETURN" prompt: Homebrew's have_sudo_access uses
// `sudo -n -v` under it, which fails outright unless the user has passwordless
// sudo. On a normal account that surfaces as "Need sudo access on macOS", which
// looks like an account problem and is not.
func InstallHomebrew() error {
	script := `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`

	cmd := exec.Command("/bin/bash", "-c", script)
	cmd.Env = os.Environ()
	if !isTerminal(os.Stdin) {
		// No terminal to type a password into. Homebrew will need
		// passwordless sudo; if it is unavailable the error explains why.
		cmd.Env = append(cmd.Env, "NONINTERACTIVE=1")
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isTerminal reports whether a file is attached to a terminal, which decides
// whether an interactive password prompt is possible.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// checkAdmin reports whether the user can obtain administrator rights.
// Membership of the admin group is checked rather than running `sudo -v`,
// because probing sudo would itself prompt for a password.
func checkAdmin() Status {
	s := Status{Name: "Administrator rights", Required: true}
	out, err := exec.Command("id", "-Gn").Output()
	if err != nil {
		s.Detail = "could not determine group membership"
		return s
	}
	for _, g := range strings.Fields(string(out)) {
		if g == "admin" {
			s.OK = true
			s.Detail = "member of the admin group"
			return s
		}
	}
	s.Fix = "Have an admin user run: dseditgroup -o edit -a $USER -t user admin"
	s.Detail = "not an administrator — Homebrew and any sudo step will fail"
	return s
}

// ShellEnvHint returns the line a user must add to their shell profile so brew
// is on PATH in future sessions. Apple Silicon installs to /opt/homebrew, which
// is not on the default PATH.
func ShellEnvHint(prefix string) string {
	if prefix == "" {
		prefix = "/opt/homebrew"
	}
	return fmt.Sprintf(`eval "$(%s/bin/brew shellenv)"`, prefix)
}

// AdoptBrewPath finds a Homebrew that is installed but not yet on PATH — the
// state immediately after installation — and adds it to this process's PATH so
// the run can continue without the user restarting their shell.
func AdoptBrewPath() (string, bool) {
	if _, err := exec.LookPath("brew"); err == nil {
		return "", false
	}
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		bin := prefix + "/bin/brew"
		if _, err := os.Stat(bin); err != nil {
			continue
		}
		os.Setenv("PATH", prefix+"/bin:"+os.Getenv("PATH"))
		return prefix, true
	}
	return "", false
}
