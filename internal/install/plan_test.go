package install

import (
	"strings"
	"testing"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
)

// testPlan builds a plan against a seeded installed-set, so no subprocess ever
// runs. Everything about plan generation is testable without Homebrew.
func testPlan(t *testing.T, req resolve.Request, installedFormulae, installedCasks []string, allowQuarantine bool) *Plan {
	t.Helper()

	cat, err := manifest.Load("")
	if err != nil {
		t.Fatalf("load catalogue: %v", err)
	}

	ctx := backend.NewTestCtx()
	ctx.AllowQuarantineStrip = allowQuarantine
	ctx.SeedInstalled(manifest.BackendBrew, set(installedFormulae))
	ctx.SeedInstalled(manifest.BackendCask, set(installedCasks))

	reg := backend.NewRegistry()
	supported := map[manifest.Backend]bool{}
	for b := range reg {
		supported[b] = true
	}
	req.SupportedBackends = supported
	if req.Arch == "" {
		req.Arch = manifest.ArchARM64
	}

	res, err := resolve.Resolve(cat, req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p, err := BuildPlan(res, reg, ctx)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	return p
}

func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func render(p *Plan) string {
	var b strings.Builder
	p.Render(&b, true)
	return b.String()
}

func TestPlanProducesExpectedCommands(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"nmap"}}, nil, nil, false)
	out := render(p)

	if !strings.Contains(out, "brew install --formula nmap") {
		t.Fatalf("expected the brew install command, got:\n%s", out)
	}
	if !strings.Contains(out, "command -v nmap") {
		t.Fatalf("expected the verify step, got:\n%s", out)
	}
}

func TestCaskProducesCaskCommand(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"burp-suite"}}, nil, nil, false)
	out := render(p)
	if !strings.Contains(out, "brew install --cask burp-suite") {
		t.Fatalf("expected a cask install, got:\n%s", out)
	}
}

// Idempotency: re-running with everything already present must produce no
// install steps at all.
func TestAlreadyInstalledIsNoOp(t *testing.T) {
	p := testPlan(t, resolve.Request{Profile: "recon"},
		[]string{"nmap", "masscan", "rustscan", "ffuf", "gobuster", "ripgrep", "fd", "bat", "jq", "eza"},
		[]string{"maccy"}, false)

	todo, already, failed := p.Counts()
	if todo != 0 || failed != 0 {
		t.Fatalf("expected a complete no-op, got todo=%d already=%d failed=%d\n%s",
			todo, already, failed, render(p))
	}
	if already == 0 {
		t.Fatal("expected tools to be reported as already installed")
	}
	if strings.Contains(render(p), "brew install") {
		t.Fatalf("no install command should appear:\n%s", render(p))
	}
}

// Quarantine stripping must not be planned unless explicitly allowed, and must
// be surfaced loudly when it is withheld.
func TestQuarantineWithheldByDefault(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"cutter"}}, nil, nil, false)
	out := render(p)

	if strings.Contains(out, "xattr -d -r com.apple.quarantine /Applications/Cutter.app\n   ") {
		t.Fatalf("quarantine strip must not be planned without consent:\n%s", out)
	}
	if len(p.PendingQuarantine()) != 1 {
		t.Fatalf("cutter should be reported as pending quarantine, got %v", p.PendingQuarantine())
	}
	if !strings.Contains(out, "--allow-quarantine-strip") {
		t.Fatalf("withheld quarantine must be explained to the user:\n%s", out)
	}
	if !strings.Contains(out, "waives a macOS malware check") {
		t.Fatalf("the security implication must be stated plainly:\n%s", out)
	}
}

func TestQuarantinePlannedWhenAllowed(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"cutter"}}, nil, nil, true)
	out := render(p)

	if !strings.Contains(out, "xattr -d -r com.apple.quarantine /Applications/Cutter.app") {
		t.Fatalf("expected the quarantine step when allowed, got:\n%s", out)
	}
	if len(p.PendingQuarantine()) != 0 {
		t.Fatal("nothing should remain pending once allowed")
	}
}

// A tool with no quarantine_strip flag must never get an xattr step, however
// the run is configured.
func TestNoQuarantineForSignedApps(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"burp-suite"}}, nil, nil, true)
	if strings.Contains(render(p), "xattr") {
		t.Fatalf("burp-suite is notarized and must never be stripped:\n%s", render(p))
	}
}

// Excluded tools must always be reported with a reason rather than silently
// dropped. sec-network contains a conflicting pair that exercises this.
func TestSkippedToolsAppearInOutput(t *testing.T) {
	p := testPlan(t, resolve.Request{Category: "sec-network"}, nil, nil, false)
	out := render(p)
	if !strings.Contains(out, "skipped") || !strings.Contains(out, "conflict") {
		t.Fatalf("conflict skip must be reported, got:\n%s", out)
	}
	if strings.Contains(out, "aircrack-ng") {
		t.Fatalf("aircrack-ng has no macOS block and must not appear:\n%s", out)
	}
}

// Dependency order must survive into the rendered plan, not just the resolver.
func TestPlanPreservesDependencyOrder(t *testing.T) {
	p := testPlan(t, resolve.Request{Tools: []string{"rustscan"}}, nil, nil, false)
	out := render(p)
	nmapAt := strings.Index(out, "brew install --formula nmap")
	rsAt := strings.Index(out, "brew install --formula rustscan")
	if nmapAt < 0 || rsAt < 0 {
		t.Fatalf("expected both installs, got:\n%s", out)
	}
	if nmapAt > rsAt {
		t.Fatalf("nmap must be planned before rustscan:\n%s", out)
	}
}

// The whole point of --dry-run is that it shows the real plan. Guard that the
// rendered output is generated from the same steps a run would execute.
func TestRenderedCommandsMatchExecutableSteps(t *testing.T) {
	p := testPlan(t, resolve.Request{Profile: "web"}, nil, nil, false)
	out := render(p)
	for _, tp := range p.Tools {
		for _, s := range tp.Steps {
			if !strings.Contains(out, s.String()) {
				t.Fatalf("step %q was not rendered; dry-run would lie about it", s.String())
			}
		}
	}
}
