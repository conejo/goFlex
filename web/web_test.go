package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"goFlex/config"
	"goFlex/radio"
)

func TestRenderToString_LogTemplate(t *testing.T) {
	data := templateData{
		LogEntries: []string{"line one", "line two"},
	}
	html := renderToString("log.html", data)
	if !strings.Contains(html, "line one") {
		t.Errorf("expected 'line one' in output, got: %s", html)
	}
	if !strings.Contains(html, "line two") {
		t.Errorf("expected 'line two' in output, got: %s", html)
	}
	if !strings.Contains(html, "log-line") {
		t.Errorf("expected 'log-line' class in output, got: %s", html)
	}
}

func TestRenderToString_LogTemplate_Empty(t *testing.T) {
	data := templateData{}
	html := renderToString("log.html", data)
	if !strings.Contains(html, "No log entries yet") {
		t.Errorf("expected 'No log entries yet' for empty log, got: %s", html)
	}
}

func TestRenderToString_StatusTemplate(t *testing.T) {
	data := templateData{
		Connected: true,
		Status:    "Connected  handle=0xABCD  version=3.0.0",
		Handle:    0xABCD,
		Version:   "3.0.0",
	}
	html := renderToString("status.html", data)
	if !strings.Contains(html, "Connected") {
		t.Errorf("expected 'Connected' in output, got: %s", html)
	}
	if !strings.Contains(html, "0xABCD") {
		t.Errorf("expected '0xABCD' in output, got: %s", html)
	}
	if !strings.Contains(html, "3.0.0") {
		t.Errorf("expected '3.0.0' in output, got: %s", html)
	}
}

func TestRenderToString_StatusTemplate_Error(t *testing.T) {
	data := templateData{
		Connected: false,
		ErrMsg:    "connection refused",
	}
	html := renderToString("status.html", data)
	if !strings.Contains(html, "connection refused") {
		t.Errorf("expected error message in output, got: %s", html)
	}
}

func TestRenderToString_SubsTemplate(t *testing.T) {
	data := templateData{
		Subs: []subscription{
			{Name: "slice", Label: "Slice", Checked: true},
			{Name: "pan", Label: "Panadapter", Checked: false},
		},
	}
	html := renderToString("subs.html", data)
	if !strings.Contains(html, "Slice") {
		t.Errorf("expected 'Slice' in output, got: %s", html)
	}
	if !strings.Contains(html, "Panadapter") {
		t.Errorf("expected 'Panadapter' in output, got: %s", html)
	}
}

func TestRenderToString_SlicesTemplate_Empty(t *testing.T) {
	data := templateData{Slices: map[string]map[string]string{}}
	html := renderToString("slices.html", data)
	if !strings.Contains(html, "No slice data yet") {
		t.Errorf("expected 'No slice data yet' for empty slices, got: %s", html)
	}
}

func TestRenderToString_SlicesTemplate_WithData(t *testing.T) {
	data := templateData{
		Slices: map[string]map[string]string{
			"0": {"RF_frequency": "14.300", "mode": "USB"},
		},
	}
	html := renderToString("slices.html", data)
	if !strings.Contains(html, "Slice 0") {
		t.Errorf("expected 'Slice 0' in output, got: %s", html)
	}
	if !strings.Contains(html, "14.300") {
		t.Errorf("expected '14.300' in output, got: %s", html)
	}
	if !strings.Contains(html, "USB") {
		t.Errorf("expected 'USB' in output, got: %s", html)
	}
}

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

// ─── Handler tests ─────────────────────────────────────────────────────────

func TestHandleIndex_Disconnected(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		subs:      defaultSubs(),
		slices:    radio.NewSliceCollector(),
		connected: false,
		addr:      "192.168.1.1:4992",
	}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	hub.handleIndex(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Scanning for radios") {
		t.Errorf("expected discovery page, got: %s", body[:200])
	}
}

func TestHandleIndex_Connected(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		subs:      defaultSubs(),
		slices:    radio.NewSliceCollector(),
		connected: true,
		status:    "Connected  handle=0xABCD  version=3.0.0",
		addr:      "192.168.1.1:4992",
	}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	hub.handleIndex(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "sse-connect") {
		t.Errorf("expected connected page with sse-connect, got: %s", body[:200])
	}
}

func TestHandleLog(t *testing.T) {
	hub := &Hub{
		cfg:    &config.Config{MaxLog: 100},
		slices: radio.NewSliceCollector(),
		logBuf: []string{"test log"},
	}

	req := httptest.NewRequest("GET", "/log", nil)
	rec := httptest.NewRecorder()
	hub.handleLog(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test log") {
		t.Errorf("expected 'test log' in response, got: %s", body)
	}
}

func TestHandleStatus(t *testing.T) {
	hub := &Hub{
		cfg:       &config.Config{MaxLog: 100},
		slices:    radio.NewSliceCollector(),
		connected: true,
		status:    "Connected  handle=0xABCD  version=3.0.0",
	}

	req := httptest.NewRequest("GET", "/status", nil)
	rec := httptest.NewRecorder()
	hub.handleStatus(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Connected") {
		t.Errorf("expected 'Connected' in response, got: %s", body)
	}
}

func TestHandleConnect_Redirects(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		addr:        "192.168.1.1:4992",
		dialFunc:    func(string) (*radio.Conn, error) { return nil, fmt.Errorf("mock dial fail") },
	}

	body := strings.NewReader("addr=192.168.1.1:4992&subs=slice&subs=pan")
	req := httptest.NewRequest("POST", "/connect", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	hub.handleConnect(rec, req)

	if rec.Code != 303 {
		t.Errorf("expected 303 redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestHandleDisconnect_Redirects(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		connected:   true,
	}

	req := httptest.NewRequest("POST", "/disconnect", nil)
	rec := httptest.NewRecorder()
	hub.handleDisconnect(rec, req)

	if rec.Code != 303 {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
}

func TestHandleSubscribe(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
	}

	body := strings.NewReader("name=slice&checked=true")
	req := httptest.NewRequest("POST", "/subscribe", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleSubscribe(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "Slice") {
		t.Errorf("expected subs panel with 'Slice', got: %s", bodyStr)
	}
}

func TestHandleTune(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		logBuf:      []string{"existing log"},
	}

	body := strings.NewReader("freq=14.300")
	req := httptest.NewRequest("POST", "/tune", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleTune(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "existing log") {
		t.Errorf("expected log pane with 'existing log', got: %s", bodyStr)
	}
}

func TestHandleCommand(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		logBuf:      []string{"existing log"},
	}

	body := strings.NewReader("cmd=sub+slice+all")
	req := httptest.NewRequest("POST", "/command", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleCommand(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "existing log") {
		t.Errorf("expected log pane with 'existing log', got: %s", bodyStr)
	}
}

func TestHandleTune_EmptyFreq(t *testing.T) {
	hub := &Hub{}
	body := strings.NewReader("freq=")
	req := httptest.NewRequest("POST", "/tune", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleTune(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400 for empty freq, got %d", rec.Code)
	}
}

func TestHandleCommand_EmptyCmd(t *testing.T) {
	hub := &Hub{}
	body := strings.NewReader("cmd=")
	req := httptest.NewRequest("POST", "/command", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleCommand(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400 for empty cmd, got %d", rec.Code)
	}
}

// ─── render() tests ────────────────────────────────────────────────────────

func TestRender_LogTemplate(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "log.html", templateData{LogEntries: []string{"test line"}})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test line") {
		t.Errorf("expected 'test line' in output, got: %s", rec.Body.String())
	}
}

// ─── Fragment handler tests ────────────────────────────────────────────────

func TestHandleDiscovery(t *testing.T) {
	hub := &Hub{
		cfg:    &config.Config{MaxLog: 100},
		subs:   defaultSubs(),
		slices: radio.NewSliceCollector(),
	}
	req := httptest.NewRequest("GET", "/discovery", nil)
	rec := httptest.NewRecorder()
	hub.handleDiscovery(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleSlices(t *testing.T) {
	hub := &Hub{
		cfg:    &config.Config{MaxLog: 100},
		slices: radio.NewSliceCollector(),
	}
	req := httptest.NewRequest("GET", "/slices", nil)
	rec := httptest.NewRecorder()
	hub.handleSlices(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No slice data yet") {
		t.Errorf("expected empty slices message, got: %s", rec.Body.String())
	}
}

func TestHandleSubsPanel(t *testing.T) {
	hub := &Hub{
		cfg:    &config.Config{MaxLog: 100},
		subs:   defaultSubs(),
		slices: radio.NewSliceCollector(),
	}
	req := httptest.NewRequest("GET", "/subs", nil)
	rec := httptest.NewRecorder()
	hub.handleSubsPanel(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Slice") {
		t.Errorf("expected subs panel with 'Slice', got: %s", rec.Body.String())
	}
}

// ─── SSE wire format tests (real method) ───────────────────────────────────

// flusherRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flusherRecorder) Flush() {}

func TestHub_WriteSSEFragment(t *testing.T) {
	hub := &Hub{}
	rec := &flusherRecorder{httptest.NewRecorder()}
	hub.writeSSEFragment(rec, rec, "log", "<div class=\"log-line\">test</div>")

	out := rec.Body.String()
	if !strings.HasPrefix(out, "event: log\n") {
		t.Errorf("SSE output must start with 'event: log\\n', got: %q", out)
	}
	if !strings.Contains(out, "data: ") {
		t.Errorf("SSE output must contain 'data:' lines, got: %q", out)
	}
	if !strings.HasSuffix(out, "\n\n") {
		t.Errorf("SSE output must end with '\\n\\n', got: %q", out)
	}
}

func TestHub_WriteSSEFragment_EmptyHTML(t *testing.T) {
	hub := &Hub{}
	rec := &flusherRecorder{httptest.NewRecorder()}
	hub.writeSSEFragment(rec, rec, "log", "")

	out := rec.Body.String()
	if !strings.Contains(out, "event: log") {
		t.Errorf("expected event line, got: %q", out)
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
		dialFunc: func(string) (*radio.Conn, error) { return nil, fmt.Errorf("mock dial fail") },
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

// ─── NewMux routing tests ─────────────────────────────────────────────────

func TestNewMux_Routes(t *testing.T) {
	hub := &Hub{
		cfg:    &config.Config{MaxLog: 100},
		subs:   defaultSubs(),
		slices: radio.NewSliceCollector(),
	}
	mux := NewMux(hub)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET / = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/log")
	if err != nil {
		t.Fatalf("GET /log failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /log = %d, want 200", resp.StatusCode)
	}
}

// ─── SSE endpoint tests ────────────────────────────────────────────────────

// safeBuffer wraps bytes.Buffer with a mutex for thread-safe access.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

// safeRecorder is a thread-safe ResponseRecorder for testing SSE handlers.
type safeRecorder struct {
	Code      int
	HeaderMap http.Header
	Body      *safeBuffer
	Flushed   bool
}

func newSafeRecorder() *safeRecorder {
	return &safeRecorder{
		Code:      200,
		HeaderMap: make(http.Header),
		Body:      &safeBuffer{},
	}
}

func (r *safeRecorder) Header() http.Header { return r.HeaderMap }

func (r *safeRecorder) Write(p []byte) (int, error) {
	return r.Body.Write(p)
}

func (r *safeRecorder) WriteHeader(code int) { r.Code = code }

func (r *safeRecorder) Flush() { r.Flushed = true }

func TestHandleEvents_InitialState(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		connected:   true,
		status:      "Connected  handle=0xABCD  version=3.0.0",
	}

	rec := newSafeRecorder()
	req := httptest.NewRequest("GET", "/events", nil)

	// Run handler in goroutine because it blocks.
	go hub.handleEvents(rec, req)

	// Wait a bit for initial fragments to be written.
	time.Sleep(50 * time.Millisecond)

	out := rec.Body.String()
	if !strings.Contains(out, "event: status") {
		t.Errorf("expected initial status event, got: %q", out[:200])
	}
	if !strings.Contains(out, "event: log") {
		t.Errorf("expected initial log event, got: %q", out[:200])
	}
	if !strings.Contains(out, "event: subs") {
		t.Errorf("expected initial subs event, got: %q", out[:200])
	}
	if !strings.Contains(out, "event: slices") {
		t.Errorf("expected initial slices event, got: %q", out[:200])
	}
}

func TestHandleEvents_Broadcast(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		connected:   true,
		status:      "Connected",
		logBuf:      []string{"test log"},
	}

	rec := newSafeRecorder()
	req := httptest.NewRequest("GET", "/events", nil)

	go hub.handleEvents(rec, req)
	time.Sleep(50 * time.Millisecond)

	// Clear initial output.
	rec.Body.Reset()

	// Broadcast a log event.
	hub.broadcast(Event{Kind: "log", Data: "new line"})
	time.Sleep(50 * time.Millisecond)

	out := rec.Body.String()
	if !strings.Contains(out, "event: log") {
		t.Errorf("expected log event after broadcast, got: %q", out[:200])
	}
	if !strings.Contains(out, "test log") {
		t.Errorf("expected rendered log content, got: %q", out[:200])
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

// ─── render() error path test ──────────────────────────────────────────────

func TestRender_MissingTemplate(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "nonexistent.html", templateData{})
	if rec.Code != 500 {
		t.Errorf("expected 500 for missing template, got %d", rec.Code)
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
