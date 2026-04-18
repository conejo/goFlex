// main.go — entry point for the FlexRadio Bubble Tea client.

package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(
		model{
			status: "Press Enter to connect",
			subs:   append([]subscription(nil), defaultSubs...),
		},
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
