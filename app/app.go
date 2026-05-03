// app.go — exported entry point for the FlexRadio Bubble Tea client.

package app

import (
	"fmt"

	"goFlex/config"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the Bubble Tea TUI using the default config loader.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	return RunWithConfig(cfg)
}

// RunWithConfig starts the Bubble Tea TUI with the provided config.
func RunWithConfig(cfg *config.Config) error {
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
		return fmt.Errorf("error: %w", err)
	}
	return nil
}
