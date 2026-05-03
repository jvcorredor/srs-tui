package card_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/card"
)

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
