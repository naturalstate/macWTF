package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

// barGradient is the fill colour ramp for the progress bar: indigo through
// violet to a light lilac. Rendering each cell at its own point on the ramp
// gives the bar depth without animation for its own sake, and it reads as one
// object rather than a row of identical blocks.
var barGradient = []string{
	"#4C1D95", "#5B21B6", "#6D28D9", "#7C3AED",
	"#8B5CF6", "#9B72F8", "#A78BFA", "#B9A2FC",
}

// pulseFrames drive the leading edge of the bar. A partially filled cell shows
// how far into the current tool the run is, which a plain block cannot.
var pulseFrames = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

// gradientBar renders a progress bar coloured along its length, with a
// fractional leading edge and a dimmed track behind it.
func gradientBar(done, total, width, tick int) string {
	if width < 4 {
		width = 4
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(done) / float64(total)
	}
	if ratio > 1 {
		ratio = 1
	}

	exact := ratio * float64(width)
	full := int(exact)
	frac := exact - float64(full)

	var b strings.Builder
	for i := 0; i < full && i < width; i++ {
		c := barGradient[i*len(barGradient)/max(1, width)]
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("█"))
	}

	if full < width {
		// The leading edge shows sub-cell progress, and breathes while a
		// tool is mid-flight so a slow download does not look frozen.
		idx := int(frac * float64(len(pulseFrames)))
		if idx == 0 {
			idx = (tick / 2) % 3
		}
		if idx >= len(pulseFrames) {
			idx = len(pulseFrames) - 1
		}
		c := barGradient[min(len(barGradient)-1, full*len(barGradient)/max(1, width))]
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(pulseFrames[idx]))

		for i := full + 1; i < width; i++ {
			b.WriteString(trackStyle.Render("─"))
		}
	}
	return b.String()
}

var trackStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D8D8E4", Dark: "#2A2A38"})

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
