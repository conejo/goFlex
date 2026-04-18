// main.go — entry point for the FlexRadio Bubble Tea client.

package main

import (
	"flag"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	addr := flag.String("addr", defaultRadioAddr, "FlexRadio address (host or host:port)")
	flag.Parse()

	p := tea.NewProgram(
		model{
			addr:   *addr,
			status: "Press Enter to connect",
			subs:   newDefaultSubs(),
		},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
