package backend

import (
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

// Every registered backend must answer InstalledKey, or the idempotency check
// silently compares the wrong things.
func TestEveryBackendHasInstalledKey(t *testing.T) {
	for id, impl := range NewRegistry() {
		if impl.ID() != id {
			t.Errorf("registry key %q does not match backend ID %q", id, impl.ID())
		}
		if impl.InstalledKey(tool("x", "pkg")) == "" {
			t.Errorf("backend %q returned an empty InstalledKey", id)
		}
	}
}
