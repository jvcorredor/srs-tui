// Package deck discovers SRS decks on disk and builds shuffled review queues
// from the Markdown card files contained within each deck directory.
package deck

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
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

// DueCount returns the number of review items in the deck that are due for
// review at the given time. An item is due if its FSRS state is "new" or if
// its scheduled due time is at or before now. Cloze cards are counted per
// group (each group is an independent review item).
func DueCount(deckDir string, now time.Time) (int, error) {
	var count int
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
				if isItemDue(c.Meta, "", now) {
					count++
				}
			} else {
				for _, g := range groups {
					if isItemDue(c.Meta, g, now) {
						count++
					}
				}
			}
		} else {
			if isItemDue(c.Meta, "", now) {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// isItemDue reports whether a review item is due at the given time.
// For cloze cards, group is the active cloze group identifier (e.g. "c1").
// An item is due if its state is new or its due time is at or before now.
func isItemDue(meta card.Meta, group string, now time.Time) bool {
	if group != "" {
		if cg, ok := meta.Clozes[group]; ok {
			state := fsrs.NormalizeState(cg.State)
			if state == fsrs.StateNew {
				return true
			}
			due := fsrs.ParseTime(cg.Due)
			return !due.IsZero() && !due.After(now)
		}
	}
	state := fsrs.NormalizeState(meta.State)
	if state == fsrs.StateNew {
		return true
	}
	due := fsrs.ParseTime(meta.Due)
	return !due.IsZero() && !due.After(now)
}
