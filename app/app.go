// app.go — exported entry point for the FlexRadio Bubble Tea client.

package app

import (
	"fmt"

	"goFlex/config"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the Bubble Tea TUI.
func Run() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		return
	}

	p := tea.NewProgram(
		model{
			status: "Scanning for radios…",
			subs:   newDefaultSubs(),
			maxLog: cfg.MaxLog,
			cfg:    cfg,
			addr:   fmt.Sprintf("%s:%d", cfg.RadioAddress, cfg.RadioPort),
		},
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
