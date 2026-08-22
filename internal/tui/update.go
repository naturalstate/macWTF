package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/sudo"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case eventMsg:
		m.handleEvent(install.Event(msg))
		return m, waitForEvent(m.events)

	case tickMsg:
		if m.screen == screenProgress {
			m.spin++
			return m, tick()
		}
		return m, nil

	case sudoDoneMsg:
		// Whether or not it succeeded, the run proceeds: declining
		// authorisation is a valid choice, and the steps that need it
		// will say so rather than the whole run refusing to start.
		if msg.err != nil {
			m.notice = "continuing without administrator authorisation"
		}
		return m, m.startInstall()

	case doneMsg:
		m.runResult = msg.result
		m.runErr = msg.err
		m.screen = screenDone
		return m, nil

	case tea.KeyMsg:
		// ctrl+c cancels a running install rather than killing the
		// program outright, so the run can stop cleanly and report what
		// it managed to do. State is saved after every tool, so the
		// work already done is never lost.
		if msg.String() == "ctrl+c" {
			if m.screen == screenProgress && m.cancelRun != nil && !m.cancelled {
				m.cancelled = true
				m.cancelRun()
				return m, nil
			}
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
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenProgress:
			if msg.String() == "v" {
				m.verbose = !m.verbose
			}
			return m, nil
		case screenDone:
			return m.updateDone(msg)
		}
	}
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "q":
		m.screen = screenPlan
		return m, nil

	case "t":
		// Toggle the quarantine decision. Separate from the install
		// confirmation on purpose: agreeing to install a tool is not
		// agreeing to waive a Gatekeeper malware check on it.
		m.allowQuarantine = !m.allowQuarantine
		return m, nil

	case "y", "enter":
		// Ask for the password before starting, with the interface
		// suspended so the prompt actually reaches the terminal.
		if m.plan != nil && len(m.plan.MayNeedSudo()) > 0 && !sudo.Active() && !m.sudoAsked {
			m.sudoAsked = true
			return m, primeSudo()
		}
		return m, m.startInstall()
	}
	return m, nil
}

func (m Model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.planScroll > 0 {
			m.planScroll--
		}
	case "down", "j":
		m.planScroll++
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
		m.rebuildKeepingCursor(r)

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
		m.buildRows()

	case "n":
		m.selected = map[string]bool{}
		m.buildRows()

	case "E":
		m.setAllCollapsed(false)
		m.buildRows()

	case "C":
		m.setAllCollapsed(true)
		m.buildRows()
		m.cursor = 0

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
		m.notice = ""
		m.screen = screenTree

	case "i", "enter":
		// Move to the confirmation screen. Installing is never one
		// keypress away from browsing.
		p, err := m.resolvePlan()
		if err != nil {
			m.notice = "could not build a plan: " + err.Error()
			return m, nil
		}
		m.plan = p

		// Nothing to do is a legitimate outcome, but it must say so.
		// A keypress that silently does nothing reads as a broken
		// button, not as "you are already up to date".
		todo, already, failed := p.Counts()
		if todo == 0 {
			switch {
			case already > 0 && failed == 0:
				m.notice = fmt.Sprintf(
					"Nothing to install — all %d selected tool(s) are already installed. Go back with esc and select more.", already)
			case failed > 0:
				m.notice = fmt.Sprintf(
					"Nothing can be installed: %d tool(s) could not be planned. See the plan above.", failed)
			default:
				m.notice = "Nothing selected. Press esc to go back and choose some tools."
			}
			return m, nil
		}
		m.notice = ""
		m.screen = screenConfirm

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
