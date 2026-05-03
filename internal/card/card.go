// Package card implements the Markdown card file model used by srs-tui.
//
// A card is stored as a Markdown file with YAML frontmatter followed by
// ## Front and ## Back sections. The frontmatter contains metadata such as
// ID, type, creation time, and FSRS scheduling fields.
package card

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Type describes the kind of card.
type Type string

const (
	// Basic is a standard question/answer card.
	Basic Type = "basic"
	// Cloze is a fill-in-the-blank card.
	Cloze Type = "cloze"
)

// ClozeGroup holds the FSRS scheduling state for a single cloze deletion
// group (e.g. "c1", "c2").
type ClozeGroup struct {
	State      string  `yaml:"state,omitempty"`
	Due        string  `yaml:"due,omitempty"`
	Stability  float64 `yaml:"stability,omitempty"`
	Difficulty float64 `yaml:"difficulty,omitempty"`
	Reps       int     `yaml:"reps,omitempty"`
	Lapses     int     `yaml:"lapses,omitempty"`
}

// Meta holds the YAML frontmatter for a card.
type Meta struct {
	Schema     int                   `yaml:"schema"`
	ID         string                `yaml:"id"`
	Type       Type                  `yaml:"type"`
	Created    string                `yaml:"created"`
	Tags       []string              `yaml:"tags"`
	State      string                `yaml:"state,omitempty"`
	Due        string                `yaml:"due,omitempty"`
	Stability  float64               `yaml:"stability,omitempty"`
	Difficulty float64               `yaml:"difficulty,omitempty"`
	Reps       int                   `yaml:"reps,omitempty"`
	Lapses     int                   `yaml:"lapses,omitempty"`
	Clozes     map[string]ClozeGroup `yaml:"clozes,omitempty"`
}

// Card represents a spaced-repetition card backed by a Markdown file.
type Card struct {
	Meta
	Front    string
	Back     string
	Body     string // raw body for cloze cards (no Front/Back headings)
	FilePath string
}

var frontHeading = regexp.MustCompile(`(?m)^## Front\s*$`)
var backHeading = regexp.MustCompile(`(?m)^## Back\s*$`)

// NewCard creates a new card with a generated UUID v7 and the given type.
func NewCard(cardType Type, now time.Time) *Card {
	id, _ := uuid.NewV7()
	return &Card{
		Meta: Meta{
			Schema:  1,
			ID:      id.String(),
			Type:    cardType,
			Created: now.Format(time.RFC3339),
			Tags:    []string{},
		},
	}
}

// ParseFile reads a Markdown card file and parses it into a Card.
// The file path is recorded in the returned Card's FilePath field.
func ParseFile(path string) (*Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("card: read %s: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("card: %s: %w", path, err)
	}
	if c != nil {
		c.FilePath = path
	}
	return c, nil
}

// Parse converts raw Markdown bytes into a Card.
// If the data has no frontmatter or no ID, it returns nil, nil.
func Parse(data []byte) (*Card, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return nil, nil
	}
	if fm.ID == "" {
		return nil, nil
	}
	front, back := splitBody(string(body))
	c := &Card{
		Meta:  *fm,
		Front: front,
		Back:  back,
	}
	// For cloze cards (no Front/Back headings), capture the raw body.
	if front == "" && back == "" && len(body) > 0 {
		c.Body = strings.TrimSpace(string(body)) + "\n"
	}
	return c, nil
}

var clozePattern = regexp.MustCompile(`\{\{c(\d+)::([^}]+)\}\}`)

// ExtractClozeGroups finds all unique cloze deletion groups (e.g. "c1", "c2")
// in body and returns them sorted. Both {{cN::answer}} and
// {{cN::answer::hint}} syntax are supported.
func ExtractClozeGroups(body string) []string {
	seen := make(map[string]bool)
	matches := clozePattern.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			group := "c" + m[1]
			seen[group] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	groups := make([]string, 0, len(seen))
	for g := range seen {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	return groups
}

// Serialize writes the card back to its Markdown representation.
// Basic cards include Front/Back sections; cloze cards write the raw body.
func (c *Card) Serialize() []byte {
	var fmData []byte
	if c.Type == Cloze {
		fmData, _ = yaml.Marshal(struct {
			Schema  int                   `yaml:"schema"`
			ID      string                `yaml:"id"`
			Type    Type                  `yaml:"type"`
			Created string                `yaml:"created"`
			Tags    []string              `yaml:"tags"`
			Clozes  map[string]ClozeGroup `yaml:"clozes,omitempty"`
		}{
			Schema:  c.Schema,
			ID:      c.ID,
			Type:    c.Type,
			Created: c.Created,
			Tags:    c.Tags,
			Clozes:  c.Clozes,
		})
	} else {
		fmData, _ = yaml.Marshal(&c.Meta)
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fmData)
	b.WriteString("---\n\n")
	if c.Type == Cloze {
		b.WriteString(c.Body)
		if !strings.HasSuffix(c.Body, "\n") {
			b.WriteByte('\n')
		}
	} else {
		b.WriteString("## Front\n\n")
		b.WriteString(c.Front)
		if !strings.HasSuffix(c.Front, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("## Back\n\n")
		b.WriteString(c.Back)
		if !strings.HasSuffix(c.Back, "\n") {
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

// SerializeNew returns a minimal Markdown template for a newly created card.
// For cloze cards it includes a placeholder; for basic cards it provides
// empty Front and Back headings.
func (c *Card) SerializeNew() []byte {
	fmData, _ := yaml.Marshal(&c.Meta)
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fmData)
	b.WriteString("---\n\n")
	if c.Type == Cloze {
		b.WriteString("{{c1::answer}}\n")
	} else {
		b.WriteString("## Front\n\n\n\n## Back\n")
	}
	return []byte(b.String())
}

// splitFrontmatter extracts the YAML frontmatter and the remaining body from data.
// If there is no frontmatter, it returns nil, data, nil.
func splitFrontmatter(data []byte) (*Meta, []byte, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, data, nil
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, data, nil
	}
	fmData := data[4 : end+4]
	body := data[end+9:]

	var meta Meta
	if err := yaml.Unmarshal(fmData, &meta); err != nil {
		return nil, nil, fmt.Errorf("card: malformed frontmatter: %w", err)
	}
	return &meta, body, nil
}

// splitBody extracts the front and back text from the Markdown body
// by looking for ## Front and ## Back headings.
func splitBody(body string) (front, back string) {
	loc := frontHeading.FindStringIndex(body)
	if loc != nil {
		frontStart := loc[1]
		afterFront := body[frontStart:]

		backLoc := backHeading.FindStringIndex(afterFront)
		if backLoc != nil {
			front = strings.TrimSpace(afterFront[:backLoc[0]]) + "\n"
			backStart := backLoc[1]
			back = strings.TrimSpace(afterFront[backStart:]) + "\n"
		} else {
			front = strings.TrimSpace(afterFront) + "\n"
		}
	}
	return
}
