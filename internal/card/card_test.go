// Package card_test contains integration tests for the card package.
// Tests exercise the public API (Parse, ParseFile, Serialize, SerializeNew)
// through real file I/O and round-trip validation.
package card_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/card"
)

// TestParseFile exercises ParseFile against golden files and edge cases
// (missing frontmatter, missing ID, malformed YAML).
func TestParseFile(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantID     string
		wantType   card.Type
		wantFront  string
		wantBack   string
		wantNil    bool
		wantErrSub string
	}{
		{
			name:      "basic card with frontmatter",
			path:      "testdata/basic.md",
			wantID:    "01923f44-5a06-7d2e-8c9f-1b2d3e4f5a6b",
			wantType:  card.Basic,
			wantFront: "What is the Go testing convention for table-driven tests?\n",
			wantBack:  "Define a slice of struct cases, range over it, and run `t.Run` per case.\n",
		},
		{
			name:    "no frontmatter returns nil",
			path:    "testdata/no_frontmatter.md",
			wantNil: true,
		},
		{
			name:    "no id returns nil",
			path:    "testdata/no_id.md",
			wantNil: true,
		},
		{
			name:       "malformed frontmatter reports error with path",
			path:       "testdata/malformed.md",
			wantErrSub: "malformed frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := card.ParseFile(tt.path)
			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.path) {
					t.Errorf("error %q should contain path %q", err.Error(), tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseFile() = %+v, want nil", got)
				}
				return
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
			if got.Front != tt.wantFront {
				t.Errorf("Front = %q, want %q", got.Front, tt.wantFront)
			}
			if got.Back != tt.wantBack {
				t.Errorf("Back = %q, want %q", got.Back, tt.wantBack)
			}
		})
	}
}

// TestRoundTripFSRSFields verifies that FSRS scheduling fields survive a
// Serialize → Parse round-trip without loss of precision.
func TestRoundTripFSRSFields(t *testing.T) {
	c := &card.Card{
		Meta: card.Meta{
			Schema:     1,
			ID:         "01923f44-5a06-7d2e-8c9f-1b2d3e4f5a6b",
			Type:       card.Basic,
			Created:    "2026-01-15T10:30:00Z",
			State:      "review",
			Due:        "2026-02-15T10:30:00Z",
			Stability:  12.5,
			Difficulty: 5.2,
			Reps:       4,
			Lapses:     1,
		},
		Front: "Q\n",
		Back:  "A\n",
	}
	serialized := c.Serialize()
	parsed, err := card.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(roundtrip) error: %v", err)
	}
	if parsed.State != c.State {
		t.Errorf("roundtrip State = %q, want %q", parsed.State, c.State)
	}
	if parsed.Due != c.Due {
		t.Errorf("roundtrip Due = %q, want %q", parsed.Due, c.Due)
	}
	if parsed.Stability != c.Stability {
		t.Errorf("roundtrip Stability = %v, want %v", parsed.Stability, c.Stability)
	}
	if parsed.Difficulty != c.Difficulty {
		t.Errorf("roundtrip Difficulty = %v, want %v", parsed.Difficulty, c.Difficulty)
	}
	if parsed.Reps != c.Reps {
		t.Errorf("roundtrip Reps = %d, want %d", parsed.Reps, c.Reps)
	}
	if parsed.Lapses != c.Lapses {
		t.Errorf("roundtrip Lapses = %d, want %d", parsed.Lapses, c.Lapses)
	}
}

// TestRoundTripBasicCard checks that a basic card's ID, Type, Front, and Back
// survive a Parse → Serialize → Parse round-trip.
func TestRoundTripBasicCard(t *testing.T) {
	original, err := os.ReadFile("testdata/basic.md")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	c, err := card.Parse(original)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	serialized := c.Serialize()
	parsed2, err := card.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(roundtrip) error: %v", err)
	}
	if parsed2.ID != c.ID {
		t.Errorf("roundtrip ID = %q, want %q", parsed2.ID, c.ID)
	}
	if parsed2.Type != c.Type {
		t.Errorf("roundtrip Type = %v, want %v", parsed2.Type, c.Type)
	}
	if parsed2.Front != c.Front {
		t.Errorf("roundtrip Front = %q, want %q", parsed2.Front, c.Front)
	}
	if parsed2.Back != c.Back {
		t.Errorf("roundtrip Back = %q, want %q", parsed2.Back, c.Back)
	}
}

// TestParseClozeCardCapturesBody verifies that a cloze card without ## Front/
// ## Back headings has its body text captured in the Body field.
func TestParseClozeCardCapturesBody(t *testing.T) {
	got, err := card.ParseFile("testdata/cloze_single.md")
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}
	if got == nil {
		t.Fatal("ParseFile() = nil, want card")
	}
	if got.Type != card.Cloze {
		t.Errorf("Type = %v, want %v", got.Type, card.Cloze)
	}
	wantBody := "The capital of France is {{c1::Paris}}.\n"
	if got.Body != wantBody {
		t.Errorf("Body = %q, want %q", got.Body, wantBody)
	}
}

// TestParseClozeCardWithGroups verifies that a cloze card with a clozes: map
// in frontmatter parses each group's FSRS state correctly.
func TestParseClozeCardWithGroups(t *testing.T) {
	got, err := card.ParseFile("testdata/cloze_multi.md")
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}
	if got == nil {
		t.Fatal("ParseFile() = nil, want card")
	}
	if got.Type != card.Cloze {
		t.Errorf("Type = %v, want %v", got.Type, card.Cloze)
	}
	if len(got.Clozes) != 2 {
		t.Fatalf("len(Clozes) = %d, want 2", len(got.Clozes))
	}
	c1, ok := got.Clozes["c1"]
	if !ok {
		t.Fatal("missing c1 in Clozes")
	}
	if c1.State != "review" {
		t.Errorf("c1.State = %q, want %q", c1.State, "review")
	}
	if c1.Due != "2026-02-15T10:30:00Z" {
		t.Errorf("c1.Due = %q, want %q", c1.Due, "2026-02-15T10:30:00Z")
	}
	if c1.Stability != 12.5 {
		t.Errorf("c1.Stability = %v, want %v", c1.Stability, 12.5)
	}
	if c1.Difficulty != 5.2 {
		t.Errorf("c1.Difficulty = %v, want %v", c1.Difficulty, 5.2)
	}
	if c1.Reps != 4 {
		t.Errorf("c1.Reps = %d, want %d", c1.Reps, 4)
	}
	if c1.Lapses != 1 {
		t.Errorf("c1.Lapses = %d, want %d", c1.Lapses, 1)
	}
	c2, ok := got.Clozes["c2"]
	if !ok {
		t.Fatal("missing c2 in Clozes")
	}
	if c2.State != "learning" {
		t.Errorf("c2.State = %q, want %q", c2.State, "learning")
	}
	if c2.Stability != 1.5 {
		t.Errorf("c2.Stability = %v, want %v", c2.Stability, 1.5)
	}
}

// TestRoundTripClozeCard checks that a cloze card's ID, Type, Body, and
// per-group FSRS fields survive a Parse → Serialize → Parse round-trip.
func TestRoundTripClozeCard(t *testing.T) {
	original, err := os.ReadFile("testdata/cloze_multi.md")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	c, err := card.Parse(original)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	serialized := c.Serialize()
	parsed2, err := card.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(roundtrip) error: %v", err)
	}
	if parsed2.ID != c.ID {
		t.Errorf("roundtrip ID = %q, want %q", parsed2.ID, c.ID)
	}
	if parsed2.Type != c.Type {
		t.Errorf("roundtrip Type = %v, want %v", parsed2.Type, c.Type)
	}
	if parsed2.Body != c.Body {
		t.Errorf("roundtrip Body = %q, want %q", parsed2.Body, c.Body)
	}
	if len(parsed2.Clozes) != len(c.Clozes) {
		t.Fatalf("roundtrip len(Clozes) = %d, want %d", len(parsed2.Clozes), len(c.Clozes))
	}
	for key, want := range c.Clozes {
		got, ok := parsed2.Clozes[key]
		if !ok {
			t.Errorf("roundtrip missing Clozes[%q]", key)
			continue
		}
		if got.State != want.State {
			t.Errorf("roundtrip Clozes[%q].State = %q, want %q", key, got.State, want.State)
		}
		if got.Due != want.Due {
			t.Errorf("roundtrip Clozes[%q].Due = %q, want %q", key, got.Due, want.Due)
		}
		if got.Stability != want.Stability {
			t.Errorf("roundtrip Clozes[%q].Stability = %v, want %v", key, got.Stability, want.Stability)
		}
		if got.Difficulty != want.Difficulty {
			t.Errorf("roundtrip Clozes[%q].Difficulty = %v, want %v", key, got.Difficulty, want.Difficulty)
		}
		if got.Reps != want.Reps {
			t.Errorf("roundtrip Clozes[%q].Reps = %d, want %d", key, got.Reps, want.Reps)
		}
		if got.Lapses != want.Lapses {
			t.Errorf("roundtrip Clozes[%q].Lapses = %d, want %d", key, got.Lapses, want.Lapses)
		}
	}
}

// TestExtractClozeGroups finds all unique deletion groups in a body string.
func TestExtractClozeGroups(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single group",
			body: "The capital of France is {{c1::Paris}}.",
			want: []string{"c1"},
		},
		{
			name: "multiple groups",
			body: "{{c1::Paris}} is in {{c2::France}}.",
			want: []string{"c1", "c2"},
		},
		{
			name: "group with hint",
			body: "The answer is {{c1::42::a number}}.",
			want: []string{"c1"},
		},
		{
			name: "duplicate groups deduplicated",
			body: "{{c1::A}} and {{c1::B}} are both c1.",
			want: []string{"c1"},
		},
		{
			name: "no groups",
			body: "Plain text without cloze.",
			want: nil,
		},
		{
			name: "mixed groups unordered",
			body: "{{c2::second}} {{c1::first}}",
			want: []string{"c1", "c2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := card.ExtractClozeGroups(tt.body)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractClozeGroups() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractClozeGroups()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
