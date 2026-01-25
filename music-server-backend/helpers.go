package main

import (
	"strings"
)

// normalizeKey trims, folds case, and collapses whitespace for stable comparisons.
func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	// Collapse consecutive whitespace and normalize internal spacing
	s = strings.Join(strings.Fields(s), " ")
	// Lowercase for case-insensitive matching
	s = strings.ToLower(s)
	return s
}