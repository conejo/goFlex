// tui_integration_test.go — integration tests using tea.Program.

package app

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestProgram_QuitViaKey exercises the full Init→Update→View pipeline
// by creating a tea.Program, sending a 'q' key, and verifying clean exit.
func TestProgram_QuitViaKey(t *testing.T) {
	m := newTestModel()
	p := tea.NewProgram(
		m,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	done := make(chan tea.Model, 1)
	go func() {
		fm, err := p.Run()
		if err != nil {
			t.Errorf("Run error: %v", err)
		}
		done <- fm
	}()

	// Allow the program to start its event loop.
	time.Sleep(50 * time.Millisecond)

	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case fm := <-done:
		final, ok := fm.(model)
		if !ok {
			t.Fatalf("expected model, got %T", fm)
		}
		// After quit, the model should still hold its config.
		if final.cfg == nil {
			t.Error("expected cfg to be set")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for program to exit")
	}
}

// TestProgram_WindowSize exercises the pipeline with a window size message.
func TestProgram_WindowSize(t *testing.T) {
	m := newTestModel(func(m *model) {
		m.connState.connected = true
	})
	p := tea.NewProgram(
		m,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	done := make(chan tea.Model, 1)
	go func() {
		fm, err := p.Run()
		if err != nil {
			t.Errorf("Run error: %v", err)
		}
		done <- fm
	}()

	time.Sleep(50 * time.Millisecond)

	p.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case fm := <-done:
		final, ok := fm.(model)
		if !ok {
			t.Fatalf("expected model, got %T", fm)
		}
		if final.width != 80 {
			t.Errorf("width = %d, want 80", final.width)
		}
		if final.height != 24 {
			t.Errorf("height = %d, want 24", final.height)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for program to exit")
	}
}

// TestProgram_FreqInputMode exercises the frequency input mode flow.
func TestProgram_FreqInputMode(t *testing.T) {
	mock := &mockRadioConn{handle: 0x1, version: "3.0.0"}
	m := newTestModel(func(m *model) {
		m.connState.connected = true
		m.connState.conn = mock
	})
	p := tea.NewProgram(
		m,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	done := make(chan tea.Model, 1)
	go func() {
		fm, err := p.Run()
		if err != nil {
			t.Errorf("Run error: %v", err)
		}
		done <- fm
	}()

	time.Sleep(50 * time.Millisecond)

	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	p.Send(tea.KeyMsg{Type: tea.KeyEnter})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case fm := <-done:
		final, ok := fm.(model)
		if !ok {
			t.Fatalf("expected model, got %T", fm)
		}
		if final.ui.settingFreq {
			t.Error("expected settingFreq to be false after enter")
		}
		if !mock.sendCalled {
			t.Error("expected mock Send to be called for frequency")
		}
		if !strings.Contains(mock.sendCmd, "slice tune") {
			t.Errorf("expected sendCmd to contain 'slice tune', got %q", mock.sendCmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for program to exit")
	}
}
