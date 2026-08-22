package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
)

func newModel(t *testing.T) tea.Model {
	t.Helper()
	cat, err := manifest.Load("")
	if err != nil {
		t.Fatalf("load catalogue: %v", err)
	}
	ctx := backend.NewTestCtx()
	ctx.SeedInstalled(manifest.BackendBrew, map[string]bool{})
	ctx.SeedInstalled(manifest.BackendCask, map[string]bool{})

	var m tea.Model = New(cat, ctx)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var msg tea.Msg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func TestProfileScreenListsProfiles(t *testing.T) {
	out := newModel(t).View()
	for _, want := range []string{"Baseline", "Recon", "Web Hacking", "Desktop"} {
		if !strings.Contains(out, want) {
			t.Errorf("profile screen missing %q:\n%s", want, out)
		}
	}
}

// Choosing a profile must pre-check what that profile resolves to, while
// leaving everything editable.
func TestChoosingProfilePreselects(t *testing.T) {
	m := press(newModel(t), "down", "down", "enter") // Recon
	mm := m.(Model)

	if mm.screen != screenTree {
		t.Fatalf("expected the tree screen, got %v", mm.screen)
	}
	for _, want := range []string{"nmap", "ffuf", "ripgrep"} {
		if !mm.selected[want] {
			t.Errorf("expected %q pre-selected by the recon profile", want)
		}
	}
	if mm.selected["burp-suite"] {
		t.Error("burp-suite is not in recon and must not be selected")
	}
}

// Tools with no macOS block must never appear in the tree at all, so the user
// is never offered something macWTF cannot install.
func TestNonMacOSToolsAreNotInTheTree(t *testing.T) {
	m := press(newModel(t), "enter", "a") // baseline, then select everything
	mm := m.(Model)

	for _, r := range mm.rows {
		if r.kind == rowTool && r.tool.ID == "aircrack-ng" {
			t.Fatal("aircrack-ng has no macOS block and must not be displayed")
		}
	}
	if mm.selected["aircrack-ng"] {
		t.Error("select-all reached a tool that is not in the catalogue")
	}
	if !mm.selected["nmap"] {
		t.Error("select-all should have selected nmap")
	}
	if !strings.Contains(mm.View(), "nmap") {
		t.Error("expected nmap to be visible in the tree")
	}
}

func TestToggleTool(t *testing.T) {
	m := press(newModel(t), "enter") // baseline -> tree, cursor on first category
	before := m.(Model).selected["ripgrep"]
	m = press(m, "down", " ") // move onto first tool, toggle
	if m.(Model).selected["ripgrep"] == before {
		t.Error("space should have toggled the highlighted tool")
	}
}

func TestToggleCategory(t *testing.T) {
	m := press(newModel(t), "c") // custom: nothing selected
	m = press(m, " ")            // cursor is on the first category header
	mm := m.(Model)
	sel, total := mm.countSelected("cli")
	if sel != total || total == 0 {
		t.Fatalf("toggling a category header should select all of it, got %d/%d", sel, total)
	}
	m = press(m, " ")
	mm2 := m.(Model)
	if sel, _ := mm2.countSelected("cli"); sel != 0 {
		t.Fatalf("toggling again should clear the category, got %d selected", sel)
	}
}

func TestCollapseAndExpand(t *testing.T) {
	m := press(newModel(t), "enter")
	rowsOpen := len(m.(Model).rows)

	m = press(m, "left")
	if len(m.(Model).rows) >= rowsOpen {
		t.Fatal("collapsing a category should reduce the row count")
	}
	m = press(m, "right")
	if len(m.(Model).rows) != rowsOpen {
		t.Fatal("expanding should restore the row count")
	}
}

// The plan screen must render the same commands --dry-run produces.
func TestPlanScreenShowsRealCommands(t *testing.T) {
	m := press(newModel(t), "down", "down", "enter", "enter") // recon -> plan
	mm := m.(Model)
	if mm.screen != screenPlan {
		t.Fatalf("expected the plan screen, got %v", mm.screen)
	}
	joined := strings.Join(mm.planLines, "\n")
	if !strings.Contains(joined, "brew install --formula nmap") {
		t.Fatalf("plan screen should show real commands:\n%s", joined)
	}
	// The TUI plan is a review step before installing, not a dry run.
	// Labelling it a dry run tells the user the opposite of the truth.
	if strings.Contains(joined, "nothing will be executed") {
		t.Fatalf("the TUI plan must not claim to be a dry run:\n%s", joined)
	}
	if !strings.Contains(mm.View(), "to install") {
		t.Fatalf("the plan screen must say how to proceed:\n%s", mm.View())
	}
}

// Installing must never be reachable without passing through the confirmation
// screen, so browsing the catalogue cannot modify the machine by accident.
func TestInstallRequiresConfirmation(t *testing.T) {
	m := press(newModel(t), "enter", "enter") // profile -> tree -> plan
	if m.(Model).screen != screenPlan {
		t.Fatalf("expected the plan screen, got %v", m.(Model).screen)
	}

	m = press(m, "i")
	if got := m.(Model).screen; got != screenConfirm {
		t.Fatalf("i on the plan screen must reach confirmation, got %v", got)
	}

	// Backing out must not start anything.
	m = press(m, "esc")
	if got := m.(Model).screen; got != screenPlan {
		t.Fatalf("esc must return to the plan, got %v", got)
	}
}

// Quarantine consent is a separate decision from agreeing to install, and
// defaults to off.
func TestQuarantineConsentDefaultsOffAndToggles(t *testing.T) {
	m := press(newModel(t), "enter", "enter", "i")
	if m.(Model).allowQuarantine {
		t.Fatal("quarantine stripping must default to off")
	}
	m = press(m, "t")
	if !m.(Model).allowQuarantine {
		t.Fatal("t should toggle quarantine consent on")
	}
	m = press(m, "t")
	if m.(Model).allowQuarantine {
		t.Fatal("t should toggle it back off")
	}
}

// The confirmation screen must state plainly that it will change the machine.
func TestConfirmScreenStatesConsequence(t *testing.T) {
	m := press(newModel(t), "enter", "enter", "i")
	out := m.(Model).View()
	if !strings.Contains(out, "About to install") {
		t.Fatalf("confirm screen must say what it will do:\n%s", out)
	}
	if !strings.Contains(out, "modify your system") {
		t.Fatalf("confirm screen must state the consequence:\n%s", out)
	}
}

func TestEmptySelectionIsHandled(t *testing.T) {
	m := press(newModel(t), "c", "enter") // custom, nothing selected, then plan
	joined := strings.Join(m.(Model).planLines, "\n")
	if !strings.Contains(joined, "Nothing selected") {
		t.Fatalf("expected an empty-selection message, got:\n%s", joined)
	}
}

func TestEscapeNavigatesBack(t *testing.T) {
	m := press(newModel(t), "enter")
	if m.(Model).screen != screenTree {
		t.Fatal("expected tree screen")
	}
	m = press(m, "esc")
	if m.(Model).screen != screenProfile {
		t.Fatal("esc should return to the profile picker")
	}
}

// Views must render at awkward sizes without panicking or producing nothing.
func TestViewsRenderAtSmallSizes(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{
		{Width: 40, Height: 10}, {Width: 200, Height: 60}, {Width: 80, Height: 24},
	} {
		m := newModel(t)
		m, _ = m.Update(size)
		for _, keys := range [][]string{{}, {"enter"}, {"enter", "enter"}} {
			mm := press(m, keys...)
			if out := mm.View(); out == "" {
				t.Fatalf("empty view at %dx%d after %v", size.Width, size.Height, keys)
			}
		}
	}
}
