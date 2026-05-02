// tui.go — Bubble Tea TUI for the FlexRadio client.

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"goFlex/config"
	"goFlex/radio"
)

const maxLogLines = 500

// ─── Tea messages ─────────────────────────────────────────────────────────────

type connectedMsg struct {
	conn     *radio.Conn
	errMsg   string
	initLogs []string
}

type disconnectedMsg struct{ conn *radio.Conn }
type reconnectFailedMsg struct{ errMsg string }
type reconnectSuccessMsg struct{ conn *radio.Conn }

type statusLineMsg struct{ text string }
type logLineMsg struct{ text string }

type discoveryStartedMsg struct {
	ch     chan tea.Msg
	cancel context.CancelFunc
}
type radioDiscoveredMsg struct{ info radio.RadioInfo }
type radioLostMsg struct{ serial string }

// ─── Subscription list ────────────────────────────────────────────────────────

type subscription struct {
	name    string // protocol name, e.g. "slice"
	label   string // display label
	checked bool
}

func newDefaultSubs() []subscription {
	return []subscription{
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
}

// ─── Model ────────────────────────────────────────────────────────────────────

type model struct {
	// Discovery phase
	discovering    bool
	radios         []radio.RadioInfo
	discoverCh     chan tea.Msg
	cancelDiscover context.CancelFunc

	// Connection
	addr       string
	connected  bool
	connecting bool
	conn       *radio.Conn

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
	maxLog       int  // maximum number of log entries to retain
	cfg        *config.Config
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
	styleSubLabelDim = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTx          = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // blue — outgoing commands
	styleRx          = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber — responses
	styleScrollbar   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleScrollThumb = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	styleRadioItem   = lipgloss.NewStyle().Foreground(lipgloss.Color("255")) // bright white
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

// maxScrollOffset computes the largest valid scrollOffset for the current log content.
func (m model) maxScrollOffset() int {
	wrapped := wrapLogEntries(m.logs, m.width-2)
	var total int
	for _, w := range wrapped {
		total += strings.Count(w, "\n") + 1
	}
	if max := total - m.logHeight(); max > 0 {
		return max
	}
	return 0
}

// withScrollUp moves the cursor up or scrolls the log up by one line.
func (m model) withScrollUp() model {
	if m.connected && !m.showSubs {
		if max := m.maxScrollOffset(); m.scrollOffset < max {
			m.scrollOffset++
		}
	} else if !m.connecting && m.cursor > 0 {
		m.cursor--
	}
	return m
}

// withScrollDown moves the cursor down or scrolls the log down.
func (m model) withScrollDown() model {
	if m.connected && !m.showSubs {
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	} else if !m.connecting {
		var maxCursor int
		if m.discovering {
			maxCursor = len(m.radios) - 1
		} else {
			maxCursor = len(m.subs) - 1
		}
		if m.cursor < maxCursor {
			m.cursor++
		}
	}
	return m
}

// ─── Init / Update / View ─────────────────────────────────────────────────────

func (m model) Init() tea.Cmd { return startDiscoveryCmd() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.conn != nil {
				m.conn.Close()
			}
			if m.cancelDiscover != nil {
				m.cancelDiscover()
			}
			return m, tea.Quit
		case "s", "tab":
			if m.connected {
				m.showSubs = !m.showSubs
			}
		case "up", "k":
			m = m.withScrollUp()
		case "down", "j":
			m = m.withScrollDown()
		case "pgup":
			m.scrollOffset += m.logHeight()
			if max := m.maxScrollOffset(); m.scrollOffset > max {
				m.scrollOffset = max
			}
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
				if m.connected && m.conn != nil {
					return m, toggleSubCmd(m.conn, m.subs[m.cursor])
				}
			}
		case "enter":
			if m.discovering && len(m.radios) > 0 {
				// Select the highlighted radio and move to subscription screen.
				if m.cursor >= len(m.radios) {
					m.cursor = len(m.radios) - 1
				}
				sel := m.radios[m.cursor]
				if m.cancelDiscover != nil {
					m.cancelDiscover()
				}
				m.discovering = false
				m.addr = fmt.Sprintf("%s:%d", sel.Address, sel.Port)
				m.status = fmt.Sprintf("%s  %s", sel.Model, sel.Address)
				m.cursor = 0
			} else if !m.connected && !m.connecting && !m.discovering {
				m.connecting = true
				m.errMsg = ""
				m.status = "Connecting…"
				return m, m.connectCmd()
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m = m.withScrollUp()
		case tea.MouseButtonWheelDown:
			m = m.withScrollDown()
		}

	case discoveryStartedMsg:
		m.discovering = true
		m.discoverCh = msg.ch
		m.cancelDiscover = msg.cancel
		return m, nextMsg(m.discoverCh)

	case radioDiscoveredMsg:
		m.radios = upsertRadio(m.radios, msg.info)
		// Keep cursor in bounds.
		if m.cursor >= len(m.radios) {
			m.cursor = len(m.radios) - 1
		}
		if m.discovering {
			return m, nextMsg(m.discoverCh)
		}

	case radioLostMsg:
		m.radios = removeRadio(m.radios, msg.serial)
		if m.cursor >= len(m.radios) && m.cursor > 0 {
			m.cursor = len(m.radios) - 1
		}
		if m.discovering {
			return m, nextMsg(m.discoverCh)
		}

	case connectedMsg:
		m.connecting = false
		if msg.errMsg != "" {
			m.errMsg = msg.errMsg
			m.status = "Disconnected"
		} else {
			m.connected = true
			m.conn = msg.conn
			m.logs = append(m.logs, msg.initLogs...)
			m.status = fmt.Sprintf("Connected  handle=0x%X  version=%s", m.conn.Handle, m.conn.Version)
			m.readCh = make(chan tea.Msg, 64)
			m.conn.EnableReconnect()
			m.conn.OnStateChange = func(oldState, newState radio.ConnectionState) {
				m.logs = append(m.logs, fmt.Sprintf("[state] %s → %s", oldState, newState))
			}
			m.conn.OnPingRtt = func(ms int) {
				m.logs = append(m.logs, fmt.Sprintf("[ping] RTT %d ms", ms))
			}
			return m, readLoopCmd(m.conn, m.readCh)
		}

	case disconnectedMsg:
		m.connected = false
		m.conn = nil
		m.status = "Disconnected — reconnecting…"
		m.logs = append(m.logs, "[conn] connection lost, attempting reconnect")
		return m, reconnectCmd(msg.conn)

	case reconnectFailedMsg:
		m.status = fmt.Sprintf("Reconnect failed: %s", msg.errMsg)
		m.logs = append(m.logs, fmt.Sprintf("[conn] reconnect failed: %s", msg.errMsg))

	case reconnectSuccessMsg:
		m.connected = true
		m.conn = msg.conn
		m.status = fmt.Sprintf("Reconnected  handle=0x%X  version=%s", m.conn.Handle, m.conn.Version)
		m.logs = append(m.logs, "[conn] reconnected successfully")
		m.readCh = make(chan tea.Msg, 64)
		return m, readLoopCmd(m.conn, m.readCh)

	case statusLineMsg:
		m.status = msg.text

	case logLineMsg:
		m.logs = append(m.logs, msg.text)
		if len(m.logs) > m.maxLog {
			m.logs = m.logs[len(m.logs)-m.maxLog:]
		}
		// Anchor the viewport: when scrolled up, compensate for the new line
		// added at the bottom so the visible content doesn't drift downward.
		if m.scrollOffset > 0 {
			m.scrollOffset++
		}
		return m, nextMsg(m.readCh)
	}
	return m, nil
}

func (m model) viewHeader() string {
	var btn string
	switch {
	case m.discovering:
		btn = styleButtonDis.Render("Discovering…")
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
	return lipgloss.JoinHorizontal(lipgloss.Center, btn, "  ", statusText)
}

func (m model) viewRadioList() string {
	if len(m.radios) == 0 {
		return styleSubLabelDim.Render("  Scanning for radios…")
	}
	var sb strings.Builder
	for i, r := range m.radios {
		var statusStyle lipgloss.Style
		if r.Status == "Available" {
			statusStyle = styleConnected
		} else {
			statusStyle = styleErr
		}
		line := fmt.Sprintf(" %-14s %-16s %s",
			styleRadioItem.Render(r.Model), styleRadioItem.Render(r.Address), statusStyle.Render(r.Status))
		if i == m.cursor {
			line = styleCursor.Render("▶") + line
		} else {
			line = " " + line
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func (m model) viewSubsPanel() string {
	if m.connected && !m.showSubs {
		return ""
	}
	var sb strings.Builder
	for i, s := range m.subs {
		var box, label string
		if s.checked {
			box = styleChecked.Render("[x]")
			label = s.label
		} else {
			box = styleUnchecked.Render("[ ]")
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
	return "\nSubscriptions:\n" + sb.String()
}

func (m model) viewLogPane() string {
	logHeight := m.logHeight()
	// Reserve 1 column on the right for the scrollbar.
	wrapWidth := m.width - 3
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	wrapped := wrapLogEntries(m.logs, wrapWidth)
	var displayLines []string
	for _, w := range wrapped {
		displayLines = append(displayLines, strings.Split(w, "\n")...)
	}
	total := len(displayLines)
	offset := m.scrollOffset
	maxOffset := total - logHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := total - offset
	start := end - logHeight
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	visible := displayLines[start:end]

	bar := buildScrollbar(logHeight, total, start)

	var sb strings.Builder
	for i, line := range visible {
		// Pad to wrapWidth so the scrollbar column stays aligned regardless
		// of the visual width of each log line (which may contain ANSI codes).
		pad := wrapWidth - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(line)
		sb.WriteString(strings.Repeat(" ", pad+1))
		sb.WriteString(bar[i])
		if i < len(visible)-1 {
			sb.WriteString("\n")
		}
	}
	return styleLog.Render(sb.String())
}

// buildScrollbar returns a slice of len `height` single-character strings
// representing a minimal scrollbar for content of `total` lines with `start`
// being the first visible line.
func buildScrollbar(height, total, start int) []string {
	bar := make([]string, height)
	track := styleScrollbar.Render("│")
	for i := range bar {
		bar[i] = track
	}
	if total <= height {
		return bar
	}
	thumbH := height * height / total
	if thumbH < 1 {
		thumbH = 1
	}
	maxStart := total - height
	thumbTop := 0
	if maxStart > 0 {
		thumbTop = start * (height - thumbH) / maxStart
	}
	for i := thumbTop; i < thumbTop+thumbH && i < height; i++ {
		bar[i] = styleScrollThumb.Render("┃")
	}
	return bar
}

func (m model) viewHelp() string {
	var text string
	switch {
	case m.discovering:
		text = "↑/↓: navigate   enter: select radio   q: quit"
	case m.connected && m.showSubs:
		text = "↑/↓: navigate   space: toggle sub   s/Tab: hide subs   q: quit"
	case m.connected && m.scrollOffset > 0:
		text = fmt.Sprintf("↑/↓/PgUp/PgDn: scroll   G/End: bottom   [+%d lines]   s: subscriptions   q: quit", m.scrollOffset)
	case m.connected:
		text = "↑/↓/PgUp/PgDn: scroll   s: subscriptions   q: quit"
	default:
		text = "↑/↓: navigate   space: toggle   enter: connect   q: quit"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(text)
}

func (m model) View() string {
	sep := strings.Repeat("─", m.width)
	if m.discovering {
		return strings.Join([]string{
			m.viewHeader() + "\n" + sep,
			m.viewRadioList(),
			m.viewHelp(),
		}, "\n")
	}
	return strings.Join([]string{
		m.viewHeader() + m.viewSubsPanel() + sep,
		m.viewLogPane(),
		m.viewHelp(),
	}, "\n")
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
		conn, err := radio.Dial(m.addr)
		if err != nil {
			return connectedMsg{errMsg: err.Error()}
		}

		var initLogs []string
		conn.OnLog = func(dir, line string) {
			var text string
			if dir == "tx" {
				text = styleTx.Render("→ ") + line
			} else {
				text = styleRx.Render("← ") + line
			}
			initLogs = append(initLogs, text)
		}

		for _, name := range subs {
			conn.Send(fmt.Sprintf("sub %s all", name), nil)
		}
		guiClientID := uuid.New().String()
		conn.Send(fmt.Sprintf("client gui %s", guiClientID), func(code int, body string) {
			if code == 0 {
				conn.Send("client program AetherSDR", nil)
				conn.Send("client station AetherSDR-Go", nil)
			}
		})

		return connectedMsg{conn: conn, initLogs: initLogs}
	}
}

// toggleSubCmd sends sub/unsub for a single subscription while connected.
func toggleSubCmd(conn *radio.Conn, s subscription) tea.Cmd {
	return func() tea.Msg {
		if s.checked {
			conn.Send(fmt.Sprintf("sub %s all", s.name), nil)
		} else {
			conn.Send(fmt.Sprintf("unsub %s all", s.name), nil)
		}
		return nil
	}
}

// startDiscoveryCmd opens the UDP discovery socket and returns a discoveryStartedMsg.
func startDiscoveryCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		evtCh, err := radio.Listen(ctx)
		if err != nil {
			cancel()
			return statusLineMsg{text: fmt.Sprintf("discovery error: %v", err)}
		}
		ch := make(chan tea.Msg, 16)
		go func() {
			for evt := range evtCh {
				if evt.Lost {
					ch <- radioLostMsg{serial: evt.Radio.Serial}
				} else {
					ch <- radioDiscoveredMsg{info: evt.Radio}
				}
			}
			close(ch)
		}()
		return discoveryStartedMsg{ch: ch, cancel: cancel}
	}
}

// upsertRadio adds or updates a radio in the list, keyed by serial.
func upsertRadio(radios []radio.RadioInfo, info radio.RadioInfo) []radio.RadioInfo {
	for i, r := range radios {
		if r.Serial == info.Serial {
			radios[i] = info
			return radios
		}
	}
	return append(radios, info)
}

// removeRadio removes the radio with the given serial from the list.
func removeRadio(radios []radio.RadioInfo, serial string) []radio.RadioInfo {
	for i, r := range radios {
		if r.Serial == serial {
			return append(radios[:i], radios[i+1:]...)
		}
	}
	return radios
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
func readLoopCmd(conn *radio.Conn, ch chan tea.Msg) tea.Cmd {
	conn.OnLog = func(direction, line string) {
		var text string
		if direction == "tx" {
			text = styleTx.Render("→ ") + line
		} else {
			text = styleRx.Render("← ") + line
		}
		ch <- logLineMsg{text: text}
	}
	go func() {
		err := conn.ReadLoop(func(msg radio.ParsedMessage) {
			ch <- logLineMsg{text: fmt.Sprintf("%-30s %v", msg.Object, msg.KVs)}
		})
		if err != nil {
			ch <- disconnectedMsg{conn: conn}
		} else {
			ch <- disconnectedMsg{conn: conn}
		}
		close(ch)
	}()
	return nextMsg(ch)
}

// reconnectCmd attempts to re-dial the radio after a disconnect.
func reconnectCmd(oldConn *radio.Conn) tea.Cmd {
	return func() tea.Msg {
		oldConn.OnDisconnected()
		// Wait for the reconnect timer to fire and attempt a new connection.
		// The Conn's OnDisconnected schedules the re-dial internally.
		// We poll briefly to see if a new connection was established.
		for i := 0; i < 60; i++ {
			time.Sleep(500 * time.Millisecond)
			if oldConn.State() == radio.StateConnected {
				return reconnectSuccessMsg{conn: oldConn}
			}
		}
		return reconnectFailedMsg{errMsg: "timeout waiting for reconnect"}
	}
}
