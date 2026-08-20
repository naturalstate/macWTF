package tui

import "github.com/charmbracelet/lipgloss"

// Palette. Adaptive so the interface stays legible on light-background
// terminals, which a meaningful share of macOS users run.
var (
	colAccent  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	colAccent2 = lipgloss.AdaptiveColor{Light: "#4C1D95", Dark: "#7C3AED"}
	colDim     = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	colMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colText    = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	colBright  = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	colOK      = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	colWarn    = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colDanger  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colInfo    = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	colSelBg   = lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#312E81"}
)

var (
	// Header: a solid brand block followed by context, on one bar.
	brandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colAccent2).
			Bold(true).
			Padding(0, 1)

	headerCtxStyle = lipgloss.NewStyle().
			Foreground(colMuted).
			Padding(0, 1)

	headerRightStyle = lipgloss.NewStyle().Foreground(colDim)

	rule = lipgloss.NewStyle().Foreground(colDim)

	// Panes.
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim).
			Padding(0, 1)

	paneFocusStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)

	paneTitleStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	// Rows.
	rowSelStyle  = lipgloss.NewStyle().Background(colSelBg).Foreground(colBright).Bold(true)
	categoryText = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	itemStyle    = lipgloss.NewStyle().Foreground(colText)
	itemDimStyle = lipgloss.NewStyle().Foreground(colDim)
	itemMuted    = lipgloss.NewStyle().Foreground(colMuted)
	checkOnStyle = lipgloss.NewStyle().Foreground(colOK).Bold(true)

	// Badges: small chips marking a tool that costs more than a plain
	// install, coloured by how much it will cost.
	chipQ = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).
		Background(colWarn).Bold(true).Padding(0, 1)
	chipT = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).
		Background(colInfo).Bold(true).Padding(0, 1)
	chipLinux = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).
			Background(colDanger).Bold(true).Padding(0, 1)
	chipPaid = lipgloss.NewStyle().Foreground(colMuted).
			Border(lipgloss.NormalBorder(), false).Padding(0, 1)

	countChip = lipgloss.NewStyle().Foreground(colMuted)

	// Semantic text.
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	dangerStyle = lipgloss.NewStyle().Foreground(colDanger)
	okStyle     = lipgloss.NewStyle().Foreground(colOK)
	infoStyle   = lipgloss.NewStyle().Foreground(colInfo)
	boldStyle   = lipgloss.NewStyle().Foreground(colBright).Bold(true)

	// Footer.
	keyStyle  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	descStyle = lipgloss.NewStyle().Foreground(colDim)
	sepStyle  = lipgloss.NewStyle().Foreground(colDim)
)

// help renders a key hint bar from alternating key/description pairs.
func help(pairs ...string) string {
	out := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += sepStyle.Render(" · ")
		}
		out += keyStyle.Render(pairs[i]) + descStyle.Render(" "+pairs[i+1])
	}
	return out
}

// padTo pads a possibly-styled string to an exact display width, measuring
// with ANSI escapes excluded.
func padTo(s string, w int) string {
	d := w - lipgloss.Width(s)
	if d <= 0 {
		return s
	}
	pad := ""
	for i := 0; i < d; i++ {
		pad += " "
	}
	return s + pad
}

// truncate shortens a string to a display width, adding an ellipsis.
func truncate(s string, w int) string {
	if w <= 1 || lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
