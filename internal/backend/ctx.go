package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Ctx is the environment a plan is built against: where Homebrew lives, what
// architecture we are on, and what the user has consented to.
//
// It is deliberately constructible by hand so that plan generation can be
// tested without a Homebrew installation, or indeed any subprocess at all.
type Ctx struct {
	// BrewPrefix is queried from `brew --prefix`, never hardcoded. It is
	// /opt/homebrew on Apple Silicon and /usr/local on Intel, and assuming
	// either one is a bug.
	BrewPrefix string

	Arch string

	DryRun    bool
	AssumeYes bool

	// AllowQuarantineStrip gates the xattr step. Off by default: removing
	// com.apple.quarantine bypasses a Gatekeeper check and must be a
	// conscious choice, never a side effect of installing something.
	AllowQuarantineStrip bool

	// DataDir holds wordlists and payloads. ~/.local/share/macwtf, not
	// /opt — /opt is a Linux convention and clutters macOS.
	DataDir string

	// StateDir holds state.toml. ~/.macwtf.
	StateDir string

	// installedCache memoises per-backend Installed() snapshots so that a
	// 400-tool run costs one `brew list` rather than four hundred.
	installedCache map[manifest.Backend]map[string]bool
	cacheMu        sync.Mutex
}

// NewCtx builds a context by probing the live system.
func NewCtx() (*Ctx, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate home directory: %w", err)
	}

	c := &Ctx{
		Arch:           runtime.GOARCH,
		DataDir:        filepath.Join(home, ".local", "share", "macwtf"),
		StateDir:       filepath.Join(home, ".macwtf"),
		installedCache: map[manifest.Backend]map[string]bool{},
	}
	if runtime.GOARCH == "arm64" {
		c.Arch = manifest.ArchARM64
	} else {
		c.Arch = manifest.ArchAMD64
	}

	// Homebrew installs to /opt/homebrew on Apple Silicon, which is not on
	// the default PATH. Any shell that has not sourced `brew shellenv` — a
	// fresh Terminal window, a non-login SSH session, an app launched from
	// Finder — cannot see it, and every brew-backed tool would be reported
	// as unavailable on a machine where Homebrew is installed and working.
	//
	// Done here rather than in a single command so that every entry point
	// benefits: CLI, TUI and doctor alike.
	adoptBrewPath()

	// Best effort: a missing brew is a backend preflight failure, not a
	// fatal error, since not every backend needs it.
	if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
		c.BrewPrefix = strings.TrimSpace(string(out))
	}
	return c, nil
}

// brewPrefixes are the standard Homebrew locations, newest convention first.
var brewPrefixes = []string{"/opt/homebrew", "/usr/local"}

// adoptBrewPath puts an installed-but-unreachable Homebrew on this process's
// PATH. It changes nothing on disk and nothing outside this process.
func adoptBrewPath() {
	if _, err := exec.LookPath("brew"); err == nil {
		return
	}
	for _, prefix := range brewPrefixes {
		bin := prefix + "/bin"
		if _, err := os.Stat(bin + "/brew"); err != nil {
			continue
		}
		os.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		return
	}
}

// NewTestCtx builds a context with no system probing, for tests.
func NewTestCtx() *Ctx {
	return &Ctx{
		BrewPrefix:     "/opt/homebrew",
		Arch:           manifest.ArchARM64,
		DataDir:        "/tmp/macwtf-test/share",
		StateDir:       "/tmp/macwtf-test/state",
		installedCache: map[manifest.Backend]map[string]bool{},
	}
}

// ResetInstalledCache drops the memoised per-backend snapshots, so the next
// query re-probes the system. Needed after a run: the whole point of the cache
// is that one `brew list` serves a whole plan, which makes it stale the moment
// anything is installed.
func (c *Ctx) ResetInstalledCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.installedCache = map[manifest.Backend]map[string]bool{}
}

// SeedInstalled injects a known installed set for a backend, bypassing the
// live probe. Used by tests, and by --dry-run when no probe is possible.
func (c *Ctx) SeedInstalled(b manifest.Backend, pkgs map[string]bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.installedCache == nil {
		c.installedCache = map[manifest.Backend]map[string]bool{}
	}
	c.installedCache[b] = pkgs
}

// InstalledFor returns the cached installed set for a backend, populating it
// via the backend on first use.
func (c *Ctx) InstalledFor(b Backend) (map[string]bool, error) {
	c.cacheMu.Lock()
	if set, ok := c.installedCache[b.ID()]; ok {
		c.cacheMu.Unlock()
		return set, nil
	}
	c.cacheMu.Unlock()

	set, err := b.Installed(c)
	if err != nil {
		return nil, err
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.installedCache == nil {
		c.installedCache = map[manifest.Backend]map[string]bool{}
	}
	c.installedCache[b.ID()] = set
	return set, nil
}
