package resolve

import (
	"strings"
	"testing"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// realCatalogue loads the shipped manifests, so these tests exercise the data
// users actually get rather than fixtures that can drift from it.
func realCatalogue(t *testing.T) *manifest.Catalogue {
	t.Helper()
	cat, err := manifest.Load("")
	if err != nil {
		t.Fatalf("load catalogue: %v", err)
	}
	return cat
}

func ids(tools []*manifest.Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.ID
	}
	return out
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}

func TestProfileExpandsIncludes(t *testing.T) {
	cat := realCatalogue(t)
	res, err := Resolve(cat, Request{Profile: "recon"})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(res.Install)

	// recon includes baseline, so baseline's tools must be present.
	for _, want := range []string{"ripgrep", "jq", "nmap", "ffuf"} {
		if indexOf(got, want) < 0 {
			t.Errorf("expected %q in resolved recon profile, got %v", want, got)
		}
	}
}

// A tool that requires another must be installed after it, or the install
// breaks in a way that is tedious to debug.
func TestDependencyOrdering(t *testing.T) {
	cat := realCatalogue(t)
	res, err := Resolve(cat, Request{Tools: []string{"rustscan"}})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(res.Install)

	nmapAt, rustscanAt := indexOf(got, "nmap"), indexOf(got, "rustscan")
	if nmapAt < 0 {
		t.Fatalf("nmap should have been pulled in as a dependency, got %v", got)
	}
	if nmapAt > rustscanAt {
		t.Fatalf("nmap must install before rustscan, got %v", got)
	}
}

// Asking for a single tool must still pull its dependencies in.
func TestTransitiveDependencies(t *testing.T) {
	cat := realCatalogue(t)
	res, err := Resolve(cat, Request{Tools: []string{"rustscan"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Install) != 2 {
		t.Fatalf("expected rustscan + nmap, got %v", ids(res.Install))
	}
}

// A catalogue entry with no macOS block is not part of the macOS catalogue at
// all — not hidden, not skipped, simply absent. aircrack-ng exists in the
// shared manifests for KaliWTF and WindowsWTF and must be invisible here.
func TestEntryWithoutMacOSBlockIsAbsent(t *testing.T) {
	cat := realCatalogue(t)

	if _, found := cat.Tool("aircrack-ng"); found {
		t.Fatal("aircrack-ng has no macOS block and must not be in the catalogue")
	}
	for _, tl := range cat.Tools {
		if tl.ID == "aircrack-ng" {
			t.Fatal("aircrack-ng leaked into the tool list")
		}
	}
	if cat.OtherPlatform == 0 {
		t.Error("expected the skipped entry to be counted")
	}

	// Asking for it by name is an unknown-tool error, not a skip.
	if _, err := Resolve(cat, Request{Tools: []string{"aircrack-ng"}}); err == nil ||
		!strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected an unknown-tool error, got %v", err)
	}
}

// sec-network contains a conflicting pair, so it exercises conflict handling.
func TestConflictsAreResolvedDeterministically(t *testing.T) {
	cat := realCatalogue(t)
	res, err := Resolve(cat, Request{Category: "sec-network"})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(res.Install)

	hasWireshark := indexOf(got, "wireshark") >= 0
	hasTshark := indexOf(got, "tshark") >= 0
	if hasWireshark && hasTshark {
		t.Fatalf("wireshark and tshark conflict and must not both install: %v", got)
	}
	if !hasWireshark && !hasTshark {
		t.Fatalf("expected exactly one of wireshark/tshark, got %v", got)
	}

	var sawConflict bool
	for _, s := range res.Skipped {
		if s.Reason == SkipConflict {
			sawConflict = true
		}
	}
	if !sawConflict {
		t.Errorf("expected a conflict skip to be reported, got %+v", res.Skipped)
	}
	for _, id := range got {
		if id == "aircrack-ng" {
			t.Error("aircrack-ng must not appear: it has no macOS block")
		}
	}
}

// Resolution must be deterministic: the same request twice must give the same
// order, or state files and dry-run output become unstable.
func TestResolutionIsDeterministic(t *testing.T) {
	cat := realCatalogue(t)
	var first []string
	for i := 0; i < 20; i++ {
		res, err := Resolve(cat, Request{Profile: "web"})
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(ids(res.Install), ",")
		if i == 0 {
			first = []string{got}
			continue
		}
		if got != first[0] {
			t.Fatalf("nondeterministic resolution:\n  %s\n  %s", first[0], got)
		}
	}
}

func TestUnsupportedBackendIsReported(t *testing.T) {
	cat := realCatalogue(t)
	res, err := Resolve(cat, Request{
		Profile:           "baseline",
		SupportedBackends: map[manifest.Backend]bool{manifest.BackendBrew: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	// baseline contains maccy, a cask; with only brew supported it must be
	// reported rather than dropped.
	var found bool
	for _, s := range res.Skipped {
		if s.Tool.ID == "maccy" && s.Reason == SkipUnsupportedBackend {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected maccy skipped as unsupported backend, got %+v", res.Skipped)
	}
}

func TestUnknownSelectors(t *testing.T) {
	cat := realCatalogue(t)
	for _, tc := range []struct {
		name string
		req  Request
		want string
	}{
		{"profile", Request{Profile: "nope"}, "unknown profile"},
		{"category", Request{Category: "nope"}, "unknown or empty category"},
		{"tool", Request{Tools: []string{"nope"}}, "unknown tool"},
		{"nothing", Request{}, "nothing selected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Resolve(cat, tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestArchFiltering(t *testing.T) {
	intel := &manifest.Tool{
		ID: "x", Name: "x", Description: "d", Category: "cli",
		Backend: manifest.BackendBrew, Package: "x", Arch: []string{manifest.ArchAMD64},
	}
	cat := manifest.NewCatalogue([]*manifest.Tool{intel}, nil)

	res, err := Resolve(cat, Request{Tools: []string{"x"}, Arch: manifest.ArchARM64})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Install) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != SkipArch {
		t.Fatalf("expected arch skip, got install=%v skipped=%+v", ids(res.Install), res.Skipped)
	}
}

// A bulk selection must never install a package name that has not been
// confirmed to resolve. The catalogue was imported in bulk with guessed names,
// so this is the difference between a profile that works and one that dies
// partway through on a typo nobody caught.
func TestProfilesNeverPullInUnverified(t *testing.T) {
	cat := realCatalogue(t)
	for _, p := range cat.Profiles {
		res, err := Resolve(cat, Request{Profile: p.ID})
		if err != nil {
			t.Fatalf("%s: %v", p.ID, err)
		}
		for _, tl := range res.Install {
			if tl.Unverified {
				t.Errorf("profile %q would install unverified tool %q", p.ID, tl.ID)
			}
		}
	}
}

// Naming a tool explicitly is a deliberate choice, so an unverified name is the
// user's risk to take rather than something macWTF refuses outright.
func TestNamingAToolExplicitlyAllowsUnverified(t *testing.T) {
	cat := realCatalogue(t)

	var unverified string
	for _, tl := range cat.Tools {
		if tl.Unverified {
			unverified = tl.ID
			break
		}
	}
	if unverified == "" {
		t.Skip("no unverified tools in the catalogue")
	}

	res, err := Resolve(cat, Request{Tools: []string{unverified}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Install) != 1 {
		t.Fatalf("expected the named tool to resolve, got %v", ids(res.Install))
	}
}
