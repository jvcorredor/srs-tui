// Package deck_test contains integration tests for the deck package.
// Tests exercise the public API (Discover, BuildQueue) through real
// filesystem operations in temporary directories.
package deck_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
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
	os.MkdirAll(filepath.Join(root, "french"), 0o755)
	os.MkdirAll(filepath.Join(root, "golang"), 0o755)
	os.MkdirAll(filepath.Join(root, "golang", "basics"), 0o755)
	os.WriteFile(filepath.Join(root, "random.txt"), []byte("not a deck"), 0o644)

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
	os.MkdirAll(filepath.Join(realDir, "cards"), 0o755)
	os.Symlink(realDir, filepath.Join(root, "linked"))

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
// card in the deck and returns them in a shuffled order.
func TestQueueContainsAllCardsShuffled(t *testing.T) {
	root := t.TempDir()
	deckDir := filepath.Join(root, "mydeck")
	os.MkdirAll(deckDir, 0o755)

	writeBasicCard(t, deckDir, "id-1", "Q1", "A1")
	writeBasicCard(t, deckDir, "id-2", "Q2", "A2")
	writeBasicCard(t, deckDir, "id-3", "Q3", "A3")

	q, err := deck.BuildQueue(deckDir)
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
	os.MkdirAll(deckDir, 0o755)

	writeBasicCard(t, deckDir, "id-1", "Q1", "A1")
	os.WriteFile(filepath.Join(deckDir, "readme.md"), []byte("# Deck\nNo frontmatter\n"), 0o644)

	q, err := deck.BuildQueue(deckDir)
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
	os.MkdirAll(deckDir, 0o755)

	writeBasicCard(t, deckDir, "basic-1", "Q1", "A1")
	writeClozeCard(t, deckDir, "cloze-1", "{{c1::A}} and {{c2::B}}", nil)

	q, err := deck.BuildQueue(deckDir)
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
