package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"goFlex/config"
	"goFlex/radio"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a model in its initial state for testing.
func newTestModel(opts ...modelOpt) model {
	cfg := &config.Config{MaxLog: 100}
	return newModel(cfg, append([]modelOpt{
		func(m *model) {
			m.connState.addr = "192.168.1.1:4992"
			m.dialFunc = func(string) (radio.RadioConn, error) {
				return nil, fmt.Errorf("mock dial: not implemented")
			}
			m.discoveryFunc = func(context.Context) (<-chan radio.DiscoveryEvent, error) {
				return nil, fmt.Errorf("mock discovery: not implemented")
			}
		},
	}, opts...)...)
}

// ─── Init ──────────────────────────────────────────────────────────────────

func TestModel_Init_ReturnsDiscoveryCmd(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil Init cmd")
	}
}

// ─── WindowSizeMsg ───────────────────────────────────────────────────────────

func TestModel_WindowSize_SetsDimensions(t *testing.T) {
	m := newTestModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := newM.(model)
	if m2.width != 80 {
		t.Errorf("width = %d, want 80", m2.width)
	}
	if m2.height != 24 {
		t.Errorf("height = %d, want 24", m2.height)
	}
}

// ─── KeyMsg: quit ────────────────────────────────────────────────────────────

func TestModel_KeyQuit_Q(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quit")
	}
}

func TestModel_KeyQuit_CtrlC(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quit")
	}
}

// ─── KeyMsg: tab / show subs ───────────────────────────────────────────────

func TestModel_KeyTab_TogglesShowSubs(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.showSubs = false

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := newM.(model)
	if !m2.ui.showSubs {
		t.Error("expected showSubs to be true after tab")
	}

	newM, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := newM.(model)
	if m3.ui.showSubs {
		t.Error("expected showSubs to be false after second tab")
	}
}

func TestModel_KeyTab_NoOpWhenDisconnected(t *testing.T) {
	m := newTestModel()
	m.connState.connected = false
	m.ui.showSubs = false

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := newM.(model)
	if m2.ui.showSubs {
		t.Error("expected showSubs to remain false when disconnected")
	}
}

// ─── KeyMsg: f / freq input ──────────────────────────────────────────────────

func TestModel_KeyF_EntersFreqMode(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.connState.conn = &radio.Conn{}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m2 := newM.(model)
	if !m2.ui.settingFreq {
		t.Error("expected settingFreq to be true after 'f'")
	}
}

func TestModel_KeyF_NoOpWhenDisconnected(t *testing.T) {
	m := newTestModel()
	m.connState.connected = false

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m2 := newM.(model)
	if m2.ui.settingFreq {
		t.Error("expected settingFreq to remain false when disconnected")
	}
}

// ─── KeyMsg: up / down (cursor) ──────────────────────────────────────────────

func TestModel_KeyUp_MovesCursor(t *testing.T) {
	m := newTestModel()
	m.ui.cursor = 1

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := newM.(model)
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

func TestModel_KeyUp_StopsAtZero(t *testing.T) {
	m := newTestModel()
	m.ui.cursor = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := newM.(model)
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

func TestModel_KeyDown_MovesCursor(t *testing.T) {
	m := newTestModel()
	m.ui.cursor = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := newM.(model)
	if m2.ui.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m2.ui.cursor)
	}
}

func TestModel_KeyDown_StopsAtMax(t *testing.T) {
	m := newTestModel()
	m.ui.cursor = len(m.ui.subs) - 1

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := newM.(model)
	if m2.ui.cursor != len(m.ui.subs)-1 {
		t.Errorf("cursor = %d, want %d", m2.ui.cursor, len(m.ui.subs)-1)
	}
}

// ─── KeyMsg: up / down (log scroll) ────────────────────────────────────────────

func TestModel_KeyUp_ScrollsLog(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 10
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9", "line10"}
	m.ui.scrollOffset = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := newM.(model)
	if m2.ui.scrollOffset <= 0 {
		t.Errorf("scrollOffset = %d, expected > 0", m2.ui.scrollOffset)
	}
}

func TestModel_KeyDown_ScrollsLogDown(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 10
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9", "line10"}
	m.ui.scrollOffset = 2

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := newM.(model)
	if m2.ui.scrollOffset != 1 {
		t.Errorf("scrollOffset = %d, want 1", m2.ui.scrollOffset)
	}
}

// ─── KeyMsg: pgup / pgdown ─────────────────────────────────────────────────

func TestModel_KeyPgUp_ScrollsByPage(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = make([]string, 50)
	for i := range m.log.entries {
		m.log.entries[i] = "log line"
	}
	m.ui.scrollOffset = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m2 := newM.(model)
	if m2.ui.scrollOffset <= 0 {
		t.Errorf("scrollOffset = %d, expected > 0 after pgup", m2.ui.scrollOffset)
	}
}

func TestModel_KeyPgDown_ScrollsByPage(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = make([]string, 50)
	for i := range m.log.entries {
		m.log.entries[i] = "log line"
	}
	m.ui.scrollOffset = 10

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m2 := newM.(model)
	if m2.ui.scrollOffset >= 10 {
		t.Errorf("scrollOffset = %d, expected < 10 after pgdown", m2.ui.scrollOffset)
	}
}

// ─── KeyMsg: G / end ─────────────────────────────────────────────────────────

func TestModel_KeyG_ScrollsToBottom(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3"}
	m.ui.scrollOffset = 5

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m2 := newM.(model)
	if m2.ui.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0", m2.ui.scrollOffset)
	}
}

func TestModel_KeyEnd_ScrollsToBottom(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3"}
	m.ui.scrollOffset = 5

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := newM.(model)
	if m2.ui.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0", m2.ui.scrollOffset)
	}
}

// ─── KeyMsg: space ───────────────────────────────────────────────────────────

func TestModel_KeySpace_TogglesSub(t *testing.T) {
	m := newTestModel()
	m.ui.cursor = 0
	initialChecked := m.ui.subs[0].checked

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := newM.(model)
	if m2.ui.subs[0].checked == initialChecked {
		t.Error("expected sub checked state to toggle")
	}
}

func TestModel_KeySpace_NoOpWhenDialing(t *testing.T) {
	m := newTestModel()
	m.connState.dialing = true
	m.ui.cursor = 0
	initialChecked := m.ui.subs[0].checked

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := newM.(model)
	if m2.ui.subs[0].checked != initialChecked {
		t.Error("expected sub checked state to remain unchanged while dialing")
	}
}

// ─── KeyMsg: enter (select radio) ──────────────────────────────────────────

func TestModel_KeyEnter_SelectsRadio(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1", Model: "6600", Address: "192.168.1.50", Port: 4992},
	}
	m.ui.cursor = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if m2.discovery.active {
		t.Error("expected discovery to be inactive after selecting radio")
	}
	if m2.connState.addr != "192.168.1.50:4992" {
		t.Errorf("addr = %q, want 192.168.1.50:4992", m2.connState.addr)
	}
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

func TestModel_KeyEnter_SelectsRadio_CursorOutOfBounds(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1", Model: "6600", Address: "192.168.1.50", Port: 4992},
	}
	m.ui.cursor = 5 // out of bounds

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

// ─── KeyMsg: enter (connect) ─────────────────────────────────────────────────

func TestModel_KeyEnter_StartsConnect(t *testing.T) {
	m := newTestModel()
	m.discovery.active = false
	m.connState.connected = false
	m.connState.dialing = false

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if !m2.connState.dialing {
		t.Error("expected dialing to be true after enter")
	}
	if m2.status != "Connecting…" {
		t.Errorf("status = %q, want 'Connecting…'", m2.status)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for connect")
	}
}

func TestModel_KeyEnter_NoOpWhenDialing(t *testing.T) {
	m := newTestModel()
	m.connState.dialing = true

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if !m2.connState.dialing {
		t.Error("expected dialing to remain true")
	}
	if cmd != nil {
		t.Error("expected nil cmd when already dialing")
	}
}

func TestModel_KeyEnter_NoOpWhenConnected(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd when already connected")
	}
}

// ─── Freq input mode ─────────────────────────────────────────────────────────

func TestModel_FreqInput_EscCancels(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14.3"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := newM.(model)
	if m2.ui.settingFreq {
		t.Error("expected settingFreq to be false after esc")
	}
	if m2.ui.freqInput != "" {
		t.Errorf("freqInput = %q, want empty", m2.ui.freqInput)
	}
}

func TestModel_FreqInput_CtrlCCancels(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14.3"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2 := newM.(model)
	if m2.ui.settingFreq {
		t.Error("expected settingFreq to be false after ctrl+c")
	}
}

func TestModel_FreqInput_EnterSends(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.connState.conn = &radio.Conn{}
	m.ui.settingFreq = true
	m.ui.freqInput = "14.300"

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if m2.ui.settingFreq {
		t.Error("expected settingFreq to be false after enter")
	}
	if m2.ui.freqInput != "" {
		t.Errorf("freqInput = %q, want empty", m2.ui.freqInput)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for setFreq")
	}
}

func TestModel_FreqInput_EnterEmptyNoOp(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.connState.conn = &radio.Conn{}
	m.ui.settingFreq = true
	m.ui.freqInput = ""

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if m2.ui.settingFreq {
		t.Error("expected settingFreq to be false after enter")
	}
	if cmd != nil {
		t.Error("expected nil cmd for empty freq")
	}
}

func TestModel_FreqInput_Backspace(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14.3"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := newM.(model)
	if m2.ui.freqInput != "14." {
		t.Errorf("freqInput = %q, want '14.'", m2.ui.freqInput)
	}
}

func TestModel_FreqInput_BackspaceEmpty(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = ""

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := newM.(model)
	if m2.ui.freqInput != "" {
		t.Errorf("freqInput = %q, want empty", m2.ui.freqInput)
	}
}

func TestModel_FreqInput_AcceptsDigits(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m2 := newM.(model)
	if m2.ui.freqInput != "143" {
		t.Errorf("freqInput = %q, want '143'", m2.ui.freqInput)
	}
}

func TestModel_FreqInput_AcceptsDecimal(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m2 := newM.(model)
	if m2.ui.freqInput != "14." {
		t.Errorf("freqInput = %q, want '14.'", m2.ui.freqInput)
	}
}

func TestModel_FreqInput_RejectsLetters(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14"

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m2 := newM.(model)
	if m2.ui.freqInput != "14" {
		t.Errorf("freqInput = %q, want '14'", m2.ui.freqInput)
	}
}

// ─── MouseMsg ──────────────────────────────────────────────────────────────

func TestModel_MouseWheelUp_ScrollsUp(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 10
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9", "line10"}
	m.ui.scrollOffset = 0

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m2 := newM.(model)
	if m2.ui.scrollOffset <= 0 {
		t.Errorf("scrollOffset = %d, expected > 0 after wheel up", m2.ui.scrollOffset)
	}
}

func TestModel_MouseWheelDown_ScrollsDown(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 10
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9", "line10"}
	m.ui.scrollOffset = 2

	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m2 := newM.(model)
	if m2.ui.scrollOffset != 1 {
		t.Errorf("scrollOffset = %d, want 1", m2.ui.scrollOffset)
	}
}

// ─── DiscoveryStartedMsg ─────────────────────────────────────────────────────

func TestModel_DiscoveryStarted_SetsActive(t *testing.T) {
	m := newTestModel()
	ch := make(chan tea.Msg)

	newM, cmd := m.Update(discoveryStartedMsg{ch: ch, cancel: func() {}})
	m2 := newM.(model)
	if !m2.discovery.active {
		t.Error("expected discovery.active to be true")
	}
	if m2.discovery.ch != ch {
		t.Error("expected discovery.ch to be set")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for nextMsg")
	}
}

// ─── RadioDiscoveredMsg ──────────────────────────────────────────────────────

func TestModel_RadioDiscovered_AddsRadio(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true

	newM, cmd := m.Update(radioDiscoveredMsg{info: radio.DiscoveredRadio{Serial: "S1", Model: "6600"}})
	m2 := newM.(model)
	if len(m2.discovery.radios) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(m2.discovery.radios))
	}
	if m2.discovery.radios[0].Serial != "S1" {
		t.Errorf("serial = %q, want S1", m2.discovery.radios[0].Serial)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for nextMsg")
	}
}

func TestModel_RadioDiscovered_UpdatesExisting(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{{Serial: "S1", Model: "6400"}}

	newM, _ := m.Update(radioDiscoveredMsg{info: radio.DiscoveredRadio{Serial: "S1", Model: "6600"}})
	m2 := newM.(model)
	if len(m2.discovery.radios) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(m2.discovery.radios))
	}
	if m2.discovery.radios[0].Model != "6600" {
		t.Errorf("model = %q, want 6600", m2.discovery.radios[0].Model)
	}
}

func TestModel_RadioDiscovered_KeepsCursorInBounds(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.ui.cursor = 5 // out of bounds for empty list

	newM, _ := m.Update(radioDiscoveredMsg{info: radio.DiscoveredRadio{Serial: "S1"}})
	m2 := newM.(model)
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

// ─── RadioLostMsg ──────────────────────────────────────────────────────────

func TestModel_RadioLost_RemovesRadio(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1"},
		{Serial: "S2"},
	}

	newM, _ := m.Update(radioLostMsg{serial: "S1"})
	m2 := newM.(model)
	if len(m2.discovery.radios) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(m2.discovery.radios))
	}
	if m2.discovery.radios[0].Serial != "S2" {
		t.Errorf("serial = %q, want S2", m2.discovery.radios[0].Serial)
	}
}

func TestModel_RadioLost_NotFound(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{{Serial: "S1"}}

	newM, _ := m.Update(radioLostMsg{serial: "S2"})
	m2 := newM.(model)
	if len(m2.discovery.radios) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(m2.discovery.radios))
	}
}

func TestModel_RadioLost_KeepsCursorInBounds(t *testing.T) {
	m := newTestModel()
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1"},
		{Serial: "S2"},
	}
	m.ui.cursor = 1

	newM, _ := m.Update(radioLostMsg{serial: "S2"})
	m2 := newM.(model)
	if m2.ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m2.ui.cursor)
	}
}

// ─── ConnectedMsg ────────────────────────────────────────────────────────────

func TestModel_ConnectedMsg_Success(t *testing.T) {
	m := newTestModel()
	m.connState.dialing = true
	conn := &radio.Conn{Handle: 1, Version: "3.0.0"}

	newM, cmd := m.Update(connectedMsg{conn: conn, initLogs: []string{"log1"}})
	m2 := newM.(model)
	if m2.connState.dialing {
		t.Error("expected dialing to be false")
	}
	if !m2.connState.connected {
		t.Error("expected connected to be true")
	}
	if m2.connState.conn != conn {
		t.Error("expected conn to be stored")
	}
	if len(m2.log.entries) != 1 || m2.log.entries[0] != "log1" {
		t.Errorf("entries = %v, want [log1]", m2.log.entries)
	}
	if !strings.Contains(m2.status, "Connected") {
		t.Errorf("status = %q, expected 'Connected'", m2.status)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for readLoop")
	}
}

func TestModel_ConnectedMsg_Failure(t *testing.T) {
	m := newTestModel()
	m.connState.dialing = true

	newM, _ := m.Update(connectedMsg{errMsg: "connection refused"})
	m2 := newM.(model)
	if m2.connState.dialing {
		t.Error("expected dialing to be false")
	}
	if m2.connState.connected {
		t.Error("expected connected to be false")
	}
	if m2.errMsg != "connection refused" {
		t.Errorf("errMsg = %q, want 'connection refused'", m2.errMsg)
	}
	if m2.status != "Disconnected" {
		t.Errorf("status = %q, want 'Disconnected'", m2.status)
	}
}

// ─── DisconnectedMsg ─────────────────────────────────────────────────────────

func TestModel_DisconnectedMsg_SetsState(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true
	m.connState.conn = &radio.Conn{}

	newM, cmd := m.Update(disconnectedMsg{conn: m.connState.conn, err: nil})
	m2 := newM.(model)
	if m2.connState.connected {
		t.Error("expected connected to be false")
	}
	if m2.connState.conn != nil {
		t.Error("expected conn to be nil")
	}
	if !strings.Contains(m2.status, "Disconnected") {
		t.Errorf("status = %q, expected 'Disconnected'", m2.status)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for reconnect")
	}
}

func TestModel_DisconnectedMsg_WithError(t *testing.T) {
	m := newTestModel()
	m.connState.connected = true

	newM, _ := m.Update(disconnectedMsg{err: fmt.Errorf("read error")})
	m2 := newM.(model)
	found := false
	for _, e := range m2.log.entries {
		if strings.Contains(e, "read error") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log to contain error, got %v", m2.log.entries)
	}
}

// ─── Reconnect messages ──────────────────────────────────────────────────────

func TestModel_ReconnectFailed_SetsStatus(t *testing.T) {
	m := newTestModel()

	newM, _ := m.Update(reconnectFailedMsg{errMsg: "timeout"})
	m2 := newM.(model)
	if !strings.Contains(m2.status, "timeout") {
		t.Errorf("status = %q, expected 'timeout'", m2.status)
	}
	found := false
	for _, e := range m2.log.entries {
		if strings.Contains(e, "timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log to contain error, got %v", m2.log.entries)
	}
}

func TestModel_ReconnectSuccess_SetsState(t *testing.T) {
	m := newTestModel()
	conn := &radio.Conn{Handle: 1, Version: "3.0.0"}

	newM, cmd := m.Update(reconnectSuccessMsg{conn: conn})
	m2 := newM.(model)
	if !m2.connState.connected {
		t.Error("expected connected to be true")
	}
	if m2.connState.conn != conn {
		t.Error("expected conn to be stored")
	}
	if !strings.Contains(m2.status, "Reconnected") {
		t.Errorf("status = %q, expected 'Reconnected'", m2.status)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for readLoop")
	}
}

// ─── StatusLineMsg ───────────────────────────────────────────────────────────

func TestModel_StatusLineMsg_UpdatesStatus(t *testing.T) {
	m := newTestModel()

	newM, _ := m.Update(statusLineMsg{text: "new status"})
	m2 := newM.(model)
	if m2.status != "new status" {
		t.Errorf("status = %q, want 'new status'", m2.status)
	}
}

// ─── LogLineMsg ──────────────────────────────────────────────────────────────

func TestModel_LogLineMsg_Appends(t *testing.T) {
	m := newTestModel()
	m.log.entries = []string{"existing"}

	newM, _ := m.Update(logLineMsg{text: "new line"})
	m2 := newM.(model)
	if len(m2.log.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m2.log.entries))
	}
	if m2.log.entries[1] != "new line" {
		t.Errorf("entries[1] = %q, want 'new line'", m2.log.entries[1])
	}
}

func TestModel_LogLineMsg_Truncates(t *testing.T) {
	m := newTestModel()
	m.log.max = 3
	m.log.entries = []string{"a", "b", "c"}

	newM, _ := m.Update(logLineMsg{text: "d"})
	m2 := newM.(model)
	if len(m2.log.entries) != 3 {
		t.Fatalf("expected 3 entries (max), got %d", len(m2.log.entries))
	}
	if m2.log.entries[2] != "d" {
		t.Errorf("entries[2] = %q, want 'd'", m2.log.entries[2])
	}
}

func TestModel_LogLineMsg_AdjustsScrollOffset(t *testing.T) {
	m := newTestModel()
	m.log.entries = []string{"a"}
	m.ui.scrollOffset = 2

	newM, _ := m.Update(logLineMsg{text: "b"})
	m2 := newM.(model)
	if m2.ui.scrollOffset != 3 {
		t.Errorf("scrollOffset = %d, want 3", m2.ui.scrollOffset)
	}
}

// ─── View rendering ──────────────────────────────────────────────────────────

func TestView_DiscoveryMode(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1", Model: "6600", Address: "192.168.1.50", Status: "Available"},
	}

	view := m.View()
	if !strings.Contains(view, "6600") {
		t.Error("expected view to contain radio model")
	}
	if !strings.Contains(view, "192.168.1.50") {
		t.Error("expected view to contain radio address")
	}
	if !strings.Contains(view, "Available") {
		t.Error("expected view to contain radio status")
	}
	if !strings.Contains(view, "▶") {
		t.Error("expected view to contain cursor indicator")
	}
}

func TestView_ConnectedMode(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 40
	m.connState.connected = true
	m.ui.showSubs = true
	m.status = "Connected  handle=0x1  version=3.0.0"
	m.log.entries = []string{"log line 1", "log line 2"}

	view := m.View()
	if !strings.Contains(view, "Connected") {
		t.Error("expected view to contain 'Connected'")
	}
	if !strings.Contains(view, "log line 1") {
		t.Error("expected view to contain log entries")
	}
	if !strings.Contains(view, "Subscriptions:") {
		t.Error("expected view to contain subscriptions panel")
	}
}

func TestView_FreqInputMode(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.settingFreq = true
	m.ui.freqInput = "14.300"

	view := m.View()
	if !strings.Contains(view, "Set frequency") {
		t.Error("expected view to contain freq prompt")
	}
	if !strings.Contains(view, "14.300") {
		t.Error("expected view to contain freq input")
	}
}

func TestView_DisconnectedMode(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = false
	m.status = "Scanning for radios…"

	view := m.View()
	if !strings.Contains(view, "Connect") {
		t.Error("expected view to contain connect button")
	}
}

func TestView_ErrorStatus(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = false
	m.errMsg = "connection refused"

	view := m.View()
	if !strings.Contains(view, "connection refused") {
		t.Error("expected view to contain error message")
	}
}

func TestView_ScrolledLog(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = make([]string, 50)
	for i := range m.log.entries {
		m.log.entries[i] = "log line"
	}
	m.ui.scrollOffset = 5

	view := m.View()
	if !strings.Contains(view, "+5 lines") {
		t.Error("expected view to contain '+5 lines' indicator")
	}
}

// ─── Helper functions ────────────────────────────────────────────────────────

func TestLogHeight_ConnectedNoSubs(t *testing.T) {
	m := newTestModel()
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false

	h := m.logHeight()
	want := 24 - 3 // height - 3 (header + sep + help)
	if h != want {
		t.Errorf("logHeight = %d, want %d", h, want)
	}
}

func TestLogHeight_ConnectedWithSubs(t *testing.T) {
	m := newTestModel()
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = true

	h := m.logHeight()
	want := 24 - 3 - (2 + len(m.ui.subs))
	if want < 1 {
		want = 1
	}
	if h != want {
		t.Errorf("logHeight = %d, want %d", h, want)
	}
}

func TestLogHeight_Disconnected(t *testing.T) {
	m := newTestModel()
	m.height = 24
	m.connState.connected = false

	h := m.logHeight()
	want := 24 - 3 - (2 + len(m.ui.subs))
	if want < 1 {
		want = 1
	}
	if h != want {
		t.Errorf("logHeight = %d, want %d", h, want)
	}
}

func TestLogHeight_SmallTerminal(t *testing.T) {
	m := newTestModel()
	m.height = 2
	m.connState.connected = true
	m.ui.showSubs = false

	h := m.logHeight()
	if h != 1 {
		t.Errorf("logHeight = %d, want 1 (minimum)", h)
	}
}

func TestMaxScrollOffset_Empty(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{}

	off := m.maxScrollOffset()
	if off != 0 {
		t.Errorf("maxScrollOffset = %d, want 0", off)
	}
}

func TestMaxScrollOffset_WithContent(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	// Fill log with enough entries to exceed logHeight.
	m.log.entries = make([]string, 100)
	for i := range m.log.entries {
		m.log.entries[i] = "a very long log line that will definitely wrap to multiple lines when rendered at this width"
	}

	off := m.maxScrollOffset()
	if off <= 0 {
		t.Errorf("maxScrollOffset = %d, expected > 0", off)
	}
}

func TestWithScrollUp_RespectsBounds(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1"}
	m.ui.scrollOffset = 0

	m2 := m.withScrollUp()
	// With only 1 line, maxScrollOffset is 0, so scrollUp should not change.
	if m2.ui.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0", m2.ui.scrollOffset)
	}
}

func TestWithScrollDown_RespectsBounds(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.connState.connected = true
	m.ui.showSubs = false
	m.log.entries = []string{"line1"}
	m.ui.scrollOffset = 0

	m2 := m.withScrollDown()
	if m2.ui.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0", m2.ui.scrollOffset)
	}
}

func TestBuildScrollbar_Empty(t *testing.T) {
	bar := buildScrollbar(10, 0, 0)
	if len(bar) != 10 {
		t.Fatalf("expected length 10, got %d", len(bar))
	}
	for i, s := range bar {
		if !strings.Contains(s, "│") {
			t.Errorf("bar[%d] = %q, expected track character", i, s)
		}
	}
}

func TestBuildScrollbar_WithThumb(t *testing.T) {
	bar := buildScrollbar(10, 100, 50)
	if len(bar) != 10 {
		t.Fatalf("expected length 10, got %d", len(bar))
	}
	// At least one element should be the thumb.
	hasThumb := false
	for _, s := range bar {
		if strings.Contains(s, "┃") {
			hasThumb = true
			break
		}
	}
	if !hasThumb {
		t.Error("expected scrollbar to contain thumb")
	}
}

func TestBuildScrollbar_NoThumbWhenFits(t *testing.T) {
	bar := buildScrollbar(10, 5, 0)
	if len(bar) != 10 {
		t.Fatalf("expected length 10, got %d", len(bar))
	}
	for i, s := range bar {
		if strings.Contains(s, "┃") {
			t.Errorf("bar[%d] = %q, expected no thumb when content fits", i, s)
		}
	}
}
