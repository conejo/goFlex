package app

import (
	"strings"
	"testing"
)

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			width:    10,
			expected: nil,
		},
		{
			name:     "shorter than width",
			input:    "hello",
			width:    10,
			expected: []string{"hello"},
		},
		{
			name:     "exactly width",
			input:    "helloworld",
			width:    10,
			expected: []string{"helloworld"},
		},
		{
			name:     "breaks at space",
			input:    "hello world",
			width:    8,
			expected: []string{"hello", "world"},
		},
		{
			name:     "hard break when no space",
			input:    "helloworld",
			width:    5,
			expected: []string{"hello", "world"},
		},
		{
			name:     "multiple breaks",
			input:    "this is a test of word wrapping",
			width:    10,
			expected: []string{"this is a", "test of", "word", "wrapping"},
		},
		{
			name:     "multiple spaces between words",
			input:    "hello  world",
			width:    8,
			expected: []string{"hello ", "world"},
		},
		{
			name:     "width of 1",
			input:    "abc",
			width:    1,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "unicode characters",
			input:    "こんにちは世界",
			width:    5,
			expected: []string{"こんにちは", "世界"},
		},
		{
			name:     "breaks at last space within width",
			input:    "one two three four",
			width:    12,
			expected: []string{"one two", "three four"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordWrap(tt.input, tt.width)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestWrapLogEntries(t *testing.T) {
	tests := []struct {
		name      string
		entries   []string
		wrapWidth int
		check     func(t *testing.T, got []string)
	}{
		{
			name:      "empty slice",
			entries:   []string{},
			wrapWidth: 20,
			check: func(t *testing.T, got []string) {
				if len(got) != 0 {
					t.Errorf("expected empty, got %v", got)
				}
			},
		},
		{
			name:      "single short entry",
			entries:   []string{"short log"},
			wrapWidth: 20,
			check: func(t *testing.T, got []string) {
				if len(got) != 1 {
					t.Fatalf("expected 1 line, got %d", len(got))
				}
				if !strings.Contains(got[0], "short log") {
					t.Errorf("expected to contain 'short log', got %q", got[0])
				}
			},
		},
		{
			name:      "entry wraps and continues",
			entries:   []string{"this is a very long log entry that should definitely wrap"},
			wrapWidth: 20,
			check: func(t *testing.T, got []string) {
				if len(got) != 1 {
					t.Fatalf("expected 1 wrapped entry, got %d", len(got))
				}
				lines := strings.Split(got[0], "\n")
				if len(lines) < 2 {
					t.Errorf("expected multiple lines after wrapping, got %d", len(lines))
				}
				// First line should contain start of text
				if !strings.Contains(lines[0], "this is a very") {
					t.Errorf("first line unexpected: %q", lines[0])
				}
				// Continuation lines should have the prefix
				for i := 1; i < len(lines); i++ {
					if !strings.Contains(lines[i], "↳") {
						t.Errorf("continuation line %d missing prefix: %q", i, lines[i])
					}
				}
			},
		},
		{
			name:      "width below minimum uses 20",
			entries:   []string{"some log entry here"},
			wrapWidth: 5,
			check: func(t *testing.T, got []string) {
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				// With width clamped to 20, this short entry should not wrap
				lines := strings.Split(got[0], "\n")
				if len(lines) != 1 {
					t.Errorf("expected no wrapping with clamped width, got %d lines", len(lines))
				}
			},
		},
		{
			name:      "multiple entries",
			entries:   []string{"first entry", "second entry that is longer and should wrap across multiple lines"},
			wrapWidth: 20,
			check: func(t *testing.T, got []string) {
				if len(got) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(got))
				}
				// First entry should be short
				if strings.Contains(got[0], "\n") {
					t.Errorf("first entry should not wrap: %q", got[0])
				}
				// Second entry should wrap
				if !strings.Contains(got[1], "\n") {
					t.Errorf("second entry should wrap: %q", got[1])
				}
			},
		},
		{
			name:      "empty string entry",
			entries:   []string{""},
			wrapWidth: 20,
			check: func(t *testing.T, got []string) {
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				if got[0] != "" {
					t.Errorf("expected empty string, got %q", got[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLogEntries(tt.entries, tt.wrapWidth)
			tt.check(t, got)
		})
	}
}
