package main

import "testing"

func TestNormalizeKey_TrimsAndLowercasesAndCollapsesSpaces(t *testing.T) {
	in := "  The   Beatles  "
	out := normalizeKey(in)
	if out != "the beatles" {
		t.Fatalf("expected 'the beatles', got %q", out)
	}
}

func TestNormalizeKey_EmptyAndWhitespace(t *testing.T) {
	if normalizeKey("  ") != "" {
		t.Fatalf("expected empty string for whitespace-only input")
	}
}