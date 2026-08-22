// Package pathenv reports and repairs the PATH entries installed tools need.
//
// The useful simplification: this is per-backend, not per-tool. Every Homebrew
// formula lands in one bin directory, so a single shellenv line covers the
// entire brew half of the catalogue. pipx, cargo and go add one directory each.
// The whole problem is a handful of lines, not one per tool.
//
// Editing a user's shell profile is a stated non-goal unless they opt in, so
// nothing here writes anything without being asked, and the exact line is shown
// before it is added.
package pathenv

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Entry is one bin directory that some backend installs into.
type Entry struct {
	// Backend that owns this directory.
	Backend manifest.Backend

	// Dir is the bin directory itself.
	Dir string

	// Line is what should be added to a shell profile. For Homebrew this
	// is `brew shellenv`, which sets MANPATH and INFOPATH too, rather than
	// a bare PATH append.
	Line string

	// OnPath reports whether Dir is already reachable.
	OnPath bool

	// Exists reports whether the directory is actually present. A backend
	// nothing has been installed through yet has no directory, and telling
	// the user to add a nonexistent path to their profile is noise.
	Exists bool
}

// Needed reports whether this entry is worth telling the user about.
func (e Entry) Needed() bool { return e.Exists && !e.OnPath }

// Detect finds the bin directories for the backends used by the given tools.
//
// Scoped to backends actually in use: a user who installed only brew formulae
// does not need to hear about cargo.
func Detect(backends map[manifest.Backend]bool) []Entry {
	pathDirs := currentPathSet()
	home, _ := os.UserHomeDir()

	var out []Entry
	add := func(b manifest.Backend, dir, line string) {
		if dir == "" {
			return
		}
		_, err := os.Stat(dir)
		out = append(out, Entry{
			Backend: b,
			Dir:     dir,
			Line:    line,
			OnPath:  pathDirs[filepath.Clean(dir)],
			Exists:  err == nil,
		})
	}

	if backends[manifest.BackendBrew] || backends[manifest.BackendCask] {
		if prefix := brewPrefix(); prefix != "" {
			add(manifest.BackendBrew, filepath.Join(prefix, "bin"),
				fmt.Sprintf(`eval "$(%s/bin/brew shellenv)"`, prefix))
		}
	}
	if backends[manifest.BackendPipx] && home != "" {
		dir := filepath.Join(home, ".local", "bin")
		add(manifest.BackendPipx, dir, `export PATH="$HOME/.local/bin:$PATH"`)
	}
	if backends[manifest.BackendCargo] && home != "" {
		dir := filepath.Join(home, ".cargo", "bin")
		add(manifest.BackendCargo, dir, `export PATH="$HOME/.cargo/bin:$PATH"`)
	}
	if backends[manifest.BackendGo] {
		if dir := goBinDir(home); dir != "" {
			add(manifest.BackendGo, dir, fmt.Sprintf(`export PATH="%s:$PATH"`, tildeify(dir, home)))
		}
	}
	return out
}

// Missing filters to the entries that exist but are unreachable.
func Missing(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Needed() {
			out = append(out, e)
		}
	}
	return out
}

func currentPathSet() map[string]bool {
	set := map[string]bool{}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if d != "" {
			set[filepath.Clean(d)] = true
		}
	}
	return set
}

func brewPrefix() string {
	if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, p := range []string{"/opt/homebrew", "/usr/local"} {
		if _, err := os.Stat(p + "/bin/brew"); err == nil {
			return p
		}
	}
	return ""
}

func goBinDir(home string) string {
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
	if home != "" {
		return filepath.Join(home, "go", "bin")
	}
	return ""
}

func tildeify(dir, home string) string {
	if home != "" && strings.HasPrefix(dir, home) {
		return "$HOME" + strings.TrimPrefix(dir, home)
	}
	return dir
}

// ProfilePath returns the shell profile to edit, based on the user's shell.
// zsh has been the macOS default since Catalina, so that is the fallback.
func ProfilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch filepath.Base(os.Getenv("SHELL")) {
	case "bash":
		return filepath.Join(home, ".bash_profile"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		return filepath.Join(home, ".zprofile"), nil
	}
}

// AlreadyPresent reports whether a profile already contains a line. Prevents
// appending the same export on every run, which is how profiles end up with a
// dozen copies of the same thing.
func AlreadyPresent(profile, line string) (bool, error) {
	f, err := os.Open(profile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	want := strings.TrimSpace(line)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == want {
			return true, nil
		}
	}
	return false, sc.Err()
}

// Append adds lines to a shell profile, backing it up first.
//
// Only ever called after explicit consent. Lines already present are skipped
// rather than duplicated, and the backup means a mistake here is recoverable
// without the user having to reconstruct their profile from memory.
func Append(profile string, entries []Entry) (added []Entry, backup string, err error) {
	var pending []Entry
	for _, e := range entries {
		present, err := AlreadyPresent(profile, e.Line)
		if err != nil {
			return nil, "", err
		}
		if !present {
			pending = append(pending, e)
		}
	}
	if len(pending) == 0 {
		return nil, "", nil
	}

	if _, statErr := os.Stat(profile); statErr == nil {
		backup = profile + ".macwtf.bak"
		data, err := os.ReadFile(profile)
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", profile, err)
		}
		if err := os.WriteFile(backup, data, 0o644); err != nil {
			return nil, "", fmt.Errorf("back up %s: %w", profile, err)
		}
	} else if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		return nil, "", err
	}

	f, err := os.OpenFile(profile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", profile, err)
	}
	defer f.Close()

	var b strings.Builder
	b.WriteString("\n# Added by macwtf\n")
	for _, e := range pending {
		b.WriteString(e.Line + "\n")
	}
	if _, err := f.WriteString(b.String()); err != nil {
		return nil, "", fmt.Errorf("write %s: %w", profile, err)
	}
	return pending, backup, nil
}
