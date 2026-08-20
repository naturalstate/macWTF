package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		// ctrl+c always exits, from any screen.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		switch m.screen {
		case screenProfile:
			return m.updateProfile(msg)
		case screenTree:
			return m.updateTree(msg)
		case screenPlan:
			return m.updatePlan(msg)
		}
	}
	return m, nil
}

func (m Model) updateProfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.profCursor > 0 {
			m.profCursor--
		}

	case "down", "j":
		if m.profCursor < len(m.profiles)-1 {
			m.profCursor++
		}

	case "enter", " ":
		if len(m.profiles) == 0 {
			return m, nil
		}
		m.applyProfile(m.profiles[m.profCursor])
		m.screen = screenTree
		m.cursor, m.scroll = 0, 0

	case "c":
		// Start from an empty selection instead of a profile.
		m.chosenProfile = "Custom"
		m.selected = map[string]bool{}
		m.screen = screenTree
		m.cursor, m.scroll = 0, 0
	}
	return m, nil
}

func (m Model) updateTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.screen = screenProfile
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}

	case "pgup":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown":
		m.cursor += 10
		if m.cursor > len(m.rows)-1 {
			m.cursor = len(m.rows) - 1
		}

	case "home", "g":
		m.cursor = 0

	case "end", "G":
		m.cursor = len(m.rows) - 1

	case " ", "x":
		if len(m.rows) == 0 {
			return m, nil
		}
		r := m.rows[m.cursor]
		if r.kind == rowCategory {
			m.toggleCategory(r.category)
		} else {
			m.selected[r.tool.ID] = !m.selected[r.tool.ID]
		}

	case "left", "h":
		if len(m.rows) == 0 {
			return m, nil
		}
		r := m.rows[m.cursor]
		m.collapsed[r.category] = true
		m.buildRows()
		// Keep the cursor on the header we just collapsed.
		for i, rr := range m.rows {
			if rr.kind == rowCategory && rr.category == r.category {
				m.cursor = i
				break
			}
		}

	case "right", "l":
		if len(m.rows) == 0 {
			return m, nil
		}
		m.collapsed[m.rows[m.cursor].category] = false
		m.buildRows()

	case "a":
		// Everything in the catalogue is installable on this platform:
		// entries without a macOS block are never loaded.
		for _, t := range m.cat.Tools {
			m.selected[t.ID] = true
		}

	case "n":
		m.selected = map[string]bool{}

	case "enter", "p":
		m.buildPlan()
		m.screen = screenPlan
	}
	return m, nil
}

func (m Model) updatePlan(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "esc", "left", "h", "backspace":
		m.screen = screenTree

	case "up", "k":
		if m.planScroll > 0 {
			m.planScroll--
		}

	case "down", "j":
		if m.planScroll < len(m.planLines)-1 {
			m.planScroll++
		}

	case "pgup":
		m.planScroll -= 10
		if m.planScroll < 0 {
			m.planScroll = 0
		}

	case "pgdown":
		m.planScroll += 10
		if m.planScroll > len(m.planLines)-1 {
			m.planScroll = len(m.planLines) - 1
		}
	}
	return m, nil
}
