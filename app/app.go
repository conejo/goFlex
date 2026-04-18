// app.go — exported entry point for the FlexRadio Bubble Tea client.

package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the Bubble Tea TUI with the given address and log-line limit.
func Run(addr string, maxLog int) {
	p := tea.NewProgram(
		model{
			addr:   addr,
			status: "Press Enter to connect",
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
