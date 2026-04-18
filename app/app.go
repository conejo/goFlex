// app.go — exported entry point for the FlexRadio Bubble Tea client.

package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the Bubble Tea TUI.
func Run(maxLog int) {
	p := tea.NewProgram(
		model{
			status: "Scanning for radios…",
			subs:   newDefaultSubs(),
			maxLog: maxLog,
		},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
