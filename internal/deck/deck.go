// Package deck discovers SRS decks on disk and builds shuffled review queues
// from the Markdown card files contained within each deck directory.
package deck

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
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

// QueueConfig controls how BuildQueue assembles the review queue.
type QueueConfig struct {
	// NewPerDay is the daily ceiling on how many state:new cards enter the queue.
	NewPerDay int
	// Now is the reference time for due-date comparisons and midnight rollover.
	Now time.Time
	// NewCount returns how many new cards have already been reviewed today
	// (i.e. log entries where prev.state == "new" for the current local-calendar day).
	NewCount func(now time.Time) (int, error)
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
//
// The cfg parameter controls new-card budgeting: at most cfg.NewPerDay new
// cards are admitted per local-calendar day, minus any already reviewed today
// (counted by cfg.NewCount). Due review cards are unbounded. New cards are
// ordered by created ASC before capping, then merged with due reviews and
// shuffled.
func BuildQueue(deckDir string, cfg QueueConfig) ([]ReviewItem, error) {
	var newItems, reviewItems []ReviewItem
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
		classifyItems(c, &newItems, &reviewItems, cfg.Now)
		return nil
	})
	if err != nil {
		return nil, err
	}

	remaining := cfg.NewPerDay
	if cfg.NewCount != nil {
		done, err := cfg.NewCount(cfg.Now)
		if err != nil {
			return nil, err
		}
		remaining -= done
	}
	if remaining < 0 {
		remaining = 0
	}

	sortByCreated(newItems)
	if len(newItems) > remaining {
		newItems = newItems[:remaining]
	}

	items := append(reviewItems, newItems...)
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	return items, nil
}

// classifyItems partitions a card's review items into newItems and reviewItems
// based on their FSRS state and due status at the given time.
func classifyItems(c *card.Card, newItems, reviewItems *[]ReviewItem, now time.Time) {
	if c.Type == card.Cloze {
		groups := card.ExtractClozeGroups(c.Body)
		if len(groups) == 0 {
			classifyOne(c, "", newItems, reviewItems, now)
		} else {
			for _, g := range groups {
				classifyOne(c, g, newItems, reviewItems, now)
			}
		}
	} else {
		classifyOne(c, "", newItems, reviewItems, now)
	}
}

// classifyOne classifies a single review item as new, due, or neither.
func classifyOne(c *card.Card, group string, newItems, reviewItems *[]ReviewItem, now time.Time) {
	item := ReviewItem{Card: c}
	if group != "" {
		item.ClozeGroup = group
	}
	if isItemNew(c.Meta, group) {
		*newItems = append(*newItems, item)
	} else if isItemDue(c.Meta, group, now) {
		*reviewItems = append(*reviewItems, item)
	}
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

// isItemNew reports whether a review item is in the "new" state.
func isItemNew(meta card.Meta, group string) bool {
	if group != "" {
		if cg, ok := meta.Clozes[group]; ok {
			return fsrs.NormalizeState(cg.State) == fsrs.StateNew
		}
	}
	return fsrs.NormalizeState(meta.State) == fsrs.StateNew
}

// sortByCreated sorts review items by their card's Created timestamp ASC.
func sortByCreated(items []ReviewItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Card.Created < items[j].Card.Created
	})
}
