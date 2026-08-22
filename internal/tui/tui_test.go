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

// selectProfile navigates the picker to a named profile and chooses it.
// Counting keypresses would break every time a profile is added, which says
// nothing about whether the code works.
func selectProfile(t *testing.T, m tea.Model, id string) tea.Model {
	t.Helper()
	mm := m.(Model)
	idx := -1
	for i, p := range mm.profiles {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no profile %q in the picker", id)
	}
	for i := 0; i < idx; i++ {
		m = press(m, "down")
	}
	return press(m, "enter")
}

// gotoTool moves the tree cursor onto a named tool. Stepping a fixed number of
// times assumes both catalogue size and ordering, which made these tests break
// whenever the catalogue grew without saying anything about the code.
func gotoTool(t *testing.T, m tea.Model, id string) tea.Model {
	t.Helper()
	mm := m.(Model)
	for i, r := range mm.rows {
		if r.kind == rowTool && r.tool.ID == id {
			mm.cursor = i
			return mm
		}
	}
	t.Fatalf("tool %q is not in the tree", id)
	return m
}

// gotoCategory moves the cursor onto a named category header.
func gotoCategory(t *testing.T, m tea.Model, cat string) tea.Model {
	t.Helper()
	mm := m.(Model)
	for i, r := range mm.rows {
		if r.kind == rowCategory && r.category == cat {
			mm.cursor = i
			return mm
		}
	}
	t.Fatalf("category %q is not in the tree", cat)
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
	m := selectProfile(t, newModel(t), "recon")
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
	m := press(selectProfile(t, newModel(t), "baseline"), "a")
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

	// nmap must be in the tree. Whether it is on screen depends on scroll
	// position, which is not what this test is about.
	var found bool
	for _, r := range mm.rows {
		if r.kind == rowTool && r.tool.ID == "nmap" {
			found = true
		}
	}
	if !found {
		t.Error("expected nmap in the tree")
	}
}

func TestToggleTool(t *testing.T) {
	m := selectProfile(t, newModel(t), "baseline")
	m = gotoTool(t, m, "ripgrep")

	before := m.(Model).selected["ripgrep"]
	m = press(m, " ")
	if m.(Model).selected["ripgrep"] == before {
		t.Error("space should have toggled the highlighted tool")
	}
	m = press(m, " ")
	if m.(Model).selected["ripgrep"] != before {
		t.Error("space again should have toggled it back")
	}
}

func TestToggleCategory(t *testing.T) {
	m := press(newModel(t), "c") // custom: nothing selected
	m = gotoCategory(t, m, "cli")
	m = press(m, " ")
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
	m := selectProfile(t, newModel(t), "baseline")
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
	m := press(selectProfile(t, newModel(t), "recon"), "enter") // -> plan
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
	m := press(selectProfile(t, newModel(t), "baseline"), "enter") // -> plan
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
	m := press(selectProfile(t, newModel(t), "baseline"), "enter", "i")
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
	m := press(selectProfile(t, newModel(t), "baseline"), "enter", "i")
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
	m := selectProfile(t, newModel(t), "baseline")
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

// No screen may render taller than the terminal. An alternate screen does not
// scroll, so an oversized view is clipped at the top and the user is left
// looking at the tail of the interface with the header gone — which is exactly
// what a large selection did to the confirmation screen.
func TestNoScreenExceedsTerminalHeight(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{
		{Width: 120, Height: 40}, {Width: 90, Height: 20},
		{Width: 80, Height: 14}, {Width: 60, Height: 10},
	} {
		m := newModel(t)
		m, _ = m.Update(size)

		// Custom start, select everything, then walk every screen. The
		// whole catalogue is the worst case for content height.
		m = press(m, "c", "a")
		for _, step := range []struct {
			name string
			keys []string
		}{
			{"profile", nil},
			{"tree", nil},
			{"plan", []string{"enter"}},
			{"confirm", []string{"i"}},
		} {
			m = press(m, step.keys...)
			lines := strings.Count(m.View(), "\n") + 1
			if lines > size.Height {
				t.Errorf("%s screen at %dx%d rendered %d lines, budget %d",
					step.name, size.Width, size.Height, lines, size.Height)
			}
		}
	}
}

// The plan screen must always show how to proceed, however long the plan is.
// That line was previously appended below the plan pane, which made it the
// first thing lost when the content was clamped.
func TestPlanCallToActionSurvivesASmallWindow(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 100, Height: 40}, {Width: 80, Height: 12}} {
		m := newModel(t)
		m, _ = m.Update(size)
		m = press(m, "c", "a", "enter")
		if !strings.Contains(m.View(), "to install") {
			t.Errorf("at %dx%d the plan screen does not say how to proceed:\n%s",
				size.Width, size.Height, m.View())
		}
	}
}
