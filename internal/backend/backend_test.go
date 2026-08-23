package backend

import (
	"strings"
	"testing"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// tool builds a minimal Tool for command-shape assertions.
func tool(id, pkg string) *manifest.Tool {
	return &manifest.Tool{ID: id, Name: id, Package: pkg}
}

// A Go module path is not the binary name. The /vN major-version suffix and a
// trailing cmd/<name> both have to be handled, or nothing installed through the
// go backend would ever be detected as present and every run would reinstall.
func TestBinaryName(t *testing.T) {
	for _, tc := range []struct{ pkg, want string }{
		{"github.com/ffuf/ffuf/v2@latest", "ffuf"},
		{"github.com/ffuf/ffuf/v2", "ffuf"},
		{"github.com/projectdiscovery/subfinder/v2/cmd/subfinder", "subfinder"},
		{"github.com/projectdiscovery/httpx/cmd/httpx", "httpx"},
		{"github.com/hahwul/dalfox/v2", "dalfox"},
		{"github.com/OJ/gobuster/v3@v3.6.0", "gobuster"},
		{"github.com/tomnomnom/waybackurls", "waybackurls"},
		{"golang.org/x/tools/cmd/goimports@latest", "goimports"},
	} {
		if got := BinaryName(tc.pkg); got != tc.want {
			t.Errorf("BinaryName(%q) = %q, want %q", tc.pkg, got, tc.want)
		}
	}
}

// go install refuses a bare module path in module-aware mode, so a version
// suffix has to be added when the manifest does not pin one.
func TestGoInstallAddsVersion(t *testing.T) {
	g := &Golang{}
	ctx := NewTestCtx()

	steps, err := g.InstallPlan(tool("subfinder", "github.com/projectdiscovery/subfinder/v2/cmd/subfinder"), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := steps[0].String(); got != "go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest" {
		t.Fatalf("expected @latest to be appended, got %q", got)
	}

	// A pinned version must be left alone.
	steps, _ = g.InstallPlan(tool("x", "example.com/x@v1.2.3"), ctx)
	if got := steps[0].String(); got != "go install example.com/x@v1.2.3" {
		t.Fatalf("a pinned version must be preserved, got %q", got)
	}
}

func TestPipxAndCargoCommands(t *testing.T) {
	ctx := NewTestCtx()
	for _, tc := range []struct {
		b    Backend
		want string
	}{
		{&Pipx{}, "pipx install impacket"},
		{&Cargo{}, "cargo install impacket"},
		{&NPM{}, "npm install -g impacket"},
	} {
		steps, err := tc.b.InstallPlan(tool("impacket", "impacket"), ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := steps[0].String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

// Every registered backend must be able to answer "is this already there",
// either by key against a snapshot or by deciding for itself. A backend that
// can do neither silently reinstalls on every run.
func TestEveryBackendCanDetectExistingState(t *testing.T) {
	for id, impl := range NewRegistry() {
		if impl.ID() != id {
			t.Errorf("registry key %q does not match backend ID %q", id, impl.ID())
		}

		_, selfChecking := impl.(interface {
			IsApplied(*manifest.Tool) bool
		})
		if selfChecking {
			// A preference is never absent, only set to some value,
			// so there is nothing to key a snapshot on.
			continue
		}
		if impl.InstalledKey(tool("x", "pkg")) == "" {
			t.Errorf("backend %q has no InstalledKey and does not self-check", id)
		}
	}
}

// A defaults entry must be reversible, and the revert must be expressed as data
// so it can be checked rather than trusted.
func TestDefaultsRevert(t *testing.T) {
	d := &Defaults{}
	ctx := NewTestCtx()

	tl := &manifest.Tool{
		ID: "finder-hidden", Package: "com.apple.finder",
		Key: "AppleShowAllFiles", Value: "true", ValueType: "bool", Revert: "false",
	}
	steps, err := d.InstallPlan(tl, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := steps[0].String(); got != "defaults write com.apple.finder AppleShowAllFiles -bool true" {
		t.Fatalf("unexpected write: %q", got)
	}

	rev, err := d.RemovePlan(tl, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := rev[0].String(); got != "defaults write com.apple.finder AppleShowAllFiles -bool false" {
		t.Fatalf("unexpected revert: %q", got)
	}

	// "delete" restores whatever the system default is.
	tl.Revert = "delete"
	rev, _ = d.RemovePlan(tl, ctx)
	if got := rev[0].String(); got != "defaults delete com.apple.finder AppleShowAllFiles" {
		t.Fatalf("unexpected delete revert: %q", got)
	}
}

// The curl backend has to handle the several shapes a release asset arrives in.
func TestCurlHandlesArchiveShapes(t *testing.T) {
	c := &Curl{}
	ctx := NewTestCtx()

	for _, tc := range []struct{ url, want string }{
		{"https://example.com/tool-v1.tar.gz", "tar xf"},
		{"https://example.com/tool.zip", "unzip"},
		{"https://example.com/tool", "chmod +x"},
	} {
		tl := &manifest.Tool{ID: "tool", Name: "tool", Package: tc.url, Binary: "tool"}
		steps, err := c.InstallPlan(tl, ctx)
		if err != nil {
			t.Fatalf("%s: %v", tc.url, err)
		}
		var all string
		for _, s := range steps {
			all += s.String() + "\n"
		}
		if !strings.Contains(all, tc.want) {
			t.Errorf("%s: expected %q in plan:\n%s", tc.url, tc.want, all)
		}
	}

	// A disk image needs to know what to copy out of it.
	tl := &manifest.Tool{ID: "x", Name: "x", Package: "https://example.com/x.dmg"}
	if _, err := c.InstallPlan(tl, ctx); err == nil {
		t.Error("a .dmg without app_path should be rejected, not guessed at")
	}
}
