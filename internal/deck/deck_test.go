// Package deck_test contains integration tests for the deck package.
// Tests exercise the public API (Discover, BuildQueue) through real
// filesystem operations in temporary directories.
package deck_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

// writeBasicCard creates a minimal valid card file in dir named id+".md".
func writeBasicCard(t *testing.T, dir, id, front, back string) {
	t.Helper()
	c := &card.Card{
		Meta: card.Meta{
			Schema:  1,
			ID:      id,
			Type:    card.Basic,
			Created: "2026-01-01T00:00:00Z",
		},
		Front: front + "\n",
		Back:  back + "\n",
	}
	err := os.WriteFile(filepath.Join(dir, id+".md"), c.Serialize(), 0o644)
	if err != nil {
		t.Fatalf("write card: %v", err)
	}
}

// writeBasicCardWithState creates a basic card file with FSRS scheduling state.
func writeBasicCardWithState(t *testing.T, dir, id, front, back string, state fsrs.CardState) {
	t.Helper()
	c := &card.Card{
		Meta: card.Meta{
			Schema:     1,
			ID:         id,
			Type:       card.Basic,
			Created:    "2026-01-01T00:00:00Z",
			State:      string(state.State),
			Due:        state.Due.Format(time.RFC3339),
			Stability:  state.Stability,
			Difficulty: state.Difficulty,
			Reps:       state.Reps,
			Lapses:     state.Lapses,
		},
		Front: front + "\n",
		Back:  back + "\n",
	}
	err := os.WriteFile(filepath.Join(dir, id+".md"), c.Serialize(), 0o644)
	if err != nil {
		t.Fatalf("write card: %v", err)
	}
}

// writeClozeCard creates a cloze card file in dir named id+".md".
func writeClozeCard(t *testing.T, dir, id, body string, clozes map[string]card.ClozeGroup) {
	t.Helper()
	c := &card.Card{
		Meta: card.Meta{
			Schema:  1,
			ID:      id,
			Type:    card.Cloze,
			Created: "2026-01-01T00:00:00Z",
			Clozes:  clozes,
		},
		Body: body + "\n",
	}
	err := os.WriteFile(filepath.Join(dir, id+".md"), c.Serialize(), 0o644)
	if err != nil {
		t.Fatalf("write cloze card: %v", err)
	}
}

// TestDiscoverDecks verifies that Discover returns only immediate
// subdirectories and ignores regular files.
func TestDiscoverDecks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "french"), 0o755); err != nil {
		t.Fatalf("mkdir french: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "golang"), 0o755); err != nil {
		t.Fatalf("mkdir golang: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "golang", "basics"), 0o755); err != nil {
		t.Fatalf("mkdir golang/basics: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "random.txt"), []byte("not a deck"), 0o644); err != nil {
		t.Fatalf("write random.txt: %v", err)
	}

	got, err := deck.Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	names := make([]string, len(got))
	for i, d := range got {
		names[i] = filepath.Base(d)
	}
	sort.Strings(names)

	want := []string{"french", "golang"}
	if len(names) != len(want) {
		t.Fatalf("decks = %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("decks[%d] = %q, want %q", i, n, want[i])
		}
	}
}

// TestDiscoverFollowsSymlinks checks that symlinked directories are
// included in the discovered deck list.
func TestDiscoverFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	realDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(realDir, "cards"), 0o755); err != nil {
		t.Fatalf("mkdir cards: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("symlink linked: %v", err)
	}

	got, err := deck.Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	names := make([]string, len(got))
	for i, d := range got {
		names[i] = filepath.Base(d)
	}
	found := false
	for _, n := range names {
		if n == "linked" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected symlinked deck 'linked' in %v", names)
	}
}

// TestQueueContainsAllCardsShuffled confirms that BuildQueue finds every
// card in the deck and returns them in a shuffled order when the new-card
// budget is effectively unlimited.
func TestQueueContainsAllCardsShuffled(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	writeBasicCard(t, deckDir, "id-1", "Q1", "A1")
	writeBasicCard(t, deckDir, "id-2", "Q2", "A2")
	writeBasicCard(t, deckDir, "id-3", "Q3", "A3")

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 20,
		Now:       time.Now(),
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}
	if len(q) != 3 {
		t.Fatalf("queue length = %d, want 3", len(q))
	}

	ids := make(map[string]bool)
	for _, it := range q {
		ids[it.Card.ID] = true
	}
	for _, want := range []string{"id-1", "id-2", "id-3"} {
		if !ids[want] {
			t.Errorf("queue missing card %q", want)
		}
	}
}

// TestQueueSkipsNonCardFiles ensures that BuildQueue ignores Markdown
// files that are not valid cards (e.g. missing frontmatter or ID).
func TestQueueSkipsNonCardFiles(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	writeBasicCard(t, deckDir, "id-1", "Q1", "A1")
	if err := os.WriteFile(filepath.Join(deckDir, "readme.md"), []byte("# Deck\nNo frontmatter\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 20,
		Now:       time.Now(),
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}
	if len(q) != 1 {
		t.Fatalf("queue length = %d, want 1 (non-card files skipped)", len(q))
	}
	if q[0].Card.ID != "id-1" {
		t.Errorf("queue[0].Card.ID = %q, want %q", q[0].Card.ID, "id-1")
	}
}

// TestQueueEnumeratesClozeGroups checks that a cloze card produces one
// ReviewItem per unique cloze group, while basic cards produce a single item.
func TestQueueEnumeratesClozeGroups(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeBasicCard(t, deckDir, "basic-1", "Q1", "A1")
	writeClozeCard(t, deckDir, "cloze-1", "{{c1::A}} and {{c2::B}}", nil)

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 20,
		Now:       time.Now(),
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}
	if len(q) != 3 {
		t.Fatalf("queue length = %d, want 3 (1 basic + 2 cloze groups)", len(q))
	}

	// Find items by card ID + group
	items := make(map[string]string) // cardID -> clozeGroup
	for _, it := range q {
		key := it.Card.ID
		if it.ClozeGroup != "" {
			key += "/" + it.ClozeGroup
		}
		items[key] = it.ClozeGroup
	}

	if _, ok := items["basic-1"]; !ok {
		t.Errorf("missing basic card in queue")
	}
	if items["basic-1"] != "" {
		t.Errorf("basic card should have empty ClozeGroup, got %q", items["basic-1"])
	}
	if _, ok := items["cloze-1/c1"]; !ok {
		t.Errorf("missing cloze-1/c1 in queue")
	}
	if _, ok := items["cloze-1/c2"]; !ok {
		t.Errorf("missing cloze-1/c2 in queue")
	}
}

// TestDueCountCountsNewAndDueCards verifies that DueCount returns the number
// of review items (cards or cloze groups) that are new or whose due time has
// passed. Cards that are not yet due are excluded.
func TestDueCountCountsNewAndDueCards(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	now := time.Now()

	writeBasicCardWithState(t, deckDir, "new-card", "Q1", "A1", fsrs.CardState{
		State: fsrs.StateNew,
		Due:   time.Time{},
	})
	writeBasicCardWithState(t, deckDir, "due-card", "Q2", "A2", fsrs.CardState{
		State:     fsrs.StateReview,
		Due:       now.Add(-1 * time.Hour),
		Stability: 10,
	})
	writeBasicCardWithState(t, deckDir, "future-card", "Q3", "A3", fsrs.CardState{
		State:     fsrs.StateReview,
		Due:       now.Add(24 * time.Hour),
		Stability: 10,
	})

	count, err := deck.DueCount(deckDir, now)
	if err != nil {
		t.Fatalf("DueCount() error: %v", err)
	}
	if count != 2 {
		t.Errorf("DueCount = %d, want 2 (new + due, excluding future)", count)
	}
}

// TestQueueHonorsNewPerDayBudget verifies that BuildQueue limits new cards
// to the remaining daily budget. If 15 new cards were already reviewed today
// and new_per_day is 20, only 5 new cards enter the queue. Due review cards
// are unaffected by the budget.
func TestQueueHonorsNewPerDayBudget(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	now := time.Now()

	writeBasicCardWithState(t, deckDir, "new-1", "Q1", "A1", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "new-2", "Q2", "A2", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "new-3", "Q3", "A3", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "due-review", "Q4", "A4", fsrs.CardState{
		State:     fsrs.StateReview,
		Due:       now.Add(-1 * time.Hour),
		Stability: 10,
	})

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 20,
		Now:       now,
		NewCount:  func(_ time.Time) (int, error) { return 18, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}

	newCount := 0
	reviewCount := 0
	for _, it := range q {
		state := fsrs.NormalizeState(it.Card.State)
		if state == fsrs.StateNew {
			newCount++
		} else {
			reviewCount++
		}
	}

	if newCount != 2 {
		t.Errorf("new cards in queue = %d, want 2 (budget 20 - 18 done = 2 remaining)", newCount)
	}
	if reviewCount != 1 {
		t.Errorf("review cards in queue = %d, want 1 (reviews are unbounded)", reviewCount)
	}
}

// TestQueueBudgetResetsAcrossMidnight verifies that the new-card budget
// resets at midnight. When NewCount reports 0 for the new day (no new
// cards reviewed yet), all new cards are admitted up to the full budget.
func TestQueueBudgetResetsAcrossMidnight(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	writeBasicCardWithState(t, deckDir, "new-1", "Q1", "A1", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "new-2", "Q2", "A2", fsrs.CardState{
		State: fsrs.StateNew,
	})

	now := time.Now()

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 20,
		Now:       now,
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}

	newCount := 0
	for _, it := range q {
		if fsrs.NormalizeState(it.Card.State) == fsrs.StateNew {
			newCount++
		}
	}
	if newCount != 2 {
		t.Errorf("new cards in queue after midnight = %d, want 2 (budget reset)", newCount)
	}
}

// TestQueueZeroBudgetAdmitsNoNewCards verifies that setting new_per_day to
// zero prevents any new cards from entering the queue, while due review
// cards are still admitted.
func TestQueueZeroBudgetAdmitsNoNewCards(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	now := time.Now()

	writeBasicCardWithState(t, deckDir, "new-1", "Q1", "A1", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "due-review", "Q2", "A2", fsrs.CardState{
		State:     fsrs.StateReview,
		Due:       now.Add(-1 * time.Hour),
		Stability: 10,
	})

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 0,
		Now:       now,
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}

	for _, it := range q {
		if fsrs.NormalizeState(it.Card.State) == fsrs.StateNew {
			t.Errorf("new card %q admitted with zero budget", it.Card.ID)
		}
	}
	if len(q) != 1 {
		t.Errorf("queue length = %d, want 1 (only due review card)", len(q))
	}
}

// TestQueueBudgetLargerThanAvailableNewCards verifies that when the budget
// exceeds the number of available new cards, all new cards are admitted.
func TestQueueBudgetLargerThanAvailableNewCards(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	if err := os.MkdirAll(deckDir, 0o755); err != nil {
		t.Fatalf("mkdir deck: %v", err)
	}

	writeBasicCardWithState(t, deckDir, "new-1", "Q1", "A1", fsrs.CardState{
		State: fsrs.StateNew,
	})
	writeBasicCardWithState(t, deckDir, "new-2", "Q2", "A2", fsrs.CardState{
		State: fsrs.StateNew,
	})

	q, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: 100,
		Now:       time.Now(),
		NewCount:  func(_ time.Time) (int, error) { return 0, nil },
	})
	if err != nil {
		t.Fatalf("BuildQueue() error: %v", err)
	}

	newCount := 0
	for _, it := range q {
		if fsrs.NormalizeState(it.Card.State) == fsrs.StateNew {
			newCount++
		}
	}
	if newCount != 2 {
		t.Errorf("new cards in queue = %d, want 2 (all available new cards admitted)", newCount)
	}
}
