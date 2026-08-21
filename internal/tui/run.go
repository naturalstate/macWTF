package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
)

// Run launches the interface.
func Run(cat *manifest.Catalogue, ctx *backend.Ctx) error {
	p := tea.NewProgram(New(cat, ctx), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := final.(Model); ok && m.err != nil {
		return m.err
	}
	return nil
}
