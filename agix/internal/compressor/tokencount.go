package compressor

import "strings"

// estimateTokens approximates the token count of a string.
// Uses word count * 1.3 as a heuristic (good enough for threshold checks).
func estimateTokens(s string) int {
	words := len(strings.Fields(s))
	return int(float64(words) * 1.3)
}
