// Package deck discovers SRS decks on disk and builds shuffled review queues
// from the Markdown card files contained within each deck directory.
package deck

import (
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/jvcorredor/srs-tui/internal/card"
)

// ReviewItem represents a single unit of review: a card and, for cloze cards,
// the specific deletion group being reviewed. Basic cards have an empty ClozeGroup.
type ReviewItem struct {
	Card       *card.Card
	ClozeGroup string
}

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
// and returns review items in random order. Basic cards produce a single item;
// cloze cards produce one item per unique cloze group found in the body.
// Files that cannot be parsed as cards or that lack frontmatter are skipped.
func BuildQueue(deckDir string) ([]ReviewItem, error) {
	var items []ReviewItem
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
		if c.Type == card.Cloze {
			groups := card.ExtractClozeGroups(c.Body)
			if len(groups) == 0 {
				// No cloze markers found; enqueue as a single item.
				items = append(items, ReviewItem{Card: c})
			} else {
				for _, g := range groups {
					items = append(items, ReviewItem{Card: c, ClozeGroup: g})
				}
			}
		} else {
			items = append(items, ReviewItem{Card: c})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	return items, nil
}
