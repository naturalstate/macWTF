package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naturalstate/macWTF/internal/backend"
	"github.com/naturalstate/macWTF/internal/manifest"
)

// Run launches the interface. It is preview-only: the model can resolve and
// render a plan, but has no path to executing one.
func Run(cat *manifest.Catalogue, ctx *backend.Ctx) error {
	p := tea.NewProgram(New(cat, ctx), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := final.(Model); ok && m.err != nil {
		return m.err
	}
	fmt.Println("Nothing was installed — the TUI is preview-only for now.")
	return nil
}
