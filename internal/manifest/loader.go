package manifest

import (
	"fmt"
	"errors"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	macwtf "github.com/naturalstate/macWTF"
)

// ManifestDirEnv overrides the embedded catalogue with a directory on disk.
// The directory is expected to contain manifest/ and profiles/ subdirectories,
// i.e. it points at the root of a working checkout.
const ManifestDirEnv = "MACWTF_MANIFEST_DIR"

// Load reads the catalogue. If dir is non-empty it is read from disk;
// otherwise MACWTF_MANIFEST_DIR is consulted; otherwise the copy embedded in
// the binary is used. The result is unvalidated — call Validate separately, so
// that callers who only want to list tools do not pay for integrity checking
// and so that validation errors can be reported all at once.
func Load(dir string) (*Catalogue, error) {
	if dir == "" {
		dir = os.Getenv(ManifestDirEnv)
	}

	var fsys fs.FS
	if dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("manifest dir %q: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("manifest dir %q is not a directory", dir)
		}
		fsys = os.DirFS(dir)
	} else {
		fsys = macwtf.Catalogue
	}

	c := &Catalogue{
		byID:        map[string]*Tool{},
		profileByID: map[string]*Profile{},
	}

	if err := c.loadTools(fsys); err != nil {
		return nil, err
	}
	if err := c.loadProfiles(fsys); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalogue) loadTools(fsys fs.FS) error {
	files, err := tomlFiles(fsys, "manifest")
	if err != nil {
		return err
	}
	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		var tf toolFile
		dec := toml.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&tf); err != nil {
			return fmt.Errorf("%s: %w", name, decodeHint(err))
		}

		for i := range tf.Tool {
			t := tf.Tool[i]
			t.SourceFile = name
			c.Tools = append(c.Tools, &t)
			// Duplicate ids are a validation error, not a load error, so
			// that Validate can report every one rather than the first.
			if _, exists := c.byID[t.ID]; !exists {
				c.byID[t.ID] = &t
			}
		}
	}
	return nil
}

func (c *Catalogue) loadProfiles(fsys fs.FS) error {
	files, err := tomlFiles(fsys, "profiles")
	if err != nil {
		return err
	}
	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		var pf profileFile
		dec := toml.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&pf); err != nil {
			return fmt.Errorf("%s: %w", name, decodeHint(err))
		}

		p := pf.Profile
		p.SourceFile = name
		c.Profiles = append(c.Profiles, &p)
		if _, exists := c.profileByID[p.ID]; !exists {
			c.profileByID[p.ID] = &p
		}
	}
	return nil
}

// tomlFiles lists the .toml files in a directory, sorted, so that load order
// and therefore error output is deterministic. A missing directory is not an
// error: a checkout may legitimately have no profiles yet.
func tomlFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		out = append(out, path.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// decodeHint unwraps go-toml's structured error into something that points at
// the offending line, since the most common authoring mistake is a typo'd key
// and DisallowUnknownFields turns that into a hard failure by design.
func decodeHint(err error) error {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		return fmt.Errorf("unknown field(s) — check for typos, or extend the schema if this is a genuinely new attribute:\n%s", strict.String())
	}
	var derr *toml.DecodeError
	if errors.As(err, &derr) {
		row, col := derr.Position()
		return fmt.Errorf("line %d col %d: %s\n%s", row, col, derr.Error(), derr.String())
	}
	return err
}
