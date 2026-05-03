package card_test

import (
	"strings"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/card"
)

func TestClozeSerializeOmitsFlatFields(t *testing.T) {
	c := &card.Card{
		Meta: card.Meta{
			Schema:  1,
			ID:      "test-id",
			Type:    card.Cloze,
			Created: "2026-01-01T00:00:00Z",
			Clozes: map[string]card.ClozeGroup{
				"c1": {State: "new"},
			},
		},
		Body: "{{c1::A}}\n",
	}
	out := string(c.Serialize())
	if strings.Contains(out, "clozes:") {
		// Extract frontmatter only
		lines := strings.Split(out, "\n")
		inFM := false
		var fmLines []string
		for _, line := range lines {
			if line == "---" {
				if !inFM {
					inFM = true
					continue
				} else {
					break
				}
			}
			if inFM {
				fmLines = append(fmLines, line)
			}
		}
		for _, flatField := range []string{"state:", "due:", "stability:", "difficulty:", "reps:", "lapses:"} {
			// Check for top-level flat field (line starts with field name, not indented)
			for _, line := range fmLines {
				if strings.HasPrefix(line, flatField) {
					t.Errorf("cloze frontmatter should not contain top-level flat field %q, got line: %s", flatField, line)
				}
			}
		}
	} else {
		t.Errorf("cloze serialization should contain clozes: map, got:\n%s", out)
	}
}

func TestBasicSerializeIncludesFlatFields(t *testing.T) {
	c := &card.Card{
		Meta: card.Meta{
			Schema:  1,
			ID:      "test-id",
			Type:    card.Basic,
			Created: "2026-01-01T00:00:00Z",
			State:   "review",
			Due:     "2026-02-01T00:00:00Z",
			Stability: 5.0,
		},
		Front: "Q\n",
		Back:  "A\n",
	}
	out := string(c.Serialize())
	if !strings.Contains(out, "state: review") {
		t.Errorf("basic serialization should contain flat state field, got:\n%s", out)
	}
	if strings.Contains(out, "clozes:") {
		t.Errorf("basic serialization should not contain clozes: map, got:\n%s", out)
	}
}
