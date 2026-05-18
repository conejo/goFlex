package web

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"goFlex/config"
	"goFlex/radio"
)

// ─── Hub state tests ───────────────────────────────────────────────────────

func TestHub_AppendLog(t *testing.T) {
	hub := &Hub{
		cfg: &config.Config{MaxLog: 10},
	}
	hub.appendLog("line 1")
	hub.appendLog("line 2")

	entries := hub.LogEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0] != "line 2" {
		t.Errorf("entries[0] = %q, want 'line 2'", entries[0])
	}
	if entries[1] != "line 1" {
		t.Errorf("entries[1] = %q, want 'line 1'", entries[1])
	}
}

func TestHub_AppendLog_Truncates(t *testing.T) {
	hub := &Hub{
		cfg: &config.Config{MaxLog: 3},
	}
	for i := range 5 {
		hub.appendLog(string(rune('a' + i)))
	}
	entries := hub.LogEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (max), got %d", len(entries))
	}
	// Should keep the newest 3: "e", "d", "c"
	if entries[0] != "e" {
		t.Errorf("entries[0] = %q, want 'e'", entries[0])
	}
}

func TestHub_Subscribe_Unsubscribe(t *testing.T) {
	hub := &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
	ch := hub.Subscribe()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	hub.mu.RLock()
	count := len(hub.subscribers)
	hub.mu.RUnlock()
	if count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	hub.Unsubscribe(ch)
	hub.mu.RLock()
	count = len(hub.subscribers)
	hub.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", count)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
	ch := hub.Subscribe()

	hub.broadcast(Event{Kind: "log", Data: "test message"})

	select {
	case evt := <-ch:
		if evt.Kind != "log" {
			t.Errorf("event kind = %q, want 'log'", evt.Kind)
		}
		if evt.Data != "test message" {
			t.Errorf("event data = %q, want 'test message'", evt.Data)
		}
	default:
		t.Error("expected to receive broadcast event")
	}
}

func TestHub_Broadcast_NoSubscribers(t *testing.T) {
	hub := &Hub{
		subscribers: make(map[chan Event]struct{}),
	}
	// Should not panic with no subscribers.
	hub.broadcast(Event{Kind: "log", Data: "test"})
}

// ─── Template data tests ───────────────────────────────────────────────────

func TestTemplateData_Connected(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		subs:      defaultSubs(),
		slices:    radio.NewSliceCollector(),
		connected: true,
		status:    "Connected  handle=0xABCD  version=3.0.0",
		logBuf:    []string{"entry"},
		addr:      "192.168.1.1:4992",
	}

	data := hub.templateData()
	if !data.Connected {
		t.Error("expected Connected=true")
	}
	if data.Status != "Connected  handle=0xABCD  version=3.0.0" {
		t.Errorf("Status = %q", data.Status)
	}
	if len(data.LogEntries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(data.LogEntries))
	}
	if len(data.Subs) != len(defaultSubs()) {
		t.Errorf("expected %d subs, got %d", len(defaultSubs()), len(data.Subs))
	}
}

func TestTemplateData_Disconnected(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		subs:      defaultSubs(),
		slices:    radio.NewSliceCollector(),
		connected: false,
		errMsg:    "connection refused",
		addr:      "192.168.1.1:4992",
	}

	data := hub.templateData()
	if data.Connected {
		t.Error("expected Connected=false")
	}
	if data.ErrMsg != "connection refused" {
		t.Errorf("ErrMsg = %q, want 'connection refused'", data.ErrMsg)
	}
}

// ─── Hub helper tests ──────────────────────────────────────────────────────

func TestHub_ConnInfo(t *testing.T) {
	hub := &Hub{conn: &radio.Conn{Handle: 0xABCD, Version: "3.0.0"}}
	handle, version := hub.ConnInfo()
	if handle != 0xABCD {
		t.Errorf("handle = 0x%X, want 0xABCD", handle)
	}
	if version != "3.0.0" {
		t.Errorf("version = %q, want '3.0.0'", version)
	}
}

func TestHub_ConnInfo_Nil(t *testing.T) {
	hub := &Hub{}
	handle, version := hub.ConnInfo()
	if handle != 0 {
		t.Errorf("handle = 0x%X, want 0", handle)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestHub_UpsertRadio(t *testing.T) {
	r1 := radio.DiscoveredRadio{Serial: "S1", Model: "6600"}
	r2 := radio.DiscoveredRadio{Serial: "S2", Model: "6400"}

	list := upsertRadio(nil, r1)
	if len(list) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(list))
	}

	list = upsertRadio(list, r2)
	if len(list) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(list))
	}

	updated := radio.DiscoveredRadio{Serial: "S1", Model: "6700"}
	list = upsertRadio(list, updated)
	if len(list) != 2 {
		t.Fatalf("expected 2 radios after upsert, got %d", len(list))
	}
	if list[0].Model != "6700" {
		t.Errorf("expected updated model '6700', got %q", list[0].Model)
	}
}

func TestHub_RemoveRadio(t *testing.T) {
	r1 := radio.DiscoveredRadio{Serial: "S1"}
	r2 := radio.DiscoveredRadio{Serial: "S2"}
	list := []radio.DiscoveredRadio{r1, r2}

	list = removeRadio(list, "S1")
	if len(list) != 1 {
		t.Fatalf("expected 1 radio after removal, got %d", len(list))
	}
	if list[0].Serial != "S2" {
		t.Errorf("expected serial 'S2', got %q", list[0].Serial)
	}

	list = removeRadio(list, "missing")
	if len(list) != 1 {
		t.Fatalf("expected 1 radio after no-op removal, got %d", len(list))
	}
}

func TestHub_DoDisconnect(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		connected: true,
		conn:      &radio.Conn{},
		status:    "Connected",
	}
	hub.doDisconnect()
	if hub.IsConnected() {
		t.Error("expected disconnected")
	}
	if hub.Status() != "Disconnected" {
		t.Errorf("status = %q, want 'Disconnected'", hub.Status())
	}
}

// ─── processCommand tests ──────────────────────────────────────────────────

func TestHub_ProcessCommand_Connect(t *testing.T) {
	hub := &Hub{
		cfg:      &config.Config{MaxLog: 100},
		addr:     "192.168.1.1:4992",
		dialFunc: func(string) (radio.RadioConn, error) { return nil, fmt.Errorf("mock dial fail") },
	}
	hub.processCommand(Command{Kind: "connect", Addr: "192.168.1.1:4992"})
	// Should set dialing then fail; status should reflect error.
	if hub.IsDialing() {
		t.Error("expected dialing to be false after failed connect")
	}
}

func TestHub_ProcessCommand_Disconnect(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		connected: true,
		conn:      &radio.Conn{},
	}
	hub.processCommand(Command{Kind: "disconnect"})
	if hub.IsConnected() {
		t.Error("expected disconnected after processCommand")
	}
}

func TestHub_ProcessCommand_Subscribe(t *testing.T) {
	hub := &Hub{
		cfg:  &config.Config{MaxLog: 100},
		subs: defaultSubs(),
	}
	// Toggle slice from true to false.
	hub.processCommand(Command{Kind: "subscribe", Name: "slice", Val: "false"})
	for _, s := range hub.Subs() {
		if s.Name == "slice" && s.Checked {
			t.Error("expected slice to be unchecked")
		}
	}
}

// ─── Discovery injection tests ─────────────────────────────────────────────

func TestHub_StartDiscovery_Mock(t *testing.T) {
	ch := make(chan radio.DiscoveryEvent, 2)
	ch <- radio.DiscoveryEvent{Radio: radio.DiscoveredRadio{Serial: "S1", Model: "6600"}}
	close(ch)

	hub := &Hub{
		cfg:           &config.Config{MaxLog: 100},
		subscribers:   make(map[chan Event]struct{}),
		discoveryFunc: func(context.Context) (<-chan radio.DiscoveryEvent, error) { return ch, nil },
	}

	hub.startDiscovery()
	time.Sleep(50 * time.Millisecond)

	if !hub.IsDiscovering() {
		t.Error("expected discovering to be true")
	}
	if len(hub.Radios()) != 1 {
		t.Errorf("expected 1 radio, got %d", len(hub.Radios()))
	}
}

func TestHub_StopDiscovery(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		discovering: true,
	}
	hub.stopDiscovery()
	if hub.IsDiscovering() {
		t.Error("expected discovering to be false after stop")
	}
}

// ─── Table-driven read accessor tests ──────────────────────────────────────

func TestHub_ReadAccessors(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		radios:      []radio.DiscoveredRadio{{Serial: "S1", Model: "6600"}},
		discovering: true,
		connected:   true,
		dialing:     true,
		status:      "Connected",
		errMsg:      "some error",
		subs:        []subscription{{Name: "slice", Checked: true}},
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Radios", len(hub.Radios()), 1},
		{"IsDiscovering", hub.IsDiscovering(), true},
		{"IsConnected", hub.IsConnected(), true},
		{"IsDialing", hub.IsDialing(), true},
		{"Status", hub.Status(), "Connected"},
		{"ErrMsg", hub.ErrMsg(), "some error"},
		{"Subs", len(hub.Subs()), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

// ─── RadioConn interface tests ─────────────────────────────────────────────

type mockRadioConn struct {
	sendCalled    bool
	sendCmd       string
	enableRecon   bool
	disableRecon  bool
	closeCalled   bool
	handle        uint32
	version       string
	state         radio.ConnectionState
	OnLog         func(string, string)
	OnStateChange func(radio.ConnectionState, radio.ConnectionState)
	OnPingRtt     func(int)
}

func (m *mockRadioConn) Send(cmd string, cb func(int, string)) (uint32, error) {
	m.sendCalled = true
	m.sendCmd = cmd
	return 1, nil
}

func (m *mockRadioConn) EnableReconnect()                         { m.enableRecon = true }
func (m *mockRadioConn) DisableReconnect()                        { m.disableRecon = true }
func (m *mockRadioConn) Close()                                   { m.closeCalled = true }
func (m *mockRadioConn) ReadLoop(func(radio.ParsedMessage)) error { return nil }
func (m *mockRadioConn) ReconnectDone() <-chan struct{}           { return nil }
func (m *mockRadioConn) State() radio.ConnectionState             { return m.state }
func (m *mockRadioConn) GetHandle() uint32                        { return m.handle }
func (m *mockRadioConn) GetVersion() string                       { return m.version }
func (m *mockRadioConn) SetOnLog(fn func(string, string))         { m.OnLog = fn }
func (m *mockRadioConn) SetOnStateChange(fn func(radio.ConnectionState, radio.ConnectionState)) {
	m.OnStateChange = fn
}
func (m *mockRadioConn) SetOnPingRtt(fn func(int)) { m.OnPingRtt = fn }

func (m *mockRadioConn) OnDisconnected() {}

func TestHub_DoSubscribe_WithMockConn(t *testing.T) {
	mock := &mockRadioConn{handle: 0x1234, version: "3.0.0"}
	hub := &Hub{
		cfg:  &config.Config{MaxLog: 100},
		subs: defaultSubs(),
		conn: mock,
	}

	hub.doSubscribe("slice", false)

	if !mock.sendCalled {
		t.Error("expected Send to be called on mock conn")
	}
	if mock.sendCmd != "unsub slice all" {
		t.Errorf("expected 'unsub slice all', got %q", mock.sendCmd)
	}

	for _, s := range hub.Subs() {
		if s.Name == "slice" && s.Checked {
			t.Error("expected slice to be unchecked")
		}
	}
}

func TestHub_DoTune_WithMockConn(t *testing.T) {
	mock := &mockRadioConn{}
	hub := &Hub{
		cfg:  &config.Config{MaxLog: 100},
		conn: mock,
	}

	hub.doTune("14.300")

	if !mock.sendCalled {
		t.Error("expected Send to be called on mock conn")
	}
	if mock.sendCmd != "slice tune 0 14.300 autopan=0" {
		t.Errorf("expected tune command, got %q", mock.sendCmd)
	}
}

func TestHub_DoRawCommand_WithMockConn(t *testing.T) {
	mock := &mockRadioConn{}
	hub := &Hub{
		cfg:  &config.Config{MaxLog: 100},
		conn: mock,
	}

	hub.doRawCommand("sub pan all")

	if !mock.sendCalled {
		t.Error("expected Send to be called on mock conn")
	}
	if mock.sendCmd != "sub pan all" {
		t.Errorf("expected 'sub pan all', got %q", mock.sendCmd)
	}
}

func TestHub_DoDisconnect_WithMockConn(t *testing.T) {
	mock := &mockRadioConn{}
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		connected: true,
		conn:      mock,
	}

	hub.doDisconnect()

	if !mock.disableRecon {
		t.Error("expected DisableReconnect to be called")
	}
	if !mock.closeCalled {
		t.Error("expected Close to be called")
	}
	if hub.IsConnected() {
		t.Error("expected disconnected")
	}
}

// ─── wireCallbacks test ────────────────────────────────────────────────────

func TestHub_WireCallbacks(t *testing.T) {
	mock := &mockRadioConn{}
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subscribers: make(map[chan Event]struct{}),
	}

	hub.wireCallbacks(mock)

	// Trigger OnLog callback.
	mock.OnLog("tx", "test command")
	entries := hub.LogEntries()
	if len(entries) != 1 || entries[0] != "→ test command" {
		t.Errorf("expected log entry from OnLog callback, got %v", entries)
	}

	// Trigger OnStateChange callback.
	mock.OnStateChange(radio.StateDisconnected, radio.StateConnected)
	entries = hub.LogEntries()
	if len(entries) != 2 { // OnLog + state change
		t.Errorf("expected 2 log entries after state change, got %d", len(entries))
	}

	// Trigger OnPingRtt callback.
	mock.OnPingRtt(42)
	entries = hub.LogEntries()
	if len(entries) != 3 {
		t.Errorf("expected 3 log entries after ping, got %d", len(entries))
	}
}

// ─── doConnect with mock RadioConn ─────────────────────────────────────────

func TestHub_DoConnect_WireCallbacksIntegration(t *testing.T) {
	mock := &mockRadioConn{handle: 0xABCD, version: "3.3.0"}
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
	}

	// Manually set the conn and call wireCallbacks as doConnect would.
	hub.mu.Lock()
	hub.conn = mock
	hub.connected = true
	hub.mu.Unlock()

	hub.wireCallbacks(mock)

	// Simulate callbacks firing.
	mock.OnLog("tx", "C1|sub slice all")
	mock.OnStateChange(radio.StateConnected, radio.StateDisconnected)
	mock.OnPingRtt(25)

	entries := hub.LogEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 log entries, got %d: %v", len(entries), entries)
	}
	if !strings.Contains(entries[2], "→ C1|sub slice all") {
		t.Errorf("expected OnLog entry, got %q", entries[2])
	}
	if !strings.Contains(entries[1], "[state]") {
		t.Errorf("expected state change entry, got %q", entries[1])
	}
	if !strings.Contains(entries[0], "[ping] RTT 25 ms") {
		t.Errorf("expected ping entry, got %q", entries[0])
	}
}

func TestHub_DoConnect_DialFailure(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		dialFunc:    func(string) (radio.RadioConn, error) { return nil, fmt.Errorf("connection refused") },
	}

	hub.doConnect("192.168.1.100:4992", nil)

	if hub.IsConnected() {
		t.Error("expected not connected after dial failure")
	}
	if hub.ErrMsg() == "" {
		t.Error("expected error message after dial failure")
	}
	if hub.Status() != "Disconnected" {
		t.Errorf("expected status 'Disconnected', got %q", hub.Status())
	}
}

func TestHub_DoConnect_AlreadyConnected(t *testing.T) {
	mock := &mockRadioConn{}
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		connected:   true,
		conn:        mock,
		subscribers: make(map[chan Event]struct{}),
		dialFunc:    func(string) (radio.RadioConn, error) { t.Fatal("dial should not be called"); return nil, nil },
	}

	hub.doConnect("192.168.1.100:4992", nil)

	if !hub.IsConnected() {
		t.Error("expected still connected")
	}
}

func TestHub_DoConnect_AlreadyDialing(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		dialing:     true,
		subscribers: make(map[chan Event]struct{}),
		dialFunc:    func(string) (radio.RadioConn, error) { t.Fatal("dial should not be called"); return nil, nil },
	}

	hub.doConnect("192.168.1.100:4992", nil)

	if !hub.IsDialing() {
		t.Error("expected still dialing (no change)")
	}
}

// ─── upsertRadio / removeRadio helper tests ────────────────────────────────

func TestUpsertRadio_AddsNew(t *testing.T) {
	radios := []radio.DiscoveredRadio{{Serial: "S1", Model: "6600"}}
	newRadio := radio.DiscoveredRadio{Serial: "S2", Model: "6400"}
	got := upsertRadio(radios, newRadio)
	if len(got) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(got))
	}
	if got[1].Serial != "S2" {
		t.Errorf("expected S2, got %s", got[1].Serial)
	}
}

func TestUpsertRadio_UpdatesExisting(t *testing.T) {
	radios := []radio.DiscoveredRadio{{Serial: "S1", Model: "6600", Status: "Available"}}
	updated := radio.DiscoveredRadio{Serial: "S1", Model: "6600", Status: "In_Use"}
	got := upsertRadio(radios, updated)
	if len(got) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(got))
	}
	if got[0].Status != "In_Use" {
		t.Errorf("expected status In_Use, got %s", got[0].Status)
	}
}

func TestUpsertRadio_Empty(t *testing.T) {
	newRadio := radio.DiscoveredRadio{Serial: "S1", Model: "6600"}
	got := upsertRadio(nil, newRadio)
	if len(got) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(got))
	}
}

func TestRemoveRadio_Removes(t *testing.T) {
	radios := []radio.DiscoveredRadio{{Serial: "S1"}, {Serial: "S2"}}
	got := removeRadio(radios, "S1")
	if len(got) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(got))
	}
	if got[0].Serial != "S2" {
		t.Errorf("expected S2, got %s", got[0].Serial)
	}
}

func TestRemoveRadio_NotFound(t *testing.T) {
	radios := []radio.DiscoveredRadio{{Serial: "S1"}}
	got := removeRadio(radios, "S2")
	if len(got) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(got))
	}
}

func TestRemoveRadio_Empty(t *testing.T) {
	got := removeRadio(nil, "S1")
	if len(got) != 0 {
		t.Fatalf("expected 0 radios, got %d", len(got))
	}
}

// ─── Hub lifecycle tests ───────────────────────────────────────────────────

func TestHub_Close_Idempotent(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		discoveryFunc: func(context.Context) (<-chan radio.DiscoveryEvent, error) {
			return make(chan radio.DiscoveryEvent), nil
		},
	}
	// Manually start loop without discovery to avoid UDP bind.
	go hub.loop()

	hub.Close()
	// Second close should not panic.
	hub.Close()
}

func TestHub_SendCommand_AfterClose(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		discoveryFunc: func(context.Context) (<-chan radio.DiscoveryEvent, error) {
			return make(chan radio.DiscoveryEvent), nil
		},
	}
	go hub.loop()
	hub.Close()

	// SendCommand after Close should not panic and should return immediately.
	done := make(chan struct{})
	go func() {
		defer close(done)
		hub.SendCommand(Command{Kind: "connect"})
	}()

	select {
	case <-done:
		// Expected — SendCommand returns safely after dropping the command.
	case <-time.After(2 * time.Second):
		t.Fatal("SendCommand should return immediately after Close")
	}
}

func TestHub_Close_WaitsForLoop(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		dialFunc:    func(string) (radio.RadioConn, error) { return nil, fmt.Errorf("mock dial") },
	}
	go hub.loop()

	// Queue a command that will be processed.
	hub.SendCommand(Command{Kind: "connect", Addr: "1.2.3.4:4992"})

	// Give loop time to process.
	time.Sleep(50 * time.Millisecond)

	// Close should shut down cleanly.
	hub.Close()

	// Verify state after close.
	if hub.IsConnected() {
		t.Error("expected not connected after close")
	}
}
