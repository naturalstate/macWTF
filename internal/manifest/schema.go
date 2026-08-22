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

	// Sibling-platform backends. macWTF never executes these, but the
	// catalogue is shared, so the schema must accept them for entries that
	// also target Kali, Windows or Android.
	BackendApt    Backend = "apt"
	BackendWinget Backend = "winget"
	BackendChoco  Backend = "choco"
	BackendScoop  Backend = "scoop"
	BackendPacman Backend = "pacman"
	BackendAPK    Backend = "apk"
)

var backends = map[Backend]bool{
	BackendBrew: true, BackendCask: true, BackendMAS: true, BackendPipx: true,
	BackendCargo: true, BackendGo: true, BackendNPM: true, BackendGem: true,
	BackendCurl: true, BackendGit: true, BackendDefaults: true,
	BackendBuiltin: true, BackendManual: true,
	BackendApt: true, BackendWinget: true, BackendChoco: true,
	BackendScoop: true, BackendPacman: true, BackendAPK: true,
}

func (b Backend) Valid() bool { return backends[b] }

// macOSBackends is the subset this engine can actually execute. The shared
// catalogue accepts apt, winget and friends for sibling platforms, but a
// [tool.macos] block naming one of those is a data error: macWTF would load the
// tool and then be unable to install it.
var macOSBackends = map[Backend]bool{
	BackendBrew: true, BackendCask: true, BackendMAS: true, BackendPipx: true,
	BackendCargo: true, BackendGo: true, BackendNPM: true, BackendGem: true,
	BackendCurl: true, BackendGit: true, BackendDefaults: true,
	BackendBuiltin: true, BackendManual: true,
}

// ValidForMacOS reports whether this backend can run on macOS.
func (b Backend) ValidForMacOS() bool { return macOSBackends[b] }

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

// Platform names an operating system in the shared catalogue.
//
// The catalogue is shared across the WTF family — macWTF, KaliWTF, WindowsWTF,
// AndroidWTF — because a tool's identity, description and category are the same
// everywhere, and curating that four times is how sibling projects drift apart.
// Only the installation mechanism differs, so only that is per-platform.
//
// Shared data, not shared code: each engine stays independent.
type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformKali    Platform = "kali"
	PlatformWindows Platform = "windows"
	PlatformAndroid Platform = "android"
)

// ThisPlatform is the platform this engine installs for. A catalogue entry with
// no block for it is not loaded at all: macWTF never shows a tool it cannot
// install, rather than listing it as unavailable.
const ThisPlatform = PlatformMacOS

// PlatformSpec is how one operating system installs a tool. Every field here
// is genuinely platform-specific — a cask token means nothing to apt, and
// Gatekeeper quarantine is a macOS concept.
type PlatformSpec struct {
	Backend Backend `toml:"backend"`

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
	ManualSteps []string `toml:"manual_steps"`

	VerifyCmd   string   `toml:"verify_cmd"`
	PostInstall []string `toml:"post_install"`

	// Notes adds platform-specific detail to the shared description.
	Notes string `toml:"notes"`
}

// entry is the on-disk shape of a catalogue item: shared identity at the top
// level, one optional block per platform beneath it.
type entry struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Category    string `toml:"category"`

	License  License `toml:"license"`
	Homepage string  `toml:"homepage"`
	Notes    string  `toml:"notes"`

	ConflictsWith []string `toml:"conflicts_with"`
	Requires      []string `toml:"requires"`

	MacOS   *PlatformSpec `toml:"macos"`
	Kali    *PlatformSpec `toml:"kali"`
	Windows *PlatformSpec `toml:"windows"`
	Android *PlatformSpec `toml:"android"`
}

// spec returns the block for a platform, or nil if the tool has none.
func (e *entry) spec(p Platform) *PlatformSpec {
	switch p {
	case PlatformMacOS:
		return e.MacOS
	case PlatformKali:
		return e.Kali
	case PlatformWindows:
		return e.Windows
	case PlatformAndroid:
		return e.Android
	}
	return nil
}

// platforms lists every platform this entry supports, in stable order.
func (e *entry) platforms() []Platform {
	var out []Platform
	for _, p := range []Platform{PlatformMacOS, PlatformKali, PlatformWindows, PlatformAndroid} {
		if e.spec(p) != nil {
			out = append(out, p)
		}
	}
	return out
}

// Tool is a catalogue entry resolved for this platform: shared identity
// flattened together with this platform's installation details.
//
// Downstream code — the resolver, the backends, the TUI — works with this and
// never has to know the catalogue is multi-platform.
type Tool struct {
	ID          string
	Name        string
	Description string
	Category    string

	Backend Backend
	Package string
	Tap     string

	Arch            []string
	RequiresRosetta bool

	QuarantineStrip bool
	AppPath         string

	TCCPermissions []TCC
	ManualSteps    []string

	VerifyCmd   string
	PostInstall []string

	ConflictsWith []string
	Requires      []string

	License  License
	Homepage string
	Notes    string

	// AlsoOn lists the other platforms this tool is available on. Carried
	// through for the website and for the lab bridge, which needs to know
	// what a Kali guest could run.
	AlsoOn []Platform

	// SourceFile records which manifest file this came from, for error
	// messages. Populated by the loader, never present in TOML.
	SourceFile string
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
	Tool []entry `toml:"tool"`
}

// EverythingID is the synthetic profile covering the whole catalogue.
const EverythingID = "everything"

// Profile is a named list of tool ids, optionally composed from other profiles.
type Profile struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Description string `toml:"description"`

	// Includes names other profiles to compose in.
	Includes []string `toml:"includes"`

	// Categories pulls in every tool in a category. This is how the
	// profile table actually reads — "Pentest = Baseline + Recon + Web +
	// Passwords + Network" — and it means a profile does not go stale as
	// the catalogue grows: adding a recon tool joins Pentest automatically
	// rather than requiring an edit to a list of eighty ids.
	Categories []string `toml:"categories"`

	// Tools names individual tools, for the picks that do not follow a
	// whole category.
	Tools []string `toml:"tools"`

	// Excludes removes tools a category pulled in that do not belong. Lets
	// a profile take a category minus its heaviest members without
	// enumerating the rest.
	Excludes []string `toml:"excludes"`

	SourceFile string `toml:"-"`

	// Synthetic marks a profile generated by the engine rather than
	// authored in TOML. Only "everything" is synthetic: writing it by hand
	// would mean re-listing every tool on every catalogue change, and it
	// would be stale the moment someone forgot.
	Synthetic bool `toml:"-"`

	// Warning is shown before applying the profile. Used by "everything",
	// which is a much bigger commitment than its one-word name suggests.
	Warning string `toml:"-"`
}

// profileFile is the on-disk shape of a profile file.
type profileFile struct {
	Profile Profile `toml:"profile"`
}

// Catalogue is the fully loaded registry: every tool and profile, indexed.
type Catalogue struct {
	Tools    []*Tool
	Profiles []*Profile

	// OtherPlatform counts entries skipped because they have no block for
	// this platform. Reported by validate so a contributor can tell the
	// difference between "not in the catalogue" and "not for macOS".
	OtherPlatform int

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

// NewCatalogue assembles a catalogue from tools and profiles directly, without
// reading TOML. Used by tests and by any caller synthesising a selection —
// notably the TUI's export path.
//
// Duplicate ids keep the first occurrence, matching the loader, so that
// Validate reports the duplication rather than the index silently disagreeing
// with the slice.
func NewCatalogue(tools []*Tool, profiles []*Profile) *Catalogue {
	c := &Catalogue{
		Tools:       tools,
		Profiles:    profiles,
		byID:        make(map[string]*Tool, len(tools)),
		profileByID: make(map[string]*Profile, len(profiles)),
	}
	for _, t := range tools {
		if _, dup := c.byID[t.ID]; !dup {
			c.byID[t.ID] = t
		}
	}
	for _, p := range profiles {
		if _, dup := c.profileByID[p.ID]; !dup {
			c.profileByID[p.ID] = p
		}
	}
	return c
}
