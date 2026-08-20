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

// chrome wraps a screen body in the shared header and footer bars, so every
// screen sits in the same frame and the interface reads as one thing.
func (m Model) chrome(context, right, body, footer string) string {
	w := m.width
	if w < 40 {
		w = 40
	}

	left := brandStyle.Render("macWTF") + headerCtxStyle.Render(context)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
		right = ""
	}
	header := left + repeat(" ", gap) + headerRightStyle.Render(right)

	var b strings.Builder
	b.WriteString(header + "\n")
	b.WriteString(rule.Render(repeat("─", w)) + "\n")
	b.WriteString(body)
	b.WriteString(rule.Render(repeat("─", w)) + "\n")
	b.WriteString(" " + footer)
	return b.String()
}

func (m Model) statusRight() string {
	return fmt.Sprintf("%s · %d tools · %d categories",
		m.ctx.Arch, len(m.cat.Tools), len(m.cat.Categories()))
}

// ---------------------------------------------------------------- profile

func (m Model) viewProfile() string {
	w := m.width
	if w < 40 {
		w = 40
	}
	inner := w - 4

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + boldStyle.Render("Choose a starting point") + "\n")
	b.WriteString("  " + descStyle.Render("Everything stays editable afterwards.") + "\n\n")

	for i, p := range m.profiles {
		selected := i == m.profCursor

		marker := "  "
		name := itemStyle.Render(p.Name)
		if selected {
			marker = keyStyle.Render("▸ ")
			name = boldStyle.Render(p.Name)
		}

		count := len(p.Tools)
		meta := countChip.Render(fmt.Sprintf("%d tools", count))
		if len(p.Includes) > 0 {
			meta += countChip.Render(" + " + strings.Join(p.Includes, ", "))
		}

		line := fmt.Sprintf("%s%s  %s", marker, padTo(name, 16), meta)
		desc := "     " + itemMuted.Render(truncate(p.Description, inner-5))

		if selected {
			b.WriteString(rowSelStyle.Render(padTo(" "+line, inner)) + "\n")
		} else {
			b.WriteString(" " + padTo(line, inner) + "\n")
		}
		b.WriteString(desc + "\n\n")
	}

	b.WriteString("  " + descStyle.Render("or press ") + keyStyle.Render("c") +
		descStyle.Render(" to start empty and pick tools by hand") + "\n\n")

	return m.chrome(
		"the tooling macOS leaves out",
		m.statusRight(),
		b.String(),
		help("↑/↓", "move", "enter", "choose", "c", "custom", "q", "quit"),
	)
}

// ------------------------------------------------------------------- tree

// toolChips renders the flag badges for a tool.
func toolChips(t *manifest.Tool) string {
	var parts []string
	if t.QuarantineStrip {
		parts = append(parts, chipQ.Render("Q"))
	}
	if len(t.TCCPermissions) > 0 {
		parts = append(parts, chipT.Render("TCC"))
	}
	if t.RequiresRosetta {
		parts = append(parts, chipQ.Render("R"))
	}
	if t.License == manifest.LicensePaid || t.License == manifest.LicenseFreemium {
		parts = append(parts, chipPaid.Render(string(t.License)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func (m Model) viewTree() string {
	w := m.width
	if w < 60 {
		w = 60
	}

	// Two columns: the tree on the left, the detail card on the right.
	// Below ~90 columns the detail card is dropped rather than squeezed
	// into illegibility.
	twoCol := w >= 90
	leftW := w - 2
	rightW := 0
	if twoCol {
		leftW = w*52/100 - 2
		rightW = w - leftW - 6
	}

	listHeight := m.height - 8
	if listHeight < 6 {
		listHeight = 6
	}

	left := m.treePane(leftW, listHeight)

	body := left
	if twoCol {
		// +1 for the title line the tree pane renders above its rows.
		right := m.detailPane(rightW, listHeight+1)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}
	body = "\n" + body + "\n"

	sel := len(m.selectedIDs())
	ctx := fmt.Sprintf("%s · %s selected", m.chosenProfile,
		okStyle.Render(fmt.Sprintf("%d", sel)))

	return m.chrome(
		ctx,
		m.statusRight(),
		body,
		help("↑/↓", "move", "space", "toggle", "←/→", "fold",
			"a/n", "all/none", "enter", "plan", "esc", "back"),
	)
}

func (m Model) treePane(width, height int) string {
	inner := width - 2

	// Keep the cursor inside the window.
	scroll := m.scroll
	if m.cursor < scroll {
		scroll = m.cursor
	}
	if m.cursor >= scroll+height {
		scroll = m.cursor - height + 1
	}
	end := scroll + height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	var b strings.Builder
	for i := scroll; i < end; i++ {
		r := m.rows[i]
		cursored := i == m.cursor

		var line string
		if r.kind == rowCategory {
			selN, tot := m.countSelected(r.category)
			arrow := "▾"
			if m.collapsed[r.category] {
				arrow = "▸"
			}
			label := categoryText.Render(fmt.Sprintf("%s %s", arrow, r.category))
			count := countChip.Render(fmt.Sprintf("%d/%d", selN, tot))
			line = padTo(label, inner-lipgloss.Width(count)-1) + " " + count
		} else {
			t := r.tool
			check := itemDimStyle.Render("○")
			nameStyle := itemStyle
			if m.selected[t.ID] {
				check = checkOnStyle.Render("◉")
				nameStyle = itemStyle
			}
			chips := toolChips(t)
			name := nameStyle.Render(t.ID)
			avail := inner - 4 - lipgloss.Width(chips) - 1
			line = "   " + check + " " + padTo(truncate(name, avail), avail)
			if chips != "" {
				line += " " + chips
			}
		}

		if cursored {
			b.WriteString(rowSelStyle.Render(padTo(line, inner)))
		} else {
			b.WriteString(padTo(line, inner))
		}
		b.WriteString("\n")
	}

	// Pad short lists so the pane keeps a stable height.
	for i := end - scroll; i < height; i++ {
		b.WriteString(padTo("", inner) + "\n")
	}

	title := paneTitleStyle.Render(" catalogue ")
	if len(m.rows) > height {
		title += countChip.Render(fmt.Sprintf("%d–%d of %d", scroll+1, end, len(m.rows)))
	}

	return paneFocusStyle.Width(width).Render(
		title + "\n" + strings.TrimRight(b.String(), "\n"))
}

// detailPane describes the highlighted row. This is where the catalogue's
// curation pays off — why a tool is crippled, what it will demand after
// install, what it fights with.
func (m Model) detailPane(width, height int) string {
	inner := width - 2
	if len(m.rows) == 0 {
		return paneStyle.Width(width).Height(height).Render("")
	}
	r := m.rows[m.cursor]

	var b strings.Builder

	if r.kind == rowCategory {
		selN, tot := m.countSelected(r.category)
		b.WriteString(boldStyle.Render(r.category) + "\n\n")
		b.WriteString(itemMuted.Render(fmt.Sprintf("%d of %d selected", selN, tot)) + "\n\n")
		b.WriteString(descStyle.Render("space toggles the whole category") + "\n")
		b.WriteString(descStyle.Render("←/→ folds and unfolds it"))
		return paneStyle.Width(width).Height(height).Render(b.String())
	}

	t := r.tool
	b.WriteString(boldStyle.Render(truncate(t.Name, inner)) + "\n")
	b.WriteString(itemMuted.Render(fmt.Sprintf("%s · %s", t.Backend, t.Package)) + "\n\n")
	b.WriteString(wrap(itemStyle.Render(t.Description), inner) + "\n")

	if chips := toolChips(t); chips != "" {
		b.WriteString("\n" + chips + "\n")
	}

	if len(t.AlsoOn) > 0 {
		var names []string
		for _, p := range t.AlsoOn {
			names = append(names, string(p))
		}
		b.WriteString("\n" + itemMuted.Render("also on: "+strings.Join(names, ", ")) + "\n")
	}
	if t.QuarantineStrip {
		b.WriteString("\n" + warnStyle.Render("Unsigned binary") + "\n")
		b.WriteString(wrap(itemMuted.Render("Gatekeeper blocks the first launch until the quarantine attribute is removed. macWTF asks before doing that."), inner) + "\n")
	}
	if len(t.TCCPermissions) > 0 {
		b.WriteString("\n" + infoStyle.Render("Needs permission granted by hand") + "\n")
		for _, p := range t.TCCPermissions {
			b.WriteString(wrap(itemMuted.Render("  "+p.Pane().Name), inner) + "\n")
		}
	}
	if len(t.Requires) > 0 {
		b.WriteString("\n" + itemMuted.Render("requires: "+strings.Join(t.Requires, ", ")) + "\n")
	}
	if len(t.ConflictsWith) > 0 {
		b.WriteString(itemMuted.Render("conflicts: "+strings.Join(t.ConflictsWith, ", ")) + "\n")
	}
	if t.Notes != "" {
		b.WriteString("\n" + wrap(descStyle.Render(t.Notes), inner) + "\n")
	}

	return paneStyle.Width(width).Height(height).Render(strings.TrimRight(b.String(), "\n"))
}

// wrap breaks text to a width on word boundaries.
func wrap(s string, w int) string {
	if w < 10 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var lines []string
	cur := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(cur)+1+lipgloss.Width(word) > w {
			lines = append(lines, cur)
			cur = word
			continue
		}
		cur += " " + word
	}
	return strings.Join(append(lines, cur), "\n")
}

// ------------------------------------------------------------------- plan

func (m Model) viewPlan() string {
	w := m.width
	if w < 60 {
		w = 60
	}
	height := m.height - 8
	if height < 6 {
		height = 6
	}

	end := m.planScroll + height
	if end > len(m.planLines) {
		end = len(m.planLines)
	}

	var b strings.Builder
	for i := m.planScroll; i < end; i++ {
		b.WriteString(padTo(truncate(m.planLines[i], w-6), w-6) + "\n")
	}
	for i := end - m.planScroll; i < height; i++ {
		b.WriteString(padTo("", w-6) + "\n")
	}

	title := paneTitleStyle.Render(" plan ")
	if len(m.planLines) > height {
		title += countChip.Render(fmt.Sprintf("lines %d–%d of %d",
			m.planScroll+1, end, len(m.planLines)))
	}

	pane := paneFocusStyle.Width(w - 2).Render(
		title + "\n" + strings.TrimRight(b.String(), "\n"))

	return m.chrome(
		"plan preview · "+warnStyle.Render("nothing will be executed"),
		m.statusRight(),
		"\n"+pane+"\n",
		help("↑/↓", "scroll", "esc", "back to selection", "q", "quit"),
	)
}
