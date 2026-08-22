package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/naturalstate/macWTF/internal/manifest"
	"github.com/naturalstate/macWTF/internal/resolve"
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
	case screenConfirm:
		return m.viewConfirm()
	case screenProgress:
		return m.viewProgress()
	case screenDone:
		return m.viewDone()
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
	if w < 60 {
		w = 60
	}

	// Same two-pane language as the catalogue screen: choose on the left,
	// see the consequence on the right. Answering "what is actually in
	// Recon?" before committing is the point.
	twoCol := w >= 90
	leftW := w - 2
	rightW := 0
	if twoCol {
		leftW = w*40/100 - 2
		rightW = w - leftW - 6
	}

	height := m.height - 8
	if height < 8 {
		height = 8
	}

	left := m.profileListPane(leftW, height)
	body := left
	if twoCol {
		body = lipgloss.JoinHorizontal(lipgloss.Top, left,
			"  ", m.profilePreviewPane(rightW, height+1))
	}

	return m.chrome(
		"the tooling macOS leaves out",
		m.statusRight(),
		"\n"+body+"\n",
		help("↑/↓", "move", "enter", "choose", "c", "start empty", "q", "quit"),
	)
}

func (m Model) profileListPane(width, height int) string {
	inner := width - 2
	var b strings.Builder

	for i, p := range m.profiles {
		cursored := i == m.profCursor

		count := len(p.Tools)
		meta := countChip.Render(fmt.Sprintf("%d", count))
		if len(p.Includes) > 0 {
			meta = countChip.Render(fmt.Sprintf("%d+", count))
		}

		name := itemStyle.Render(p.Name)
		if cursored {
			name = boldStyle.Render(p.Name)
		}
		line := " " + padTo(name, inner-lipgloss.Width(meta)-2) + " " + meta

		if cursored {
			b.WriteString(rowSelStyle.Render(padTo(line, inner)))
		} else {
			b.WriteString(padTo(line, inner))
		}
		b.WriteString("\n")
	}

	b.WriteString(padTo("", inner) + "\n")
	b.WriteString(padTo(" "+descStyle.Render("c")+itemMuted.Render("  start empty"), inner) + "\n")

	for i := len(m.profiles) + 2; i < height; i++ {
		b.WriteString(padTo("", inner) + "\n")
	}

	return paneFocusStyle.Width(width).Render(
		paneTitleStyle.Render(" profiles ") + "\n" + strings.TrimRight(b.String(), "\n"))
}

// profilePreviewPane shows what the highlighted profile resolves to, grouped by
// category. This is the resolver's real output, not the raw tool list, so
// dependencies pulled in and conflicts dropped are already accounted for.
func (m Model) profilePreviewPane(width, height int) string {
	inner := width - 2
	if len(m.profiles) == 0 {
		return paneStyle.Width(width).Height(height).Render("")
	}
	p := m.profiles[m.profCursor]

	var b strings.Builder
	title := boldStyle.Render(p.Name)
	if p.Synthetic {
		title += "  " + countChip.Render("generated")
	}
	b.WriteString(title + "\n")
	b.WriteString(wrap(itemMuted.Render(p.Description), inner) + "\n\n")
	if p.Warning != "" {
		b.WriteString(wrap(warnStyle.Render(p.Warning), inner) + "\n\n")
	}

	res, err := resolve.Resolve(m.cat, resolve.Request{Profile: p.ID, Arch: m.ctx.Arch})
	if err != nil {
		b.WriteString(dangerStyle.Render(err.Error()))
		return paneStyle.Width(width).Height(height).Render(b.String())
	}

	// Group by category, preserving catalogue order.
	byCat := map[string][]string{}
	var order []string
	var needsTCC, needsQ int
	for _, t := range res.Install {
		if _, seen := byCat[t.Category]; !seen {
			order = append(order, t.Category)
		}
		byCat[t.Category] = append(byCat[t.Category], t.ID)
		if len(t.TCCPermissions) > 0 {
			needsTCC++
		}
		if t.QuarantineStrip {
			needsQ++
		}
	}

	b.WriteString(okStyle.Render(fmt.Sprintf("%d tools", len(res.Install))))
	if needsTCC > 0 {
		b.WriteString(itemMuted.Render(fmt.Sprintf("  ·  %d need permissions", needsTCC)))
	}
	if needsQ > 0 {
		b.WriteString(itemMuted.Render(fmt.Sprintf("  ·  %d unsigned", needsQ)))
	}
	b.WriteString("\n\n")

	labelW := 14
	for _, cat := range order {
		names := strings.Join(byCat[cat], " ")
		label := categoryText.Render(padTo(cat, labelW))
		b.WriteString(label + wrapIndent(itemStyle.Render(names), inner-labelW, labelW) + "\n")
	}

	return paneStyle.Width(width).Height(height).Render(strings.TrimRight(b.String(), "\n"))
}

// wrapIndent wraps text to a width and indents every line after the first, so
// a wrapped list stays aligned under its label.
func wrapIndent(s string, w, indent int) string {
	lines := strings.Split(wrap(s, w), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = repeat(" ", indent) + lines[i]
	}
	return strings.Join(lines, "\n")
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

	sel := m.selectedIDs()
	var fresh int
	for _, id := range sel {
		if !m.installed[id] {
			fresh++
		}
	}
	ctx := fmt.Sprintf("%s · %s selected", m.chosenProfile,
		okStyle.Render(fmt.Sprintf("%d", len(sel))))
	if len(sel) > 0 {
		if fresh == 0 {
			ctx += itemMuted.Render(" · all already installed")
		} else {
			ctx += itemMuted.Render(fmt.Sprintf(" · %d to install", fresh))
		}
	}

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
			// Already-installed tools are dimmed and marked, so a
			// selection that would change nothing is obvious before
			// reaching the plan.
			if m.installed[t.ID] {
				nameStyle = itemDimStyle
				chips = itemDimStyle.Render("installed") + chipSep(chips)
			}
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
	b.WriteString(itemMuted.Render(fmt.Sprintf("%s · %s", t.Backend, t.Package)) + "\n")
	if m.installed[t.ID] {
		b.WriteString(okStyle.Render("already installed") + "\n")
	}
	b.WriteString("\n")
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

	title := paneTitleStyle.Render(" review ")
	if len(m.planLines) > height {
		title += countChip.Render(fmt.Sprintf("lines %d–%d of %d",
			m.planScroll+1, end, len(m.planLines)))
	}

	pane := paneFocusStyle.Width(w - 2).Render(
		title + "\n" + strings.TrimRight(b.String(), "\n"))

	if m.notice != "" {
		pane += "\n" + warnStyle.Render("  "+wrap(m.notice, w-6))
	} else if m.plan != nil || len(m.planLines) > 0 {
		pane += "\n  " + boldStyle.Render("Press ") + keyStyle.Render("i") +
			boldStyle.Render(" to install") +
			itemMuted.Render("   nothing has been changed yet")
	}

	return m.chrome(
		"review",
		m.statusRight(),
		"\n"+pane+"\n",
		help("i", "install", "↑/↓", "scroll", "esc", "back", "q", "quit"),
	)
}

// ---------------------------------------------------------------- confirm

func (m Model) viewConfirm() string {
	w := m.width
	if w < 60 {
		w = 60
	}
	inner := w - 6

	todo, already, failed := m.plan.Counts()
	pending := m.plan.PendingQuarantine()

	var tcc []*manifest.Tool
	for _, tp := range m.plan.Tools {
		if tp.AlreadyInstalled || tp.PlanErr != nil {
			continue
		}
		if len(tp.Tool.TCCPermissions) > 0 {
			tcc = append(tcc, tp.Tool)
		}
	}

	var b strings.Builder
	b.WriteString(boldStyle.Render(fmt.Sprintf("About to install %d tool(s)", todo)) + "\n\n")
	if already > 0 {
		b.WriteString(itemMuted.Render(fmt.Sprintf("%d already present and will be skipped.", already)) + "\n")
	}
	if failed > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf("%d cannot be planned and will be reported.", failed)) + "\n")
	}
	b.WriteString("\n")

	if len(tcc) > 0 {
		b.WriteString(infoStyle.Render("Will need permissions granted by hand afterwards:") + "\n")
		for _, t := range tcc {
			var panes []string
			for _, p := range t.TCCPermissions {
				panes = append(panes, p.Pane().Name)
			}
			b.WriteString(itemMuted.Render("  "+t.ID+" — "+strings.Join(panes, ", ")) + "\n")
		}
		b.WriteString(itemMuted.Render("  macOS does not allow an installer to grant these.") + "\n\n")
	}

	if len(pending) > 0 {
		box := warnStyle.Render(fmt.Sprintf("%d unsigned tool(s): Gatekeeper will block first launch", len(pending))) + "\n"
		for _, t := range pending {
			box += itemMuted.Render("  "+t.ID+" — "+t.AppPath) + "\n"
		}
		box += "\n"
		if m.allowQuarantine {
			box += okStyle.Render("  [x] remove quarantine") +
				itemMuted.Render("   waives the malware check for these apps") + "\n"
		} else {
			box += itemMuted.Render("  [ ] remove quarantine") +
				itemMuted.Render("   they will install, but not launch until you allow them") + "\n"
		}
		box += descStyle.Render("  press ") + keyStyle.Render("t") + descStyle.Render(" to change")
		b.WriteString(paneStyle.Width(inner).Render(box) + "\n\n")
	}

	b.WriteString(boldStyle.Render("Proceed?") + itemMuted.Render("   this will modify your system") + "\n")

	return m.chrome(
		"confirm",
		m.statusRight(),
		"\n  "+strings.ReplaceAll(b.String(), "\n", "\n  ")+"\n",
		help("y", "install", "t", "quarantine", "esc", "back", "ctrl+c", "quit"),
	)
}

// --------------------------------------------------------------- progress

func (m Model) viewProgress() string {
	w := m.width
	if w < 60 {
		w = 60
	}
	inner := w - 6

	var b strings.Builder

	const barWidth = 34
	filled := 0
	if m.runTotal > 0 {
		filled = m.runDone * barWidth / m.runTotal
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := okStyle.Render(strings.Repeat("█", filled)) +
		itemDimStyle.Render(strings.Repeat("░", barWidth-filled))

	spinner := " "
	if m.runDone < m.runTotal && !m.cancelled {
		spinner = keyStyle.Render(spinFrames[m.spin%len(spinFrames)])
	}

	pct := 0
	if m.runTotal > 0 {
		pct = m.runDone * 100 / m.runTotal
	}
	fmt.Fprintf(&b, "%s  %s  %d/%d  %d%%\n\n", spinner, bar, m.runDone, m.runTotal, pct)

	if m.cancelled {
		b.WriteString(warnStyle.Render("Cancelling — finishing the current step…") + "\n\n")
	} else if m.runCurrent != "" && m.runDone < m.runTotal {
		fmt.Fprintf(&b, "%s %s\n", boldStyle.Render(m.runCurrent),
			itemMuted.Render(truncate(m.runStatus, inner-lipgloss.Width(m.runCurrent)-2)))
		b.WriteString("\n")
	}

	// Completed tools, newest last, trimmed to fit.
	maxLog := m.height - 14
	if maxLog < 3 {
		maxLog = 3
	}
	log := m.runLog
	if len(log) > maxLog {
		log = log[len(log)-maxLog:]
	}
	for _, e := range log {
		if e.err != nil {
			fmt.Fprintf(&b, "  %s %-16s %s\n", dangerStyle.Render("✗"), e.tool,
				itemMuted.Render(truncate(e.err.Error(), inner-22)))
			continue
		}
		fmt.Fprintf(&b, "  %s %-16s %s\n", okStyle.Render("✓"), e.tool,
			itemMuted.Render(fmt.Sprintf("%.1fs", e.elapsed.Seconds())))
	}

	return m.chrome(
		"installing",
		m.statusRight(),
		"\n  "+strings.ReplaceAll(b.String(), "\n", "\n  ")+"\n",
		help("ctrl+c", "cancel"),
	)
}

// ------------------------------------------------------------------- done

func (m Model) viewDone() string {
	w := m.width
	if w < 60 {
		w = 60
	}

	var b strings.Builder
	if m.runErr != nil {
		b.WriteString(dangerStyle.Render("Run ended: "+m.runErr.Error()) + "\n\n")
	}
	if m.runResult != nil {
		var sb strings.Builder
		m.runResult.RenderSummary(&sb)
		b.WriteString(sb.String())
	}

	lines := strings.Split(b.String(), "\n")
	height := m.height - 8
	if height < 6 {
		height = 6
	}
	if m.planScroll > len(lines)-1 {
		m.planScroll = len(lines) - 1
	}
	if m.planScroll < 0 {
		m.planScroll = 0
	}
	end := m.planScroll + height
	if end > len(lines) {
		end = len(lines)
	}

	var out strings.Builder
	for i := m.planScroll; i < end; i++ {
		out.WriteString(truncate(lines[i], w-4) + "\n")
	}
	if len(lines) > height {
		out.WriteString(itemDimStyle.Render(fmt.Sprintf("  … %d–%d of %d lines",
			m.planScroll+1, end, len(lines))) + "\n")
	}

	ctx := "finished"
	if m.runResult != nil && len(m.runResult.Failed) > 0 {
		ctx = warnStyle.Render(fmt.Sprintf("finished with %d failure(s)", len(m.runResult.Failed)))
	}

	return m.chrome(ctx, m.statusRight(), "\n"+out.String(),
		help("↑/↓", "scroll", "q", "quit"))
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// chipSep joins the installed marker to any other chips.
func chipSep(chips string) string {
	if chips == "" {
		return ""
	}
	return " " + chips
}
