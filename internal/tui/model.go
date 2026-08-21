// Package tui implements macwtf's terminal interface.
//
// Three screens: pick a profile, adjust the selection in a category tree, then
// review the resolved plan. The plan screen reuses the same renderer as
// --dry-run, so what you see here is what a real install would execute.
//
// Nothing in this package installs anything. The TUI resolves and previews;
// executing is a separate, deliberate step.
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
)

type screen int

const (
	screenProfile screen = iota
	screenTree
	screenPlan
)

// rowKind distinguishes a category header from a tool within it, so one flat
// slice can back a tree view with collapsible sections.
type rowKind int

const (
	rowCategory rowKind = iota
	rowTool
)

type row struct {
	kind     rowKind
	category string
	tool     *manifest.Tool
}

// Model is the bubbletea model for the whole interface.
type Model struct {
	cat *manifest.Catalogue
	ctx *backend.Ctx

	screen screen
	width  int
	height int

	// Profile screen.
	profiles   []*manifest.Profile
	profCursor int

	// Tree screen.
	rows      []row
	cursor    int
	scroll    int
	selected  map[string]bool
	collapsed map[string]bool

	// Plan screen.
	planLines  []string
	planScroll int

	// chosenProfile records which profile seeded the selection, for display.
	chosenProfile string

	quitting bool
	err      error
}

// New builds the initial model.
func New(cat *manifest.Catalogue, ctx *backend.Ctx) Model {
	profiles := append([]*manifest.Profile(nil), cat.Profiles...)
	sort.SliceStable(profiles, func(i, j int) bool {
		// Everything sorts last: it is the escape hatch, not a
		// starting point, and putting it first invites a mis-click
		// that installs the whole catalogue.
		if a, b := profiles[i].Synthetic, profiles[j].Synthetic; a != b {
			return b
		}
		return profiles[i].ID < profiles[j].ID
	})

	m := Model{
		cat:       cat,
		ctx:       ctx,
		screen:    screenProfile,
		profiles:  profiles,
		selected:  map[string]bool{},
		collapsed: map[string]bool{},
		width:     100,
		height:    30,
	}
	m.buildRows()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// buildRows rebuilds the flat row list from the catalogue and collapse state.
func (m *Model) buildRows() {
	m.rows = nil
	for _, c := range m.cat.Categories() {
		m.rows = append(m.rows, row{kind: rowCategory, category: c})
		if m.collapsed[c] {
			continue
		}
		for _, t := range m.cat.InCategory(c) {
			m.rows = append(m.rows, row{kind: rowTool, category: c, tool: t})
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// applyProfile pre-checks everything a profile resolves to. Selections stay
// individually toggleable afterwards — the profile is a starting point, not a
// commitment.
func (m *Model) applyProfile(p *manifest.Profile) {
	m.chosenProfile = p.Name
	m.selected = map[string]bool{}

	res, err := resolve.Resolve(m.cat, resolve.Request{Profile: p.ID, Arch: m.ctx.Arch})
	if err != nil {
		m.err = err
		return
	}
	for _, t := range res.Install {
		m.selected[t.ID] = true
	}
	// Tools the resolver excluded (linux-only, conflicts) stay unchecked,
	// so the tree reflects what would actually be installed.
}

// selectedIDs returns the current selection in catalogue order.
func (m *Model) selectedIDs() []string {
	var out []string
	for _, t := range m.cat.Tools {
		if m.selected[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}

// buildPlan resolves the current selection and renders it with the same
// renderer --dry-run uses.
func (m *Model) buildPlan() {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		m.planLines = []string{"", "  Nothing selected.", ""}
		return
	}

	reg := backend.NewRegistry()
	supported := map[manifest.Backend]bool{}
	for b := range reg {
		supported[b] = true
	}

	res, err := resolve.Resolve(m.cat, resolve.Request{
		Tools:             ids,
		Arch:              m.ctx.Arch,
		SupportedBackends: supported,
	})
	if err != nil {
		m.planLines = []string{"", "  error: " + err.Error(), ""}
		return
	}

	p, err := install.BuildPlan(res, reg, m.ctx)
	if err != nil {
		m.planLines = []string{"", "  error: " + err.Error(), ""}
		return
	}

	var b strings.Builder
	p.Render(&b, true)
	m.planLines = strings.Split(b.String(), "\n")
	m.planScroll = 0
}

// countSelected reports how many tools in a category are selected.
func (m *Model) countSelected(cat string) (sel, total int) {
	for _, t := range m.cat.InCategory(cat) {
		total++
		if m.selected[t.ID] {
			sel++
		}
	}
	return
}

// toggleCategory selects all of a category, or clears it if all are selected.
func (m *Model) toggleCategory(cat string) {
	sel, total := m.countSelected(cat)
	want := sel < total
	for _, t := range m.cat.InCategory(cat) {
		m.selected[t.ID] = want
	}
}
