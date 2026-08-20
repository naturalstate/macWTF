package tui

import "github.com/charmbracelet/lipgloss"

// The palette is defined once here so the whole interface stays coherent.
// Colours are adaptive: terminals with a light background get darker variants,
// which matters because a lot of macOS users run Terminal.app in light mode.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#A78BFA"}
	colDim    = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"}
	colText   = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	colOK     = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colDanger = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colInfo   = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colAccent).
			Bold(true).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().Foreground(colDim)

	categoryStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	cursorStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	itemStyle         = lipgloss.NewStyle().Foreground(colText)
	itemDimStyle      = lipgloss.NewStyle().Foreground(colDim)
	itemSelectedStyle = lipgloss.NewStyle().Foreground(colOK)

	// Flag badges. Each marks a tool that needs something beyond a plain
	// install, and each is coloured by how much it will cost the user.
	badgeQ     = lipgloss.NewStyle().Foreground(colWarn).Bold(true)   // quarantine
	badgeT     = lipgloss.NewStyle().Foreground(colInfo).Bold(true)   // TCC
	badgeLinux = lipgloss.NewStyle().Foreground(colDanger).Bold(true) // unusable on macOS
	badgePaid  = lipgloss.NewStyle().Foreground(colDim)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().Foreground(colDim)

	helpKeyStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	dangerStyle = lipgloss.NewStyle().Foreground(colDanger)
	okStyle     = lipgloss.NewStyle().Foreground(colOK)
)

// help renders a key hint bar from alternating key/description pairs.
func help(pairs ...string) string {
	out := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += helpStyle.Render("  ·  ")
		}
		out += helpKeyStyle.Render(pairs[i]) + helpStyle.Render(" "+pairs[i+1])
	}
	return out
}
