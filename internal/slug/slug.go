// Package slug normalizes human-readable names into filesystem-safe
// identifiers. It is the shared foundation used by both the CLI commands and
// the TUI when auto-slugifying deck and card names before constructing paths.
package slug

import "strings"

// Slugify converts input into a filesystem-safe identifier: it lowercases the
// input, replaces every run of non-alphanumeric runes (spaces, punctuation,
// unicode letters, etc.) with a single hyphen, and strips leading and trailing
// hyphens. Only ASCII [a-z0-9] runes are preserved; everything else acts as a
// separator. Input that contains no alphanumeric runes yields an empty string.
func Slugify(input string) string {
	var b strings.Builder
	b.Grow(len(input))

	// pendingHyphen records that a separator run is in progress. Starting it
	// true makes leading separators collapse away; a hyphen is only emitted
	// once an alphanumeric rune has actually been written.
	pendingHyphen := true

	for _, r := range strings.ToLower(input) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			pendingHyphen = false
		default:
			pendingHyphen = true
		}
	}

	return b.String()
}
