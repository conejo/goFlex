// cmd_test.go — tests for I/O-bound tea.Cmd functions using mock dependencies.

package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"goFlex/radio"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Mock RadioConn ─────────────────────────────────────────────────────────

type mockRadioConn struct {
	sendCalled    bool
	sendCmd       string
	sendSeq       uint32
	sendErr       error
	enableRecon   bool
	disableRecon  bool
	closeCalled   bool
	readLoopErr   error
	readLoopCb    func(radio.ParsedMessage)
	readLoopBlock chan struct{}
	handle        uint32
	version       string
	state         radio.ConnectionState
	onLog         func(string, string)
	onStateChange func(radio.ConnectionState, radio.ConnectionState)
	onPingRtt     func(int)
	reconnectDone chan struct{}
}

func (m *mockRadioConn) Send(cmd string, cb func(int, string)) (uint32, error) {
	m.sendCalled = true
	m.sendCmd = cmd
	return m.sendSeq, m.sendErr
}
func (m *mockRadioConn) EnableReconnect()  { m.enableRecon = true }
func (m *mockRadioConn) DisableReconnect() { m.disableRecon = true }
func (m *mockRadioConn) Close()            { m.closeCalled = true }
func (m *mockRadioConn) ReadLoop(onStatus func(radio.ParsedMessage)) error {
	if m.readLoopCb != nil {
		m.readLoopCb = onStatus
	}
	// If a block channel is set, wait on it to allow tests to control timing.
	if m.readLoopBlock != nil {
		<-m.readLoopBlock
	}
	return m.readLoopErr
}
func (m *mockRadioConn) ReconnectDone() <-chan struct{}   { return m.reconnectDone }
func (m *mockRadioConn) State() radio.ConnectionState     { return m.state }
func (m *mockRadioConn) GetHandle() uint32                { return m.handle }
func (m *mockRadioConn) GetVersion() string               { return m.version }
func (m *mockRadioConn) SetOnLog(fn func(string, string)) { m.onLog = fn }
func (m *mockRadioConn) SetOnStateChange(fn func(radio.ConnectionState, radio.ConnectionState)) {
	m.onStateChange = fn
}
func (m *mockRadioConn) SetOnPingRtt(fn func(int)) { m.onPingRtt = fn }
func (m *mockRadioConn) OnDisconnected()           {}

// ─── connectCmd tests ───────────────────────────────────────────────────────

func TestConnectCmd_Success(t *testing.T) {
	mock := &mockRadioConn{handle: 0xABCD, version: "3.0.0"}
	m := newTestModel(func(m *model) {
		m.dialFunc = func(string) (radio.RadioConn, error) { return mock, nil }
		m.ui.subs[0].checked = true // slice
		m.ui.subs[1].checked = true // pan
	})

	cmd := m.connectCmd()
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		t.Fatalf("expected connectedMsg, got %T", msg)
	}
	if cm.errMsg != "" {
		t.Errorf("unexpected errMsg: %q", cm.errMsg)
	}
	if cm.conn != mock {
		t.Error("expected conn to be mock")
	}
	if !mock.sendCalled {
		t.Error("expected Send to be called")
	}
	// connectCmd sends multiple commands; the last one recorded is "client gui ...".
	if !strings.Contains(mock.sendCmd, "client gui") {
		t.Errorf("expected sendCmd to contain 'client gui', got %q", mock.sendCmd)
	}
	// EnableReconnect is called in Update(connectedMsg), not inside connectCmd.
}

func TestConnectCmd_DialFailure(t *testing.T) {
	m := newTestModel(func(m *model) {
		m.dialFunc = func(string) (radio.RadioConn, error) {
			return nil, fmt.Errorf("connection refused")
		}
	})

	cmd := m.connectCmd()
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		t.Fatalf("expected connectedMsg, got %T", msg)
	}
	if cm.errMsg != "connection refused" {
		t.Errorf("errMsg = %q, want 'connection refused'", cm.errMsg)
	}
	if cm.conn != nil {
		t.Error("expected conn to be nil on failure")
	}
}

// ─── toggleSubCmd tests ─────────────────────────────────────────────────────

func TestToggleSubCmd_Subscribe(t *testing.T) {
	mock := &mockRadioConn{}
	s := subscription{name: "slice", checked: true}
	cmd := toggleSubCmd(mock, s)
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg, got %T", msg)
	}
	if mock.sendCmd != "sub slice all" {
		t.Errorf("sendCmd = %q, want 'sub slice all'", mock.sendCmd)
	}
}

func TestToggleSubCmd_Unsubscribe(t *testing.T) {
	mock := &mockRadioConn{}
	s := subscription{name: "pan", checked: false}
	cmd := toggleSubCmd(mock, s)
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg, got %T", msg)
	}
	if mock.sendCmd != "unsub pan all" {
		t.Errorf("sendCmd = %q, want 'unsub pan all'", mock.sendCmd)
	}
}

func TestToggleSubCmd_SendError(t *testing.T) {
	mock := &mockRadioConn{sendErr: fmt.Errorf("write error")}
	s := subscription{name: "slice", checked: true}
	cmd := toggleSubCmd(mock, s)
	msg := cmd()
	lm, ok := msg.(logLineMsg)
	if !ok {
		t.Fatalf("expected logLineMsg, got %T", msg)
	}
	if !strings.Contains(lm.text, "write error") {
		t.Errorf("expected log to contain 'write error', got %q", lm.text)
	}
}

// ─── setFreqCmd tests ───────────────────────────────────────────────────────

func TestSetFreqCmd_Success(t *testing.T) {
	mock := &mockRadioConn{}
	cmd := setFreqCmd(mock, "14.300")
	msg := cmd()
	lm, ok := msg.(logLineMsg)
	if !ok {
		t.Fatalf("expected logLineMsg, got %T", msg)
	}
	if !strings.Contains(lm.text, "14.300000") {
		t.Errorf("expected log to contain '14.300000', got %q", lm.text)
	}
	if !strings.Contains(mock.sendCmd, "slice tune 0 14.300000 autopan=0") {
		t.Errorf("expected sendCmd to contain tune command, got %q", mock.sendCmd)
	}
}

func TestSetFreqCmd_InvalidFreq(t *testing.T) {
	mock := &mockRadioConn{}
	cmd := setFreqCmd(mock, "abc")
	msg := cmd()
	lm, ok := msg.(logLineMsg)
	if !ok {
		t.Fatalf("expected logLineMsg, got %T", msg)
	}
	if !strings.Contains(lm.text, "invalid frequency") {
		t.Errorf("expected log to contain 'invalid frequency', got %q", lm.text)
	}
}

func TestSetFreqCmd_SendError(t *testing.T) {
	mock := &mockRadioConn{sendErr: fmt.Errorf("write error")}
	cmd := setFreqCmd(mock, "14.300")
	msg := cmd()
	lm, ok := msg.(logLineMsg)
	if !ok {
		t.Fatalf("expected logLineMsg, got %T", msg)
	}
	if !strings.Contains(lm.text, "failed to send tune") {
		t.Errorf("expected log to contain 'failed to send tune', got %q", lm.text)
	}
}

// ─── readLoopCmd tests ──────────────────────────────────────────────────────

func TestReadLoopCmd_SetsOnLog(t *testing.T) {
	mock := &mockRadioConn{}
	ch := make(chan tea.Msg, 64)
	ch <- logLineMsg{text: "preloaded"} // so nextMsg doesn't block
	cmd := readLoopCmd(mock, ch)
	msg := cmd()
	if msg == nil {
		t.Fatal("expected non-nil msg")
	}
	if mock.onLog == nil {
		t.Error("expected OnLog to be set")
	}
}

func TestReadLoopCmd_ProducesDisconnectedMsg(t *testing.T) {
	mock := &mockRadioConn{readLoopErr: fmt.Errorf("read error")}
	ch := make(chan tea.Msg, 64)
	ch <- logLineMsg{text: "dummy"} // preload so nextMsg doesn't block

	cmd := readLoopCmd(mock, ch)
	msg := cmd() // returns the preloaded message via nextMsg
	if msg == nil {
		t.Fatal("expected non-nil msg")
	}

	// The goroutine has sent disconnectedMsg by now.
	msg2 := nextMsg(ch)()
	dm, ok := msg2.(disconnectedMsg)
	if !ok {
		t.Fatalf("expected disconnectedMsg, got %T", msg2)
	}
	if dm.err == nil {
		t.Error("expected non-nil err")
	}
}

// ─── reconnectCmd tests ─────────────────────────────────────────────────────

func TestReconnectCmd_Success(t *testing.T) {
	done := make(chan struct{})
	close(done)
	mock := &mockRadioConn{reconnectDone: done, state: radio.StateConnected}
	cmd := reconnectCmd(mock)
	msg := cmd()
	rm, ok := msg.(reconnectSuccessMsg)
	if !ok {
		t.Fatalf("expected reconnectSuccessMsg, got %T", msg)
	}
	if rm.conn != mock {
		t.Error("expected conn to be mock")
	}
}

func TestReconnectCmd_FailureStateNotConnected(t *testing.T) {
	done := make(chan struct{})
	close(done)
	mock := &mockRadioConn{reconnectDone: done, state: radio.StateDisconnected}
	cmd := reconnectCmd(mock)
	msg := cmd()
	rm, ok := msg.(reconnectFailedMsg)
	if !ok {
		t.Fatalf("expected reconnectFailedMsg, got %T", msg)
	}
	if rm.errMsg != "reconnect cancelled" {
		t.Errorf("errMsg = %q, want 'reconnect cancelled'", rm.errMsg)
	}
}

func TestReconnectCmd_NotEnabled(t *testing.T) {
	mock := &mockRadioConn{reconnectDone: nil}
	cmd := reconnectCmd(mock)
	msg := cmd()
	rm, ok := msg.(reconnectFailedMsg)
	if !ok {
		t.Fatalf("expected reconnectFailedMsg, got %T", msg)
	}
	if rm.errMsg != "reconnect not enabled" {
		t.Errorf("errMsg = %q, want 'reconnect not enabled'", rm.errMsg)
	}
}

// ─── startDiscoveryCmd tests ────────────────────────────────────────────────

func TestStartDiscoveryCmd_Success(t *testing.T) {
	evtCh := make(chan radio.DiscoveryEvent, 2)
	evtCh <- radio.DiscoveryEvent{Radio: radio.DiscoveredRadio{Serial: "S1", Model: "6600"}}
	close(evtCh)

	m := newTestModel(func(m *model) {
		m.discoveryFunc = func(context.Context) (<-chan radio.DiscoveryEvent, error) {
			return evtCh, nil
		}
	})

	cmd := m.startDiscoveryCmd()
	msg := cmd()
	dsm, ok := msg.(discoveryStartedMsg)
	if !ok {
		t.Fatalf("expected discoveryStartedMsg, got %T", msg)
	}
	if dsm.cancel == nil {
		t.Error("expected cancel to be set")
	}

	// Drain the channel to verify events are translated.
	var found bool
	for msg := range dsm.ch {
		if _, ok := msg.(radioDiscoveredMsg); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected radioDiscoveredMsg in channel")
	}
}

func TestStartDiscoveryCmd_Error(t *testing.T) {
	m := newTestModel(func(m *model) {
		m.discoveryFunc = func(context.Context) (<-chan radio.DiscoveryEvent, error) {
			return nil, fmt.Errorf("bind error")
		}
	})

	cmd := m.startDiscoveryCmd()
	msg := cmd()
	slm, ok := msg.(statusLineMsg)
	if !ok {
		t.Fatalf("expected statusLineMsg, got %T", msg)
	}
	if !strings.Contains(slm.text, "bind error") {
		t.Errorf("expected status to contain 'bind error', got %q", slm.text)
	}
}

// ─── Full lifecycle message flow test ───────────────────────────────────────

func TestLifecycle_ConnectionFlow(t *testing.T) {
	mock := &mockRadioConn{handle: 0x1, version: "3.0.0"}
	m := newTestModel(func(m *model) {
		m.dialFunc = func(string) (radio.RadioConn, error) { return mock, nil }
	})
	m.discovery.active = true
	m.discovery.radios = []radio.DiscoveredRadio{
		{Serial: "S1", Model: "6600", Address: "192.168.1.50", Port: 4992},
	}

	// 1. Select radio from discovery list.
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newM.(model)
	if m2.discovery.active {
		t.Error("expected discovery to be inactive after select")
	}
	if m2.connState.addr != "192.168.1.50:4992" {
		t.Errorf("addr = %q, want 192.168.1.50:4992", m2.connState.addr)
	}

	// 2. Press enter again to connect.
	newM, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := newM.(model)
	if !m3.connState.dialing {
		t.Error("expected dialing to be true")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd for connect")
	}

	// 3. Simulate connectedMsg.
	newM, cmd = m3.Update(connectedMsg{conn: mock, initLogs: []string{"log1"}})
	m4 := newM.(model)
	if !m4.connState.connected {
		t.Error("expected connected to be true")
	}
	if m4.connState.conn != mock {
		t.Error("expected conn to be mock")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for readLoop")
	}

	// 4. Simulate logLineMsg.
	newM, _ = m4.Update(logLineMsg{text: "status update"})
	m5 := newM.(model)
	if len(m5.log.entries) == 0 || m5.log.entries[len(m5.log.entries)-1] != "status update" {
		t.Error("expected log to contain 'status update'")
	}

	// 5. Simulate disconnectedMsg.
	newM, cmd = m5.Update(disconnectedMsg{conn: mock, err: fmt.Errorf("read error")})
	m6 := newM.(model)
	if m6.connState.connected {
		t.Error("expected connected to be false")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for reconnect")
	}

	// 6. Simulate reconnectSuccessMsg.
	newM, cmd = m6.Update(reconnectSuccessMsg{conn: mock})
	m7 := newM.(model)
	if !m7.connState.connected {
		t.Error("expected connected to be true after reconnect")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for readLoop after reconnect")
	}
}
