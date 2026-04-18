// tui.go — Bubble Tea TUI for the FlexRadio client.

package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

const radioAddr = "192.168.50.151"
const maxLogLines = 200

// ─── Tea messages ─────────────────────────────────────────────────────────────

type connectedMsg struct {
	radio  *RadioConn
	errMsg string
}

type statusLineMsg struct{ text string }
type logLineMsg struct{ text string }

// ─── Subscription list ────────────────────────────────────────────────────────

type subscription struct {
	name    string // protocol name, e.g. "slice"
	label   string // display label
	checked bool
}

var defaultSubs = []subscription{
	// Core — checked by default
	{name: "slice", label: "Slice", checked: true},
	{name: "pan", label: "Panadapter", checked: true},
	{name: "tx", label: "TX", checked: true},
	{name: "meter", label: "Meter", checked: true},
	{name: "audio", label: "Audio", checked: true},
	{name: "radio", label: "Radio", checked: true},
	{name: "client", label: "Client", checked: true},
	// Optional hardware
	{name: "atu", label: "ATU", checked: false},
	{name: "amplifier", label: "Amplifier", checked: false},
	{name: "gps", label: "GPS", checked: false},
	{name: "xvtr", label: "Transverter", checked: false},
	{name: "apd", label: "APD", checked: false},
	// Digital / data
	{name: "tnf", label: "TNF", checked: false},
	{name: "memories", label: "Memories", checked: false},
	{name: "cwx", label: "CWX", checked: false},
	{name: "dax", label: "DAX", checked: false},
	{name: "daxiq", label: "DAX IQ", checked: false},
	{name: "codec", label: "Codec", checked: false},
	{name: "dvk", label: "DVK", checked: false},
	{name: "usb_cable", label: "USB Cable", checked: false},
	{name: "spot", label: "Spot", checked: false},
	{name: "license", label: "License", checked: false},
}

// ─── Model ────────────────────────────────────────────────────────────────────

type model struct {
	connected    bool
	connecting   bool
	radio        *RadioConn
	errMsg       string
	status       string
	logs         []string
	height       int
	width        int
	readCh       chan tea.Msg
	subs         []subscription
	cursor       int
	scrollOffset int  // display lines scrolled up from bottom; 0 = pinned to bottom
	showSubs     bool // subscription panel visible while connected
}

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	styleButton      = lipgloss.NewStyle().Bold(true).Padding(0, 2).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("230"))
	styleButtonDis   = lipgloss.NewStyle().Bold(true).Padding(0, 2).Background(lipgloss.Color("240")).Foreground(lipgloss.Color("245"))
	styleStatus      = lipgloss.NewStyle().Padding(0, 1)
	styleLog         = lipgloss.NewStyle().Padding(0, 1)
	styleErr         = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleConnected   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleCursor      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleChecked     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleUnchecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleSubLabel    = lipgloss.NewStyle()
	styleSubLabelDim = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTx          = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // blue — outgoing commands
	styleRx          = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber — responses
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// logHeight returns the number of display lines available for the log pane.
func (m model) logHeight() int {
	subsLines := 0
	if !m.connected || m.showSubs {
		// "\nSubscriptions:\n" + N sub rows
		subsLines = 2 + len(m.subs)
	}
	h := m.height - 3 - subsLines
	if h < 1 {
		h = 1
	}
	return h
}

// ─── Init / Update / View ─────────────────────────────────────────────────────

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.radio != nil {
				m.radio.Close()
			}
			return m, tea.Quit
		case "s", "tab":
			if m.connected {
				m.showSubs = !m.showSubs
			}
		case "up", "k":
			if m.connected && !m.showSubs {
				m.scrollOffset++
			} else if !m.connecting && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.connected && !m.showSubs {
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			} else if !m.connecting && m.cursor < len(m.subs)-1 {
				m.cursor++
			}
		case "pgup":
			m.scrollOffset += m.logHeight()
		case "pgdown":
			m.scrollOffset -= m.logHeight()
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		case "G", "end":
			m.scrollOffset = 0
		case " ":
			if !m.connecting && (m.showSubs || !m.connected) {
				m.subs[m.cursor].checked = !m.subs[m.cursor].checked
				if m.connected && m.radio != nil {
					return m, m.toggleSubCmd(m.subs[m.cursor])
				}
			}
		case "enter":
			if !m.connected && !m.connecting {
				m.connecting = true
				m.errMsg = ""
				m.status = "Connecting…"
				return m, m.connectCmd()
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.connected && !m.showSubs {
				m.scrollOffset++
			} else if !m.connecting && m.cursor > 0 {
				m.cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.connected && !m.showSubs {
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			} else if !m.connecting && m.cursor < len(m.subs)-1 {
				m.cursor++
			}
		}

	case connectedMsg:
		m.connecting = false
		if msg.errMsg != "" {
			m.errMsg = msg.errMsg
			m.status = "Disconnected"
		} else {
			m.connected = true
			m.radio = msg.radio
			m.status = fmt.Sprintf("Connected  handle=0x%X  version=%s", m.radio.Handle, m.radio.Version)
			m.readCh = make(chan tea.Msg, 64)
			return m, readLoopCmd(m.radio, m.readCh)
		}

	case statusLineMsg:
		m.status = msg.text

	case logLineMsg:
		m.logs = append(m.logs, msg.text)
		if len(m.logs) > maxLogLines {
			m.logs = m.logs[len(m.logs)-maxLogLines:]
		}
		return m, nextMsg(m.readCh)
	}
	return m, nil
}

func (m model) View() string {
	var btn string
	switch {
	case m.connecting:
		btn = styleButtonDis.Render("Connecting…")
	case m.connected:
		btn = styleButtonDis.Render("Connected")
	default:
		btn = styleButton.Render("[ Connect ]")
	}

	var statusText string
	if m.errMsg != "" {
		statusText = styleErr.Render(m.errMsg)
	} else if m.connected {
		statusText = styleConnected.Render(m.status)
	} else {
		statusText = styleStatus.Render(m.status)
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center, btn, "  ", statusText)

	// Subscription checkboxes — shown before connecting and when showSubs is true.
	var subsBlock string
	if !m.connected || m.showSubs {
		var sb strings.Builder
		for i, s := range m.subs {
			var box, label string
			if s.checked {
				box = styleChecked.Render("[x]")
			} else {
				box = styleUnchecked.Render("[ ]")
			}
			if s.checked {
				label = styleSubLabel.Render(s.label)
			} else {
				label = styleSubLabelDim.Render(s.label)
			}
			row := fmt.Sprintf(" %s %s", box, label)
			if (!m.connecting || m.showSubs) && i == m.cursor {
				row = styleCursor.Render("▶") + row
			} else {
				row = " " + row
			}
			sb.WriteString(row + "\n")
		}
		subsBlock = "\nSubscriptions:\n" + sb.String()
	}

	// Lines used: header(1) + subsBlock lines + separator(1) + help(1)
	subsLines := strings.Count(subsBlock, "\n")
	logHeight := m.height - 3 - subsLines
	if logHeight < 1 {
		logHeight = 1
	}

	wrapped := wrapLogEntries(m.logs, m.width-2)

	// Flatten wrapped entries into individual display lines.
	var displayLines []string
	for _, w := range wrapped {
		displayLines = append(displayLines, strings.Split(w, "\n")...)
	}

	// Clamp scrollOffset to valid range.
	total := len(displayLines)
	offset := m.scrollOffset
	maxOffset := total - logHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	// Select the visible window (offset lines up from the bottom).
	end := total - offset
	start := end - logHeight
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	logBlock := styleLog.Render(strings.Join(displayLines[start:end], "\n"))

	var helpText string
	switch {
	case m.connected && m.showSubs:
		helpText = "↑/↓: navigate   space: toggle sub   s/Tab: hide subs   q: quit"
	case m.connected && m.scrollOffset > 0:
		helpText = fmt.Sprintf("↑/↓/PgUp/PgDn: scroll   G/End: bottom   [+%d lines]   s: subscriptions   q: quit", m.scrollOffset)
	case m.connected:
		helpText = "↑/↓/PgUp/PgDn: scroll   s: subscriptions   q: quit"
	default:
		helpText = "↑/↓: navigate   space: toggle   enter: connect   q: quit"
	}
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(helpText)

	return fmt.Sprintf("%s%s%s\n%s\n%s",
		header,
		subsBlock,
		strings.Repeat("─", m.width),
		logBlock,
		help,
	)
}

// ─── Cmds ─────────────────────────────────────────────────────────────────────

// connectCmd returns a tea.Cmd that dials the radio using the selected subscriptions.
func (m model) connectCmd() tea.Cmd {
	subs := make([]string, 0, len(m.subs))
	for _, s := range m.subs {
		if s.checked {
			subs = append(subs, s.name)
		}
	}
	return func() tea.Msg {
		radio, err := Dial(radioAddr)
		if err != nil {
			return connectedMsg{errMsg: err.Error()}
		}

		for _, name := range subs {
			radio.Send(fmt.Sprintf("sub %s all", name), nil)
		}
		guiClientID := uuid.New().String()
		radio.Send(fmt.Sprintf("client gui %s", guiClientID), func(code int, body string) {
			if code == 0 {
				radio.Send("client program AetherSDR", nil)
				radio.Send("client station AetherSDR-Go", nil)
			}
		})

		return connectedMsg{radio: radio}
	}
}

// toggleSubCmd sends sub/unsub for a single subscription while connected.
func (m model) toggleSubCmd(s subscription) tea.Cmd {
	return func() tea.Msg {
		if s.checked {
			m.radio.Send(fmt.Sprintf("sub %s all", s.name), nil)
		} else {
			m.radio.Send(fmt.Sprintf("unsub %s all", s.name), nil)
		}
		return nil
	}
}

// nextMsg reads one message from ch as a Cmd.
func nextMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// readLoopCmd fans out status lines into ch and starts draining with nextMsg.
func readLoopCmd(radio *RadioConn, ch chan tea.Msg) tea.Cmd {
	radio.OnLog = func(direction, line string) {
		var text string
		if direction == "tx" {
			text = styleTx.Render("→ ") + line
		} else {
			text = styleRx.Render("← ") + line
		}
		ch <- logLineMsg{text: text}
	}
	go func() {
		err := radio.ReadLoop(func(msg ParsedMessage) {
			ch <- logLineMsg{text: fmt.Sprintf("%-30s %v", msg.Object, msg.KVs)}
		})
		if err != nil {
			ch <- statusLineMsg{text: fmt.Sprintf("disconnected: %v", err)}
		} else {
			ch <- statusLineMsg{text: "Disconnected"}
		}
		close(ch)
	}()
	return nextMsg(ch)
}

// ─── Log rendering ────────────────────────────────────────────────────────────

const contPrefix = "  ↳ "

var styleContPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// wrapLogEntries wraps each log entry to wrapWidth, indenting continuation lines.
func wrapLogEntries(entries []string, wrapWidth int) []string {
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	contPrefixLen := len([]rune(contPrefix))
	contWidth := wrapWidth - contPrefixLen
	if contWidth < 1 {
		contWidth = 1
	}

	wrapped := make([]string, 0, len(entries))
	for _, l := range entries {
		lines := wordWrap(l, wrapWidth)
		if len(lines) == 0 {
			wrapped = append(wrapped, l)
			continue
		}
		var sb strings.Builder
		sb.WriteString(lines[0])
		for _, cont := range lines[1:] {
			for _, cl := range wordWrap(cont, contWidth) {
				sb.WriteString("\n")
				sb.WriteString(styleContPrefix.Render(contPrefix))
				sb.WriteString(cl)
			}
		}
		wrapped = append(wrapped, sb.String())
	}
	return wrapped
}
