// Package tui implements macwtf's terminal interface.
//
// Three screens: pick a profile, adjust the selection in a category tree, then
// review the resolved plan. The plan screen reuses the same renderer as
// --dry-run, so what you see here is what a real install would execute.
//
// Installing is reached only through an explicit confirmation screen that
// states what will change, so browsing the catalogue can never install
// anything by accident.
package tui

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

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
	screenConfirm
	screenProgress
	screenDone
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
	profiles     []*manifest.Profile
	profCursor   int
	profileSizes map[string]int

	// installed marks tools already present on this machine, so the user can
	// see at a glance what a selection would actually change. Without it,
	// selecting a fully-installed profile looks identical to selecting a
	// fresh one right up until the plan turns out to be empty.
	installed map[string]bool

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

	// Install run.
	plan            *install.Plan
	allowQuarantine bool
	events          <-chan install.Event
	doneCh          chan doneMsg
	cancelRun       context.CancelFunc
	runTotal        int
	runDone         int
	runCurrent      string
	runStatus       string
	runRecent       []string
	runLog          []logEntry
	runResult       *install.Result
	runErr          error
	startedAt       time.Time
	cancelled       bool
	spin            int

	// notice is a transient message shown to the user, for the cases where
	// a keypress correctly does nothing. Silence reads as a broken button.
	notice string

	quitting bool
	err      error
}

var errNothingSelected = errors.New("nothing selected")

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
		cat:          cat,
		ctx:          ctx,
		screen:       screenProfile,
		profiles:     profiles,
		selected:     map[string]bool{},
		collapsed:    map[string]bool{},
		installed:    detectInstalled(cat, ctx),
		doneCh:       make(chan doneMsg, 1),
		profileSizes: map[string]int{},
		width:        100,
		height:       30,
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
	p.Render(&b, false)
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

// resolveSelection resolves the current selection, optionally filtered to
// backends that are implemented.
func resolveSelection(m *Model, supported map[manifest.Backend]bool) (*resolve.Result, error) {
	return resolve.Resolve(m.cat, resolve.Request{
		Tools:             m.selectedIDs(),
		Arch:              m.ctx.Arch,
		SupportedBackends: supported,
	})
}

// detectInstalled asks each backend once what it already has, and maps that
// back onto tool ids. Failures are silent: a backend that cannot be queried
// simply means nothing is known to be installed through it, which is a display
// concern rather than a reason to refuse to start.
func detectInstalled(cat *manifest.Catalogue, ctx *backend.Ctx) map[string]bool {
	out := map[string]bool{}
	reg := backend.NewRegistry()

	for _, t := range cat.Tools {
		impl, err := reg.Get(t.Backend)
		if err != nil {
			continue
		}
		set, err := ctx.InstalledFor(impl)
		if err != nil {
			continue
		}
		if set[t.Package] {
			out[t.ID] = true
		}
	}
	return out
}

// refreshInstalled re-detects after a run, so the tree reflects reality if the
// user goes back to pick more.
func (m *Model) refreshInstalled() {
	m.ctx.ResetInstalledCache()
	m.installed = detectInstalled(m.cat, m.ctx)
}

// profileSize is how many tools a profile actually resolves to. Cached because
// the picker renders every profile on every keystroke and resolving is not
// free once the catalogue is large.
func (m *Model) profileSize(p *manifest.Profile) int {
	if n, ok := m.profileSizes[p.ID]; ok {
		return n
	}
	res, err := resolve.Resolve(m.cat, resolve.Request{Profile: p.ID, Arch: m.ctx.Arch})
	n := 0
	if err == nil {
		n = len(res.Install)
	}
	if m.profileSizes == nil {
		m.profileSizes = map[string]int{}
	}
	m.profileSizes[p.ID] = n
	return n
}
