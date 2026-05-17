// Package slug_test contains unit tests for the Slugify normalizer.
package slug_test

import (
	"testing"

	"github.com/jvcorredor/srs-tui/internal/slug"
)

// TestSlugify exercises every normalization rule and edge case.
func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"already a slug", "hello-world", "hello-world"},
		{"lowercases", "Hello World", "hello-world"},
		{"all uppercase", "SPANISH VERBS", "spanish-verbs"},
		{"spaces to hyphens", "my deck name", "my-deck-name"},
		{"punctuation to hyphens", "what's up?", "what-s-up"},
		{"collapses multiple hyphens", "a---b", "a-b"},
		{"collapses mixed separators", "a _ - . b", "a-b"},
		{"strips leading hyphens", "---hello", "hello"},
		{"strips trailing hyphens", "hello---", "hello"},
		{"strips leading and trailing", "  hello world  ", "hello-world"},
		{"keeps digits", "deck 42", "deck-42"},
		{"all special chars", "!@#$%^&*()", ""},
		{"only spaces", "   ", ""},
		{"only hyphens", "------", ""},
		{"unicode letters become separators", "café münchen", "caf-m-nchen"},
		{"unicode emoji become separators", "deck 🎴 one", "deck-one"},
		{"accented run collapses", "résumé", "r-sum"},
		{"underscores treated as separators", "snake_case_name", "snake-case-name"},
		{"tabs and newlines", "line one\tline\ntwo", "line-one-line-two"},
		{"single character", "A", "a"},
		{"leading digit preserved", "3 blind mice", "3-blind-mice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slug.Slugify(tt.input); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSlugifyIsIdempotent verifies that slugifying an already-slugified value
// returns it unchanged, which callers rely on when re-normalizing stored names.
func TestSlugifyIsIdempotent(t *testing.T) {
	inputs := []string{
		"",
		"hello-world",
		"deck-42",
		"a-b-c",
	}
	for _, in := range inputs {
		once := slug.Slugify(in)
		twice := slug.Slugify(once)
		if once != twice {
			t.Errorf("Slugify not idempotent for %q: once=%q twice=%q", in, once, twice)
		}
	}
}
