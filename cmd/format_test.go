package cmd

import (
	"strings"
	"testing"
)

func TestFormatLabel(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"callsign", "Callsign"},
		{"software_ver", "Software Ver"},
		{"chassis_serial", "Chassis Serial"},
		{"num_slice", "Num Slice"},
		{"RF_frequency", "Rf Frequency"},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatLabel(tt.input)
		if got != tt.want {
			t.Errorf("formatLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPrintSettings(t *testing.T) {
	settings := map[string]string{
		"callsign":        "K1ABC",
		"model":           "FLEX-6600",
		"software_ver":    "3.0.0",
		"unknown_key":     "some_value",
		"another_unknown": "42",
	}

	var sb strings.Builder
	printSettings(&sb, settings)
	out := sb.String()

	// Priority keys should use formatLabel.
	if !strings.Contains(out, "Callsign") {
		t.Error("expected formatted 'Callsign' label")
	}
	if !strings.Contains(out, "Model") {
		t.Error("expected formatted 'Model' label")
	}
	if !strings.Contains(out, "Software Ver") {
		t.Error("expected formatted 'Software Ver' label")
	}
	// All keys should use formatLabel now.
	if strings.Contains(out, "unknown_key") {
		t.Error("expected 'unknown_key' to be formatted, not raw")
	}
	if !strings.Contains(out, "Unknown Key") {
		t.Error("expected formatted 'Unknown Key' label")
	}
	if !strings.Contains(out, "Another Unknown") {
		t.Error("expected formatted 'Another Unknown' label")
	}
	// Values should be present.
	if !strings.Contains(out, "K1ABC") {
		t.Error("expected callsign value 'K1ABC'")
	}
	if !strings.Contains(out, "FLEX-6600") {
		t.Error("expected model value 'FLEX-6600'")
	}
}

func TestPrintSettings_Empty(t *testing.T) {
	var sb strings.Builder
	printSettings(&sb, map[string]string{})
	out := sb.String()
	if !strings.Contains(out, "Additional Settings") {
		t.Error("expected 'Additional Settings' header even for empty settings")
	}
}

func TestPrintSlices(t *testing.T) {
	slices := map[string]map[string]string{
		"0": {
			"RF_frequency": "14.300",
			"mode":         "USB",
			"active":       "1",
			"custom_prop":  "hello",
		},
		"1": {
			"RF_frequency": "7.200",
			"mode":         "LSB",
		},
	}

	var sb strings.Builder
	printSlices(&sb, slices)
	out := sb.String()

	if !strings.Contains(out, "Slice 0") {
		t.Error("expected 'Slice 0' header")
	}
	if !strings.Contains(out, "Slice 1") {
		t.Error("expected 'Slice 1' header")
	}
	if !strings.Contains(out, "Rf Frequency") {
		t.Error("expected formatted 'Rf Frequency' label")
	}
	if !strings.Contains(out, "14.300") {
		t.Error("expected frequency value '14.300'")
	}
	if !strings.Contains(out, "USB") {
		t.Error("expected mode 'USB'")
	}
	if !strings.Contains(out, "Custom Prop") {
		t.Error("expected formatted non-priority key 'Custom Prop'")
	}
}

func TestPrintSlices_Empty(t *testing.T) {
	var sb strings.Builder
	printSlices(&sb, map[string]map[string]string{})
	out := sb.String()
	if out != "" {
		t.Errorf("expected empty output for no slices, got %q", out)
	}
}
