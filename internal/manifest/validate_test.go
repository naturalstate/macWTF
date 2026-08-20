package manifest

import (
	"strings"
	"testing"
)

// TestRealCatalogueIsValid guards the shipped manifests. This is the test that
// fails when someone adds a tool with a dangling dependency or a mismatched
// category, and it runs entirely offline.
func TestRealCatalogueIsValid(t *testing.T) {
	cat, err := Load("")
	if err != nil {
		t.Fatalf("load embedded catalogue: %v", err)
	}
	if err := cat.Validate(); err != nil {
		t.Fatalf("embedded catalogue is invalid:\n%v", err)
	}
	if len(cat.Tools) == 0 {
		t.Fatal("embedded catalogue is empty")
	}
	if len(cat.Profiles) == 0 {
		t.Fatal("no profiles in embedded catalogue")
	}
}

// build assembles a catalogue in memory, bypassing TOML parsing so that
// validation rules can be tested in isolation.
func build(tools []Tool, profiles []Profile) *Catalogue {
	c := &Catalogue{byID: map[string]*Tool{}, profileByID: map[string]*Profile{}}
	for i := range tools {
		tl := tools[i]
		if tl.SourceFile == "" {
			tl.SourceFile = "manifest/" + tl.Category + ".toml"
		}
		c.Tools = append(c.Tools, &tl)
		if _, dup := c.byID[tl.ID]; !dup {
			c.byID[tl.ID] = &tl
		}
	}
	for i := range profiles {
		p := profiles[i]
		if p.SourceFile == "" {
			p.SourceFile = "profiles/" + p.ID + ".toml"
		}
		c.Profiles = append(c.Profiles, &p)
		if _, dup := c.profileByID[p.ID]; !dup {
			c.profileByID[p.ID] = &p
		}
	}
	return c
}

// ok is a minimal tool that passes every rule, used as a base to mutate.
func ok(id string) Tool {
	return Tool{
		ID: id, Name: id, Description: "d", Category: "cli",
		Backend: BackendBrew, Package: id, License: LicenseFree,
	}
}

func requireProblem(t *testing.T, c *Catalogue, want string) {
	t.Helper()
	err := c.Validate()
	if err == nil {
		t.Fatalf("expected a validation problem containing %q, got none", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected problem containing %q, got:\n%v", want, err)
	}
}

func requireValid(t *testing.T, c *Catalogue) {
	t.Helper()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid, got:\n%v", err)
	}
}

func TestValidBaseline(t *testing.T) {
	requireValid(t, build([]Tool{ok("nmap")}, []Profile{
		{ID: "p", Name: "P", Tools: []string{"nmap"}},
	}))
}

func TestDuplicateID(t *testing.T) {
	requireProblem(t, build([]Tool{ok("nmap"), ok("nmap")}, nil), "duplicate id")
}

func TestIDShape(t *testing.T) {
	bad := ok("Nmap_Scanner")
	requireProblem(t, build([]Tool{bad}, nil), "kebab-case")
}

func TestCategoryMustMatchFilename(t *testing.T) {
	tl := ok("nmap")
	tl.SourceFile = "manifest/sec-recon.toml" // but Category is "cli"
	requireProblem(t, build([]Tool{tl}, nil), "does not match filename")
}

func TestUnknownBackend(t *testing.T) {
	tl := ok("nmap")
	tl.Backend = "apt"
	requireProblem(t, build([]Tool{tl}, nil), "unknown backend")
}

func TestBackendNeedsPackage(t *testing.T) {
	tl := ok("nmap")
	tl.Package = ""
	requireProblem(t, build([]Tool{tl}, nil), "requires a package")
}

func TestBuiltinNeedsNoPackage(t *testing.T) {
	tl := ok("zsh")
	tl.Backend = BackendBuiltin
	tl.Package = ""
	requireValid(t, build([]Tool{tl}, nil))
}

func TestDanglingRequires(t *testing.T) {
	tl := ok("rustscan")
	tl.Requires = []string{"nmap"}
	requireProblem(t, build([]Tool{tl}, nil), `requires unknown tool "nmap"`)
}

func TestDanglingConflict(t *testing.T) {
	tl := ok("wireshark")
	tl.ConflictsWith = []string{"tshark"}
	requireProblem(t, build([]Tool{tl}, nil), "conflicts_with unknown tool")
}

func TestRequireCycle(t *testing.T) {
	a, b := ok("a"), ok("b")
	a.Requires = []string{"b"}
	b.Requires = []string{"a"}
	requireProblem(t, build([]Tool{a, b}, nil), "dependency cycle")
}

func TestSelfRequire(t *testing.T) {
	tl := ok("a")
	tl.Requires = []string{"a"}
	requireProblem(t, build([]Tool{tl}, nil), "requires itself")
}

func TestUnknownTCC(t *testing.T) {
	tl := ok("rectangle")
	tl.TCCPermissions = []TCC{"telepathy"}
	requireProblem(t, build([]Tool{tl}, nil), "unknown tcc permission")
}

// Quarantine stripping without a target is silently useless, which is worse
// than an error: the user consents to a security-relevant action that then
// does nothing.
func TestQuarantineNeedsAppPath(t *testing.T) {
	tl := ok("cutter")
	tl.QuarantineStrip = true
	requireProblem(t, build([]Tool{tl}, nil), "nothing to strip")
}

func TestProfileDanglingTool(t *testing.T) {
	c := build([]Tool{ok("nmap")}, []Profile{{ID: "p", Name: "P", Tools: []string{"ffuf"}}})
	requireProblem(t, c, `references unknown tool "ffuf"`)
}

func TestProfileDanglingInclude(t *testing.T) {
	c := build([]Tool{ok("nmap")}, []Profile{
		{ID: "p", Name: "P", Tools: []string{"nmap"}, Includes: []string{"baseline"}},
	})
	requireProblem(t, c, `includes unknown profile "baseline"`)
}

func TestProfileIncludeCycle(t *testing.T) {
	c := build([]Tool{ok("nmap")}, []Profile{
		{ID: "a", Name: "A", Tools: []string{"nmap"}, Includes: []string{"b"}},
		{ID: "b", Name: "B", Tools: []string{"nmap"}, Includes: []string{"a"}},
	})
	requireProblem(t, c, "profile include cycle")
}

func TestEmptyProfile(t *testing.T) {
	c := build([]Tool{ok("nmap")}, []Profile{{ID: "p", Name: "P"}})
	requireProblem(t, c, "profile is empty")
}

// Validate must report every problem in one pass. Bailing on the first means a
// contributor fixes one typo, re-runs, and finds another — repeatedly.
func TestReportsAllProblems(t *testing.T) {
	a, b := ok("a"), ok("b")
	a.Backend = "apt"
	b.Requires = []string{"nope"}
	err := build([]Tool{a, b}, nil).Validate()
	if err == nil {
		t.Fatal("expected problems")
	}
	ps, isProblems := err.(Problems)
	if !isProblems {
		t.Fatalf("expected Problems, got %T", err)
	}
	if len(ps) < 2 {
		t.Fatalf("expected at least 2 problems reported together, got %d:\n%v", len(ps), err)
	}
}

func TestSupportsArch(t *testing.T) {
	universal := ok("a")
	if !universal.SupportsArch(ArchARM64) || !universal.SupportsArch(ArchAMD64) {
		t.Fatal("empty arch should mean universal")
	}
	intel := ok("b")
	intel.Arch = []string{ArchAMD64}
	if intel.SupportsArch(ArchARM64) {
		t.Fatal("x86_64-only tool should not claim arm64 support")
	}
}
