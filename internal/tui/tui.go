package tui

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
)

func Run() error {
    m := NewModel()
    p := tea.NewProgram(
        m,
        tea.WithAltScreen(),
    )
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("TUI error: %w", err)
    }
    return nil
}


