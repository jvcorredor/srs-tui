// Package deck discovers SRS decks on disk and builds shuffled review queues
// from the Markdown card files contained within each deck directory.
package deck

import (
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/jvcorredor/srs-tui/internal/card"
)

// Discover returns the absolute paths of every immediate subdirectory inside
// root. Only directories are returned; regular files are ignored. Symlinks to
// directories are followed.
func Discover(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var decks []string
	for _, e := range entries {
		full := filepath.Join(root, e.Name())
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			decks = append(decks, full)
		}
	}
	return decks, nil
}

// BuildQueue walks deckDir recursively, parses every .md file into a Card,
// and returns the collected cards in random order. Files that cannot be
// parsed as cards or that lack frontmatter are skipped.
func BuildQueue(deckDir string) ([]*card.Card, error) {
	var cards []*card.Card
	err := filepath.WalkDir(deckDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		c, err := card.ParseFile(path)
		if err != nil {
			return err
		}
		if c == nil {
			return nil
		}
		cards = append(cards, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
	return cards, nil
}
