// app_test.go — unit tests for pure functions in the app package.

package app

import (
	"strings"
	"testing"

	"goFlex/radio"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── wordWrap / wrapLogEntries ──────────────────────────────────────────────

func TestWordWrap_ShortLine(t *testing.T) {
	lines := wordWrap("hello", 10)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Fatalf("want [hello], got %v", lines)
	}
}

func TestWordWrap_ExactWidth(t *testing.T) {
	lines := wordWrap("hello world", 11)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("want [hello world], got %v", lines)
	}
}

func TestWordWrap_BreakOnSpace(t *testing.T) {
	lines := wordWrap("hello world", 8)
	want := []string{"hello", "world"}
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("want %v, got %v", want, lines)
	}
}

func TestWordWrap_HardBreak(t *testing.T) {
	lines := wordWrap("helloworld", 5)
	want := []string{"hello", "world"}
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("want %v, got %v", want, lines)
	}
}

func TestWordWrap_Empty(t *testing.T) {
	lines := wordWrap("", 10)
	if len(lines) != 0 {
		t.Fatalf("want empty, got %v", lines)
	}
}

func TestWrapLogEntries_SingleLine(t *testing.T) {
	entries := []string{"hello world"}
	wrapped := wrapLogEntries(entries, 20)
	if len(wrapped) != 1 || wrapped[0] != "hello world" {
		t.Fatalf("want [hello world], got %v", wrapped)
	}
}

func TestWrapLogEntries_WrapsAndIndents(t *testing.T) {
	entries := []string{"hello world foo bar baz qux something long"}
	wrapped := wrapLogEntries(entries, 20)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 wrapped entry, got %d", len(wrapped))
	}
	// Should contain continuation prefix because the line exceeds 20 chars
	if !strings.Contains(wrapped[0], contPrefix) {
		t.Fatalf("expected continuation prefix in wrapped output, got %q", wrapped[0])
	}
}

func TestWrapLogEntries_MinWidth(t *testing.T) {
	entries := []string{"hello"}
	wrapped := wrapLogEntries(entries, 5)
	if len(wrapped) != 1 || wrapped[0] != "hello" {
		t.Fatalf("want [hello], got %v", wrapped)
	}
}

// ─── newDefaultSubs ─────────────────────────────────────────────────────────

func TestNewDefaultSubs(t *testing.T) {
	subs := newDefaultSubs()
	if len(subs) == 0 {
		t.Fatal("expected non-empty subscription list")
	}

	// Verify core subs are checked by default.
	core := map[string]bool{"slice": true, "pan": true, "tx": true, "meter": true}
	for _, s := range subs {
		if want, ok := core[s.name]; ok {
			if s.checked != want {
				t.Fatalf("%s: checked want %v, got %v", s.name, want, s.checked)
			}
			delete(core, s.name)
		}
	}
	if len(core) != 0 {
		t.Fatalf("missing core subscriptions: %v", core)
	}
}

// ─── upsertRadio / removeRadio ──────────────────────────────────────────────

func TestUpsertRadio_AddsNew(t *testing.T) {
	radios := []radio.RadioInfo{}
	info := radio.RadioInfo{Serial: "1234", Model: "FLEX-6600"}
	result := upsertRadio(radios, info)
	if len(result) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(result))
	}
	if result[0].Serial != "1234" {
		t.Fatalf("serial want 1234, got %s", result[0].Serial)
	}
}

func TestUpsertRadio_UpdatesExisting(t *testing.T) {
	radios := []radio.RadioInfo{{Serial: "1234", Model: "FLEX-6600"}}
	info := radio.RadioInfo{Serial: "1234", Model: "FLEX-6700"}
	result := upsertRadio(radios, info)
	if len(result) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(result))
	}
	if result[0].Model != "FLEX-6700" {
		t.Fatalf("model want FLEX-6700, got %s", result[0].Model)
	}
}

func TestRemoveRadio(t *testing.T) {
	radios := []radio.RadioInfo{
		{Serial: "1111"},
		{Serial: "2222"},
		{Serial: "3333"},
	}
	result := removeRadio(radios, "2222")
	if len(result) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(result))
	}
	for _, r := range result {
		if r.Serial == "2222" {
			t.Fatal("removed radio still present")
		}
	}
}

func TestRemoveRadio_NotFound(t *testing.T) {
	radios := []radio.RadioInfo{{Serial: "1111"}}
	result := removeRadio(radios, "9999")
	if len(result) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(result))
	}
}

// ─── Model helpers ──────────────────────────────────────────────────────────

func TestModel_LogHeight(t *testing.T) {
	m := model{height: 30, width: 80, ui: uiState{subs: newDefaultSubs()}}
	// Not connected, showSubs true → subs panel visible.
	m.connState.connected = false
	m.ui.showSubs = true
	h := m.logHeight()
	want := m.height - 3 - (2 + len(m.ui.subs))
	if h != want {
		t.Fatalf("logHeight: want %d, got %d", want, h)
	}
}

func TestModel_LogHeight_Connected(t *testing.T) {
	m := model{height: 30, width: 80, ui: uiState{subs: newDefaultSubs()}}
	m.connState.connected = true
	m.ui.showSubs = false
	h := m.logHeight()
	want := m.height - 3 // no subs panel
	if h != want {
		t.Fatalf("logHeight: want %d, got %d", want, h)
	}
}

func TestModel_WithScrollUp(t *testing.T) {
	// Create logs long enough to require scrolling.
	entries := make([]string, 50)
	for i := range entries {
		entries[i] = "this is a moderately long log line that will wrap or at least consume height"
	}
	m := model{height: 10, width: 40, connState: connectionState{connected: true}, ui: uiState{showSubs: false}, log: logState{entries: entries}}
	m.ui.scrollOffset = 0
	m = m.withScrollUp()
	if m.ui.scrollOffset != 1 {
		t.Fatalf("scrollOffset: want 1, got %d", m.ui.scrollOffset)
	}
}

func TestModel_WithScrollDown(t *testing.T) {
	m := model{height: 30, width: 80, connState: connectionState{connected: true}, ui: uiState{showSubs: false, scrollOffset: 2}}
	m = m.withScrollDown()
	if m.ui.scrollOffset != 1 {
		t.Fatalf("scrollOffset: want 1, got %d", m.ui.scrollOffset)
	}
}

func TestModel_MaxScrollOffset(t *testing.T) {
	m := model{height: 10, width: 40, log: logState{entries: []string{"hello world this is a long log entry"}}}
	max := m.maxScrollOffset()
	if max < 0 {
		t.Fatalf("maxScrollOffset should be >= 0, got %d", max)
	}
}

// ─── nextMsg ────────────────────────────────────────────────────────────────

func TestNextMsg(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	ch <- statusLineMsg{text: "test"}
	close(ch)

	cmd := nextMsg(ch)
	msg := cmd()
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	slm, ok := msg.(statusLineMsg)
	if !ok {
		t.Fatalf("expected statusLineMsg, got %T", msg)
	}
	if slm.text != "test" {
		t.Fatalf("text want %q, got %q", "test", slm.text)
	}
}

func TestNextMsg_ClosedChannel(t *testing.T) {
	ch := make(chan tea.Msg)
	close(ch)

	cmd := nextMsg(ch)
	msg := cmd()
	if msg != nil {
		t.Fatalf("expected nil for closed channel, got %v", msg)
	}
}

// ─── Frequency input model helpers ──────────────────────────────────────────

func TestModel_SettingFreq_Enter(t *testing.T) {
	m := model{height: 30, width: 80, connState: connectionState{connected: true}, ui: uiState{settingFreq: true, freqInput: "14.300"}}
	m = m.withScrollUp() // should not affect freq input mode
	if !m.ui.settingFreq {
		t.Fatal("settingFreq should remain true")
	}
}

func TestModel_ViewFreqPrompt(t *testing.T) {
	m := model{height: 30, width: 80, connState: connectionState{connected: true}, ui: uiState{settingFreq: true, freqInput: "7.200"}}
	prompt := m.viewFreqPrompt()
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "7.200") {
		t.Fatalf("prompt should contain input '7.200', got %q", prompt)
	}
}

func TestModel_ViewFreqPrompt_Hidden(t *testing.T) {
	m := model{height: 30, width: 80, connState: connectionState{connected: true}, ui: uiState{settingFreq: false}}
	prompt := m.viewFreqPrompt()
	if prompt != "" {
		t.Fatalf("expected empty prompt when not setting freq, got %q", prompt)
	}
}
