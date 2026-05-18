package web

import (
	"net/http/httptest"
	"strings"
	"testing"
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

func TestRender_MissingTemplate(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "nonexistent.html", templateData{})
	if rec.Code != 500 {
		t.Errorf("expected 500 for missing template, got %d", rec.Code)
	}
}
