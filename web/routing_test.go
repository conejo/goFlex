package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goFlex/config"
	"goFlex/radio"
)

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
