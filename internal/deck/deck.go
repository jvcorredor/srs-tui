// Package deck discovers decks and builds review queues from card files.
package deck

import (
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/jvcorredor/srs-tui/internal/card"
)

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
