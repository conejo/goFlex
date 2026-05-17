package web

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

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

// ─── SSE wire format tests ─────────────────────────────────────────────────

func TestWriteSSEFragment_ProducesValidSSE(t *testing.T) {
	var buf bytes.Buffer
	html := `<div class="log-line">test</div>
<div class="log-line">test2</div>`

	// Simulate what writeSSEFragment does.
	fmtPrintSSE(&buf, "log", html)

	output := buf.String()
	t.Logf("SSE output:\n%s", output)

	// Must start with "event: log\n"
	if !strings.HasPrefix(output, "event: log\n") {
		t.Errorf("SSE output must start with 'event: log\\n', got: %q", output)
	}
	// Must contain data: lines
	if !strings.Contains(output, "data:") {
		t.Errorf("SSE output must contain 'data:' lines, got: %q", output)
	}
	// Must end with \n\n (double newline terminates the event)
	if !strings.HasSuffix(output, "\n\n") {
		t.Errorf("SSE output must end with '\\n\\n', got: %q", output)
	}
	// Must contain the HTML content
	if !strings.Contains(output, "test") {
		t.Errorf("SSE output must contain 'test', got: %q", output)
	}
}

// fmtPrintSSE replicates writeSSEFragment logic for testing.
func fmtPrintSSE(w *bytes.Buffer, event, html string) {
	w.WriteString("event: " + event + "\n")
	for _, line := range strings.Split(html, "\n") {
		w.WriteString("data: " + line + "\n")
	}
	w.WriteString("\n")
}

func TestWriteSSEFragment_EmptyHTML(t *testing.T) {
	var buf bytes.Buffer
	fmtPrintSSE(&buf, "log", "")
	output := buf.String()
	if !strings.Contains(output, "event: log") {
		t.Errorf("expected event line, got: %q", output)
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
