package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"goFlex/config"
	"goFlex/radio"
)

// ─── SSE wire format tests (real method) ───────────────────────────────────

// flusherRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flusherRecorder) Flush() {}

func TestHub_WriteSSEFragment(t *testing.T) {
	hub := &Hub{}
	rec := &flusherRecorder{httptest.NewRecorder()}
	hub.writeSSEFragment(rec, rec, "log", "<div class=\"log-line\"\u003etest\u003c/div\u003e")

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

// ─── handleEvents error paths ──────────────────────────────────────────────

// nonFlusher is a ResponseWriter that does not implement http.Flusher.
type nonFlusher struct {
	http.ResponseWriter
}

func TestHandleEvents_NonFlusher(t *testing.T) {
	hub := &Hub{cfg: &config.Config{MaxLog: 100}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	hub.handleEvents(nonFlusher{rec}, req)
	if rec.Code != 500 {
		t.Errorf("expected 500 for non-flusher, got %d", rec.Code)
	}
}

func TestHandleEvents_ContextCancelled(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		connected:   true,
		status:      "Connected",
	}

	rec := newSafeRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)

	go hub.handleEvents(rec, req)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Handler should exit gracefully when context is cancelled.
	// We verify it didn't panic by reaching this point.
}
