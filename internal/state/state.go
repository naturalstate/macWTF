// Package state records what macWTF has installed.
//
// The critical field is Preexisting. If a user already had jq for three years
// and then ran a profile containing jq, removing that profile must not take
// their jq. Without recording what was already present before a run, uninstall
// cannot tell the difference between "we installed this" and "this was here",
// and would delete things it never added.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Version is the state file schema version, so a future change can migrate
// rather than silently misread an old file.
const Version = 1

// Record is one tool macWTF has seen.
type Record struct {
	ID      string           `toml:"id"`
	Backend manifest.Backend `toml:"backend"`
	Package string           `toml:"package"`

	// Preexisting means the package was already installed before macWTF
	// touched it. Such tools are never removed by `macwtf remove`.
	Preexisting bool `toml:"preexisting"`

	// QuarantineStripped records that a Gatekeeper quarantine attribute was
	// removed, since that is a security-relevant change worth an audit
	// trail even though it cannot meaningfully be undone.
	QuarantineStripped bool `toml:"quarantine_stripped"`

	InstalledAt time.Time `toml:"installed_at"`

	// Failed records a tool whose install was attempted and did not
	// succeed, so a re-run can retry it rather than assuming it is present.
	Failed bool   `toml:"failed,omitempty"`
	Error  string `toml:"error,omitempty"`
}

// State is the whole file.
type State struct {
	Version   int      `toml:"version"`
	Installed []Record `toml:"installed"`

	path string
}

// DefaultPath is ~/.macwtf/state.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".macwtf", "state.toml"), nil
}

// Load reads the state file. A missing file is not an error — it means nothing
// has been installed yet.
func Load(path string) (*State, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	s := &State{Version: Version, path: path}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := toml.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.path = path

	if s.Version > Version {
		return nil, fmt.Errorf("%s was written by a newer macwtf (schema v%d, this build understands v%d)",
			path, s.Version, Version)
	}
	return s, nil
}

// Record looks up a tool.
func (s *State) Record(id string) (Record, bool) {
	for _, r := range s.Installed {
		if r.ID == id {
			return r, true
		}
	}
	return Record{}, false
}

// Put inserts or replaces a record.
func (s *State) Put(r Record) {
	for i := range s.Installed {
		if s.Installed[i].ID == r.ID {
			s.Installed[i] = r
			return
		}
	}
	s.Installed = append(s.Installed, r)
}

// Remove deletes a record.
func (s *State) Remove(id string) {
	for i := range s.Installed {
		if s.Installed[i].ID == id {
			s.Installed = append(s.Installed[:i], s.Installed[i+1:]...)
			return
		}
	}
}

// Removable returns the tools macWTF actually installed — never those that
// were already present.
func (s *State) Removable() []Record {
	var out []Record
	for _, r := range s.Installed {
		if !r.Preexisting && !r.Failed {
			out = append(out, r)
		}
	}
	return out
}

// Save writes the file atomically, so an interrupted write cannot leave a
// truncated state file and lose the record of everything installed.
func (s *State) Save() error {
	s.Version = Version
	sort.Slice(s.Installed, func(i, j int) bool { return s.Installed[i].ID < s.Installed[j].ID })

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	raw, err := toml.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.toml")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

// Path returns where this state lives.
func (s *State) Path() string { return s.path }
