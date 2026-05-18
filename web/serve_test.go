package web

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"goFlex/config"
	"goFlex/radio"
)

// ─── Integration test via ServeWithListener ────────────────────────────────

func TestServeWithListener_EndToEnd(t *testing.T) {
	hub := &Hub{
		cfg:         &config.Config{MaxLog: 100},
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		dialFunc:    func(string) (*radio.Conn, error) { return nil, fmt.Errorf("mock dial") },
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- ServeWithListener(hub, ln)
	}()

	// Give the server a moment to start.
	time.Sleep(50 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", ln.Addr().String())

	// Test GET / returns 200.
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET / = %d, want 200", resp.StatusCode)
	}

	// Test GET /log returns 200.
	resp, err = http.Get(baseURL + "/log")
	if err != nil {
		t.Fatalf("GET /log failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /log = %d, want 200", resp.StatusCode)
	}

	// Test GET /status returns 200.
	resp, err = http.Get(baseURL + "/status")
	if err != nil {
		t.Fatalf("GET /status failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /status = %d, want 200", resp.StatusCode)
	}

	// Test GET /events returns 200 with SSE headers.
	resp, err = http.Get(baseURL + "/events")
	if err != nil {
		t.Fatalf("GET /events failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /events = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Test POST /connect returns 303 redirect.
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = noRedirectClient.Post(baseURL+"/connect", "application/x-www-form-urlencoded", strings.NewReader("addr=1.2.3.4:4992"))
	if err != nil {
		t.Fatalf("POST /connect failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 303 {
		t.Errorf("POST /connect = %d, want 303", resp.StatusCode)
	}

	// Test POST /disconnect returns 303 redirect.
	resp, err = noRedirectClient.Post(baseURL+"/disconnect", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("POST /disconnect failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 303 {
		t.Errorf("POST /disconnect = %d, want 303", resp.StatusCode)
	}

	// Test POST /subscribe returns 200.
	resp, err = http.Post(baseURL+"/subscribe", "application/x-www-form-urlencoded", strings.NewReader("name=slice&checked=true"))
	if err != nil {
		t.Fatalf("POST /subscribe failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("POST /subscribe = %d, want 200", resp.StatusCode)
	}

	// Test POST /tune with empty freq returns 400.
	resp, err = http.Post(baseURL+"/tune", "application/x-www-form-urlencoded", strings.NewReader("freq="))
	if err != nil {
		t.Fatalf("POST /tune failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("POST /tune empty = %d, want 400", resp.StatusCode)
	}

	// Test POST /command with empty cmd returns 400.
	resp, err = http.Post(baseURL+"/command", "application/x-www-form-urlencoded", strings.NewReader("cmd="))
	if err != nil {
		t.Fatalf("POST /command failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("POST /command empty = %d, want 400", resp.StatusCode)
	}

	// Test static asset route returns 200 (htmx.js is embedded).
	resp, err = http.Get(baseURL + "/static/htmx.min.js")
	if err != nil {
		t.Fatalf("GET /static/htmx.min.js failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /static/htmx.min.js = %d, want 200", resp.StatusCode)
	}

	// Shut down the server.
	ln.Close()

	select {
	case err := <-serverErr:
		// Accept either graceful shutdown or listener-closed.
		if err != nil && err != http.ErrServerClosed && !strings.Contains(err.Error(), "use of closed network connection") {
			t.Fatalf("server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
