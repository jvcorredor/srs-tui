package card

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Type string

const (
	Basic Type = "basic"
	Cloze Type = "cloze"
)

type Meta struct {
	Schema  int      `yaml:"schema"`
	ID      string   `yaml:"id"`
	Type    Type     `yaml:"type"`
	Created string   `yaml:"created"`
	Tags    []string `yaml:"tags"`
}

type Card struct {
	Meta
	Front string
	Back  string
}

var frontHeading = regexp.MustCompile(`(?m)^## Front\s*$`)
var backHeading = regexp.MustCompile(`(?m)^## Back\s*$`)

func ParseFile(path string) (*Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("card: read %s: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("card: %s: %w", path, err)
	}
	return c, nil
}

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
	return &Card{
		Meta:  *fm,
		Front: front,
		Back:  back,
	}, nil
}

func (c *Card) Serialize() []byte {
	fmData, _ := yaml.Marshal(&c.Meta)
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fmData)
	b.WriteString("---\n\n")
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
	return []byte(b.String())
}

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
