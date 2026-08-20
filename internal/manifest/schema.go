package manifest

import (
	"fmt"
	"sort"
)

// Backend names the installation mechanism a tool uses. Homebrew is one
// backend among many, not a privileged default.
type Backend string

const (
	BackendBrew     Backend = "brew"
	BackendCask     Backend = "cask"
	BackendMAS      Backend = "mas"
	BackendPipx     Backend = "pipx"
	BackendCargo    Backend = "cargo"
	BackendGo       Backend = "go"
	BackendNPM      Backend = "npm"
	BackendGem      Backend = "gem"
	BackendCurl     Backend = "curl"
	BackendGit      Backend = "git"
	BackendDefaults Backend = "defaults"
	BackendBuiltin  Backend = "builtin"
	BackendManual   Backend = "manual"
)

var backends = map[Backend]bool{
	BackendBrew: true, BackendCask: true, BackendMAS: true, BackendPipx: true,
	BackendCargo: true, BackendGo: true, BackendNPM: true, BackendGem: true,
	BackendCurl: true, BackendGit: true, BackendDefaults: true,
	BackendBuiltin: true, BackendManual: true,
}

func (b Backend) Valid() bool { return backends[b] }

// TCC names a privacy permission that macOS will not let any installer grant
// programmatically. Every one of these costs the user a manual trip to System
// Settings, which is why they are collected and reported rather than attempted.
type TCC string

const (
	TCCFullDisk        TCC = "full-disk"
	TCCScreenRecording TCC = "screen-recording"
	TCCAccessibility   TCC = "accessibility"
	TCCInputMonitoring TCC = "input-monitoring"
	TCCDeveloperTools  TCC = "developer-tools"
	TCCCamera          TCC = "camera"
	TCCMicrophone      TCC = "microphone"
	TCCLocation        TCC = "location"
)

// Pane is the System Settings location where a permission is granted, together
// with the deep link that opens it. Reported verbatim to the user at the end of
// a run; the whole point is that they should not have to go hunting.
type Pane struct {
	Name string
	URL  string
}

var tccPanes = map[TCC]Pane{
	TCCFullDisk: {
		"Privacy & Security → Full Disk Access",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_AllFiles",
	},
	TCCScreenRecording: {
		"Privacy & Security → Screen & System Audio Recording",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_ScreenCapture",
	},
	TCCAccessibility: {
		"Privacy & Security → Accessibility",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_Accessibility",
	},
	TCCInputMonitoring: {
		"Privacy & Security → Input Monitoring",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_ListenEvent",
	},
	TCCDeveloperTools: {
		"Privacy & Security → Developer Tools",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_DevTools",
	},
	TCCCamera: {
		"Privacy & Security → Camera",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_Camera",
	},
	TCCMicrophone: {
		"Privacy & Security → Microphone",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_Microphone",
	},
	TCCLocation: {
		"Privacy & Security → Location Services",
		"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_LocationServices",
	},
}

func (t TCC) Valid() bool    { _, ok := tccPanes[t]; return ok }
func (t TCC) Pane() Pane     { return tccPanes[t] }
func (t TCC) String() string { return string(t) }

// License records how a tool is licensed, so the TUI can warn before installing
// something that will demand payment or a key before it does anything useful.
type License string

const (
	LicenseFree     License = "free"
	LicensePaid     License = "paid"
	LicenseFreemium License = "freemium"
	LicenseTrial    License = "trial"
	LicenseUnknown  License = ""
)

var licenses = map[License]bool{
	LicenseFree: true, LicensePaid: true, LicenseFreemium: true,
	LicenseTrial: true, LicenseUnknown: true,
}

func (l License) Valid() bool { return licenses[l] }

// Arch constrains a tool to a CPU architecture. Empty means universal.
const (
	ArchARM64 = "arm64"
	ArchAMD64 = "x86_64"
)

// Tool is one catalogue entry. Every field that could vary between tools lives
// here rather than in code: if installing something new would require a change
// to the engine, this struct is missing a field.
type Tool struct {
	ID          string  `toml:"id"`
	Name        string  `toml:"name"`
	Description string  `toml:"description"`
	Category    string  `toml:"category"`
	Backend     Backend `toml:"backend"`

	// Package is the backend-specific identifier: a formula name, a cask
	// token, a numeric App Store id, a PyPI name, a Go module path, a URL.
	Package string `toml:"package"`

	// Tap is an optional third-party Homebrew tap to add before installing.
	Tap string `toml:"tap"`

	Arch            []string `toml:"arch"`
	RequiresRosetta bool     `toml:"requires_rosetta"`

	// QuarantineStrip marks a tool as unsigned or ad-hoc signed, such that
	// Gatekeeper will refuse to launch it until com.apple.quarantine is
	// removed. Never acted on without explicit consent.
	QuarantineStrip bool `toml:"quarantine_strip"`

	// AppPath is the installed bundle or binary, used as the target for
	// quarantine stripping and as the default verification check.
	AppPath string `toml:"app_path"`

	TCCPermissions []TCC `toml:"tcc_permissions"`

	// ManualSteps are anything else a human must do by hand: kernel
	// extension approval, license activation, a helper tool install.
	// These join the TCC entries in the end-of-run report.
	ManualSteps []string `toml:"manual_steps"`

	VerifyCmd   string   `toml:"verify_cmd"`
	PostInstall []string `toml:"post_install"`

	ConflictsWith []string `toml:"conflicts_with"`
	Requires      []string `toml:"requires"`

	License  License `toml:"license"`
	Homepage string  `toml:"homepage"`
	Notes    string  `toml:"notes"`

	// LinuxOnly marks tools that install cleanly but cannot actually do
	// their job on macOS — chiefly anything needing wireless monitor mode,
	// which has not worked on the internal card since Big Sur. Surfaced as
	// unavailable and routed to the lab bridge instead of installed.
	LinuxOnly bool `toml:"linux_only"`

	// SourceFile records which manifest file this came from, for error
	// messages. Populated by the loader, never present in TOML.
	SourceFile string `toml:"-"`
}

// NeedsManualSteps reports whether this tool contributes anything to the
// end-of-run report.
func (t *Tool) NeedsManualSteps() bool {
	return len(t.TCCPermissions) > 0 || len(t.ManualSteps) > 0 || t.QuarantineStrip
}

// SupportsArch reports whether the tool can run on the given architecture.
func (t *Tool) SupportsArch(arch string) bool {
	if len(t.Arch) == 0 {
		return true
	}
	for _, a := range t.Arch {
		if a == arch {
			return true
		}
	}
	return false
}

func (t *Tool) String() string { return fmt.Sprintf("%s (%s)", t.ID, t.Backend) }

// toolFile is the on-disk shape of a manifest file.
type toolFile struct {
	Tool []Tool `toml:"tool"`
}

// Profile is a named list of tool ids, optionally composed from other profiles.
type Profile struct {
	ID          string   `toml:"id"`
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Includes    []string `toml:"includes"`
	Tools       []string `toml:"tools"`

	SourceFile string `toml:"-"`
}

// profileFile is the on-disk shape of a profile file.
type profileFile struct {
	Profile Profile `toml:"profile"`
}

// Catalogue is the fully loaded registry: every tool and profile, indexed.
type Catalogue struct {
	Tools    []*Tool
	Profiles []*Profile

	byID        map[string]*Tool
	profileByID map[string]*Profile
}

// Tool looks up a tool by id.
func (c *Catalogue) Tool(id string) (*Tool, bool) { t, ok := c.byID[id]; return t, ok }

// Profile looks up a profile by id.
func (c *Catalogue) Profile(id string) (*Profile, bool) { p, ok := c.profileByID[id]; return p, ok }

// Categories returns every distinct category present, in sorted order.
func (c *Catalogue) Categories() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range c.Tools {
		if !seen[t.Category] {
			seen[t.Category] = true
			out = append(out, t.Category)
		}
	}
	sort.Strings(out)
	return out
}

// InCategory returns every tool in a category, in manifest order.
func (c *Catalogue) InCategory(cat string) []*Tool {
	var out []*Tool
	for _, t := range c.Tools {
		if t.Category == cat {
			out = append(out, t)
		}
	}
	return out
}
