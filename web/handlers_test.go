package web

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"goFlex/config"
	"goFlex/radio"
)

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
		dialFunc:    func(string) (radio.RadioConn, error) { return nil, fmt.Errorf("mock dial fail") },
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

// ─── ParseForm error handling tests ────────────────────────────────────────

func TestHandleConnect_ParseFormError(t *testing.T) {
	hub := &Hub{cfg: &config.Config{MaxLog: 100}}
	// Invalid percent-encoding triggers ParseForm error.
	req := httptest.NewRequest("POST", "/connect", strings.NewReader("addr=%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleConnect(rec, req)
	if rec.Code != 400 {
		t.Errorf("expected 400 for bad form, got %d", rec.Code)
	}
}

func TestHandleDisconnect_ParseFormError(t *testing.T) {
	hub := &Hub{cfg: &config.Config{MaxLog: 100}}
	req := httptest.NewRequest("POST", "/disconnect", strings.NewReader("bad=%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleDisconnect(rec, req)
	if rec.Code != 400 {
		t.Errorf("expected 400 for bad form, got %d", rec.Code)
	}
}

func TestHandleSubscribe_ParseFormError(t *testing.T) {
	hub := &Hub{cfg: &config.Config{MaxLog: 100}}
	req := httptest.NewRequest("POST", "/subscribe", strings.NewReader("name=%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	hub.handleSubscribe(rec, req)
	if rec.Code != 400 {
		t.Errorf("expected 400 for bad form, got %d", rec.Code)
	}
}
