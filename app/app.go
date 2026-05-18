// app.go — exported entry point for the FlexRadio Bubble Tea client.

package app

import (
	"fmt"

	"goFlex/config"
	"goFlex/radio"

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
		newModel(cfg),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error: %w", err)
	}
	return nil
}

// modelOpt is a functional option for configuring a model.
type modelOpt func(*model)

// newModel creates a model with production defaults.
func newModel(cfg *config.Config, opts ...modelOpt) model {
	m := model{
		status: "Scanning for radios…",
		ui:     uiState{subs: newDefaultSubs()},
		log:    logState{max: cfg.MaxLog},
		cfg:    cfg,
		connState: connectionState{
			addr: fmt.Sprintf("%s:%d", cfg.RadioAddress, cfg.RadioPort),
		},
		dialFunc: func(addr string) (radio.RadioConn, error) {
			return radio.Dial(addr)
		},
		discoveryFunc: radio.Listen,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}
