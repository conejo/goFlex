// wordwrap.go — word-wrap utility.

package main

// wordWrap breaks s into lines no wider than width, preferring space boundaries.
// Falls back to a hard break when no space exists within the width.
func wordWrap(s string, width int) []string {
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
