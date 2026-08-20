package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/naturalstate/macWTF/internal/manifest"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	switch m.screen {
	case screenProfile:
		return m.viewProfile()
	case screenTree:
		return m.viewTree()
	case screenPlan:
		return m.viewPlan()
	}
	return ""
}

func (m Model) header(sub string) string {
	title := titleStyle.Render("macWTF")
	return title + "  " + subtitleStyle.Render(sub) + "\n\n"
}

func (m Model) viewProfile() string {
	var b strings.Builder
	b.WriteString(m.header("the tooling macOS leaves out"))
	b.WriteString("  Pick a starting point. Everything stays editable afterwards.\n\n")

	for i, p := range m.profiles {
		cursor := "  "
		name := itemStyle.Render(p.Name)
		if i == m.profCursor {
			cursor = cursorStyle.Render("▸ ")
			name = cursorStyle.Render(p.Name)
		}
		fmt.Fprintf(&b, "%s%-16s %s\n", cursor, name, itemDimStyle.Render(p.Description))
	}

	b.WriteString("\n  ")
	b.WriteString(itemDimStyle.Render("or start from nothing and pick tools by hand"))
	b.WriteString("\n\n")
	b.WriteString("  " + help(
		"↑/↓", "move",
		"enter", "choose",
		"c", "custom",
		"q", "quit",
	))
	b.WriteString("\n")
	return b.String()
}

// toolBadges renders the flag markers for a tool.
func toolBadges(t *manifest.Tool) string {
	var parts []string
	if t.LinuxOnly {
		parts = append(parts, badgeLinux.Render("linux-only"))
	}
	if t.QuarantineStrip {
		parts = append(parts, badgeQ.Render("Q"))
	}
	if len(t.TCCPermissions) > 0 {
		parts = append(parts, badgeT.Render("T"))
	}
	if t.RequiresRosetta {
		parts = append(parts, badgeQ.Render("R"))
	}
	if t.License == manifest.LicensePaid || t.License == manifest.LicenseFreemium {
		parts = append(parts, badgePaid.Render(string(t.License)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func (m Model) viewTree() string {
	var b strings.Builder

	total := len(m.selectedIDs())
	sub := fmt.Sprintf("%s — %d selected", m.chosenProfile, total)
	b.WriteString(m.header(sub))

	// Reserve space for header, help bar and detail pane.
	listHeight := m.height - 14
	if listHeight < 6 {
		listHeight = 6
	}

	// Keep the cursor inside the window.
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+listHeight {
		m.scroll = m.cursor - listHeight + 1
	}

	end := m.scroll + listHeight
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scroll; i < end; i++ {
		r := m.rows[i]
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}

		if r.kind == rowCategory {
			sel, tot := m.countSelected(r.category)
			arrow := "▾"
			if m.collapsed[r.category] {
				arrow = "▸"
			}
			label := fmt.Sprintf("%s %s", arrow, r.category)
			count := itemDimStyle.Render(fmt.Sprintf("(%d/%d)", sel, tot))
			fmt.Fprintf(&b, "%s%s %s\n", cursor, categoryStyle.Render(label), count)
			continue
		}

		t := r.tool
		check := "○"
		style := itemStyle
		if m.selected[t.ID] {
			check = "◉"
			style = itemSelectedStyle
		}
		if t.LinuxOnly {
			style = itemDimStyle
		}
		fmt.Fprintf(&b, "%s   %s %s%s\n",
			cursor, style.Render(check), style.Render(t.ID), toolBadges(t))
	}

	if len(m.rows) > listHeight {
		fmt.Fprintf(&b, "  %s\n", itemDimStyle.Render(
			fmt.Sprintf("… %d–%d of %d", m.scroll+1, end, len(m.rows))))
	}

	b.WriteString("\n")
	b.WriteString(m.detailPane())
	b.WriteString("\n  " + help(
		"↑/↓", "move",
		"space", "toggle",
		"←/→", "fold",
		"a/n", "all/none",
		"enter", "plan",
		"esc", "back",
	))
	b.WriteString("\n")
	return b.String()
}

// detailPane describes the highlighted tool. This is where the catalogue's
// curation pays off — the notes explaining why a tool is crippled, or what it
// will demand after install, are the reason to read before selecting.
func (m Model) detailPane() string {
	width := m.width - 4
	if width < 40 {
		width = 40
	}
	if width > 110 {
		width = 110
	}

	if len(m.rows) == 0 {
		return ""
	}
	r := m.rows[m.cursor]

	var b strings.Builder
	if r.kind == rowCategory {
		sel, tot := m.countSelected(r.category)
		fmt.Fprintf(&b, "%s\n%s",
			categoryStyle.Render(r.category),
			itemDimStyle.Render(fmt.Sprintf("%d of %d selected · space toggles the whole category", sel, tot)))
		return paneStyle.Width(width).Render(b.String())
	}

	t := r.tool
	fmt.Fprintf(&b, "%s  %s\n", lipgloss.NewStyle().Bold(true).Render(t.Name),
		itemDimStyle.Render(fmt.Sprintf("%s · %s", t.Backend, t.Package)))
	b.WriteString(t.Description + "\n")

	if t.LinuxOnly {
		b.WriteString("\n" + dangerStyle.Render("Does not work on macOS. Routed to the lab bridge.") + "\n")
	}
	if t.QuarantineStrip {
		b.WriteString("\n" + warnStyle.Render("Unsigned — Gatekeeper will block first launch unless quarantine is removed.") + "\n")
	}
	if len(t.TCCPermissions) > 0 {
		b.WriteString("\n" + okStyle.Render("Needs permission granted by hand:") + "\n")
		for _, p := range t.TCCPermissions {
			fmt.Fprintf(&b, "  %s\n", p.Pane().Name)
		}
	}
	if len(t.Requires) > 0 {
		fmt.Fprintf(&b, "\nrequires: %s\n", strings.Join(t.Requires, ", "))
	}
	if len(t.ConflictsWith) > 0 {
		fmt.Fprintf(&b, "conflicts with: %s\n", strings.Join(t.ConflictsWith, ", "))
	}
	if t.Notes != "" {
		b.WriteString("\n" + itemDimStyle.Render(t.Notes) + "\n")
	}

	return paneStyle.Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) viewPlan() string {
	var b strings.Builder
	b.WriteString(m.header("plan preview — nothing will be executed"))

	height := m.height - 8
	if height < 6 {
		height = 6
	}
	end := m.planScroll + height
	if end > len(m.planLines) {
		end = len(m.planLines)
	}
	for i := m.planScroll; i < end; i++ {
		b.WriteString(m.planLines[i] + "\n")
	}

	if len(m.planLines) > height {
		fmt.Fprintf(&b, "%s\n", itemDimStyle.Render(
			fmt.Sprintf("  … %d–%d of %d lines", m.planScroll+1, end, len(m.planLines))))
	}

	b.WriteString("\n  " + help(
		"↑/↓", "scroll",
		"esc", "back to selection",
		"q", "quit",
	))
	b.WriteString("\n")
	return b.String()
}
