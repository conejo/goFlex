// wordwrap.go — word-wrap utility.

package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const contPrefix = "  ↳ "

var styleContPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// wordWrap breaks s into lines no wider than width, preferring space boundaries.
// Falls back to a hard break when no space exists within the width.
// Tabs are expanded to 4 spaces before wrapping.
func wordWrap(s string, width int) []string {
	s = strings.ReplaceAll(s, "\t", "    ")
	var lines []string
	runes := []rune(s)
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		// Find the last space within [0, width].
		cut := width
		for cut > 0 && runes[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			// No space found — hard break at width.
			cut = width
		}
		lines = append(lines, string(runes[:cut]))
		// Skip the breaking space if we broke on one.
		if cut < len(runes) && runes[cut] == ' ' {
			cut++
		}
		runes = runes[cut:]
	}
	return lines
}

// wrapLogEntries wraps each log entry to wrapWidth, indenting continuation lines.
func wrapLogEntries(entries []string, wrapWidth int) []string {
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	contPrefixLen := len([]rune(contPrefix))
	contWidth := wrapWidth - contPrefixLen
	if contWidth < 1 {
		contWidth = 1
	}

	wrapped := make([]string, 0, len(entries))
	for _, l := range entries {
		lines := wordWrap(l, wrapWidth)
		if len(lines) == 0 {
			wrapped = append(wrapped, l)
			continue
		}
		var sb strings.Builder
		sb.WriteString(lines[0])
		for _, cont := range lines[1:] {
			for _, cl := range wordWrap(cont, contWidth) {
				sb.WriteString("\n")
				sb.WriteString(styleContPrefix.Render(contPrefix))
				sb.WriteString(cl)
			}
		}
		wrapped = append(wrapped, sb.String())
	}
	return wrapped
}
