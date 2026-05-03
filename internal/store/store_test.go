// Package store_test contains integration tests for the store package.
// Tests exercise the public API (AppendLog, Persist, RewriteCard, EnsureID)
// through real file I/O and round-trip validation.
package store_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/store"
)

// TestAppendLogWritesOneJSONLinePerCall checks that a single AppendLog call
// produces exactly one valid JSON line with the expected fields.
func TestAppendLogWritesOneJSONLinePerCall(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir, "mydeck")

	entry := store.LogEntry{
		Schema:     1,
		TS:         time.Now().UTC().Truncate(time.Millisecond),
		CardID:     "card-1",
		Rating:     3,
		DurationMs: 5000,
		Prev:       fsrs.CardState{State: fsrs.StateNew},
		Next:       fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5},
	}

	if err := s.AppendLog(entry); err != nil {
		t.Fatalf("AppendLog() error: %v", err)
	}

	path := filepath.Join(dir, "mydeck.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var got store.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal line %d: %v", lineCount, err)
		}
		if got.Schema != entry.Schema {
			t.Errorf("schema = %d, want %d", got.Schema, entry.Schema)
		}
		if got.CardID != entry.CardID {
			t.Errorf("card_id = %q, want %q", got.CardID, entry.CardID)
		}
		if got.Rating != entry.Rating {
			t.Errorf("rating = %d, want %d", got.Rating, entry.Rating)
		}
		if got.DurationMs != entry.DurationMs {
			t.Errorf("duration_ms = %d, want %d", got.DurationMs, entry.DurationMs)
		}
		if got.Prev.State != entry.Prev.State {
			t.Errorf("prev.state = %q, want %q", got.Prev.State, entry.Prev.State)
		}
		if got.Next.State != entry.Next.State {
			t.Errorf("next.state = %q, want %q", got.Next.State, entry.Next.State)
		}
		if got.Next.Stability != entry.Next.Stability {
			t.Errorf("next.stability = %v, want %v", got.Next.Stability, entry.Next.Stability)
		}
	}
	if lineCount != 1 {
		t.Errorf("line count = %d, want 1", lineCount)
	}
}

// TestAppendLogMultipleEntries verifies that consecutive AppendLog calls
// append multiple independent JSON lines to the log file.
func TestAppendLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir, "mydeck")

	for i := 0; i < 3; i++ {
		entry := store.LogEntry{
			Schema: 1,
			TS:     time.Now().UTC().Truncate(time.Millisecond),
			CardID: "card-1",
			Rating: i + 1,
			Prev:   fsrs.CardState{State: fsrs.StateNew},
			Next:   fsrs.CardState{State: fsrs.StateLearning},
		}
		if err := s.AppendLog(entry); err != nil {
			t.Fatalf("AppendLog(%d) error: %v", i, err)
		}
	}

	path := filepath.Join(dir, "mydeck.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var got store.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal line %d: %v", lineCount, err)
		}
	}
	if lineCount != 3 {
		t.Errorf("line count = %d, want 3", lineCount)
	}
}

// TestAppendLogIncludesClozeGroupWhenSet confirms that the ClozeGroup
// field is marshaled when non-nil and omitted when nil.
func TestAppendLogIncludesClozeGroupWhenSet(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir, "mydeck")

	clozeGroup := "c1"
	entry := store.LogEntry{
		Schema:     1,
		TS:         time.Now().UTC().Truncate(time.Millisecond),
		CardID:     "card-1",
		ClozeGroup: &clozeGroup,
		Rating:     3,
		Prev:       fsrs.CardState{State: fsrs.StateNew},
		Next:       fsrs.CardState{State: fsrs.StateLearning},
	}

	if err := s.AppendLog(entry); err != nil {
		t.Fatalf("AppendLog() error: %v", err)
	}

	path := filepath.Join(dir, "mydeck.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}

	var got store.LogEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ClozeGroup == nil || *got.ClozeGroup != clozeGroup {
		t.Errorf("cloze_group = %v, want %q", got.ClozeGroup, clozeGroup)
	}
}

// TestRewriteCardAtomicNoTmpArtifact checks that RewriteCard leaves no
// temporary files behind and correctly updates card frontmatter fields.
func TestRewriteCardAtomicNoTmpArtifact(t *testing.T) {
	cardDir := t.TempDir()
	cardPath := filepath.Join(cardDir, "test.md")

	orig := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			ID:     "card-1",
			Type:   card.Basic,
		},
		Front: "Q\n",
		Back:  "A\n",
	}
	os.WriteFile(cardPath, orig.Serialize(), 0o644)

	s := store.NewStore(t.TempDir(), "mydeck")
	orig.State = "learning"
	orig.Stability = 1.5

	if err := s.RewriteCard(cardPath, orig); err != nil {
		t.Fatalf("RewriteCard() error: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(cardDir, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("tmp artifacts remain: %v", matches)
	}

	parsed, err := card.ParseFile(cardPath)
	if err != nil {
		t.Fatalf("ParseFile after rewrite: %v", err)
	}
	if parsed.State != "learning" {
		t.Errorf("state after rewrite = %q, want %q", parsed.State, "learning")
	}
	if parsed.Stability != 1.5 {
		t.Errorf("stability after rewrite = %v, want 1.5", parsed.Stability)
	}
	if parsed.ID != "card-1" {
		t.Errorf("id after rewrite = %q, want %q", parsed.ID, "card-1")
	}
}

// TestPersistWritesLogBeforeFrontmatter verifies that Persist writes the
// review log entry and updates the card file with the new FSRS state.
func TestPersistWritesLogBeforeFrontmatter(t *testing.T) {
	cardDir := t.TempDir()
	cardPath := filepath.Join(cardDir, "test.md")

	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			ID:     "card-1",
			Type:   card.Basic,
		},
		Front: "Q\n",
		Back:  "A\n",
	}
	os.WriteFile(cardPath, c.Serialize(), 0o644)

	stateDir := t.TempDir()
	s := store.NewStore(stateDir, "mydeck")

	entry := store.LogEntry{
		Schema: 1,
		TS:     time.Now().UTC().Truncate(time.Millisecond),
		CardID: "card-1",
		Rating: 3,
		Prev:   fsrs.CardState{State: fsrs.StateNew},
		Next:   fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5},
	}

	c.State = string(entry.Next.State)
	c.Stability = entry.Next.Stability

	if err := s.Persist(entry, cardPath, c); err != nil {
		t.Fatalf("Persist() error: %v", err)
	}

	logData, err := os.ReadFile(filepath.Join(stateDir, "mydeck.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if len(logData) == 0 {
		t.Fatal("jsonl is empty after persist")
	}

	parsed, err := card.ParseFile(cardPath)
	if err != nil {
		t.Fatalf("ParseFile after persist: %v", err)
	}
	if parsed.State != "learning" {
		t.Errorf("state after persist = %q, want %q", parsed.State, "learning")
	}
}

// TestEnsureIDAssignsUUIDv7WhenCardLacksID confirms that EnsureID generates
// a UUID v7 and returns true when the card has no ID.
func TestEnsureIDAssignsUUIDv7WhenCardLacksID(t *testing.T) {
	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			Type:   card.Basic,
		},
		Front: "Q\n",
		Back:  "A\n",
	}

	assigned := store.EnsureID(c)
	if c.ID == "" {
		t.Error("ID should be assigned")
	}
	if !assigned {
		t.Error("EnsureID should return true when ID was assigned")
	}
	if len(c.ID) < 20 {
		t.Errorf("UUID v7 too short: %q", c.ID)
	}
}

// TestEnsureIDNoOpWhenCardHasID ensures that EnsureID is a no-op and
// returns false when the card already has an ID.
func TestEnsureIDNoOpWhenCardHasID(t *testing.T) {
	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			ID:     "existing-id",
			Type:   card.Basic,
		},
		Front: "Q\n",
		Back:  "A\n",
	}

	assigned := store.EnsureID(c)
	if assigned {
		t.Error("EnsureID should return false when ID already exists")
	}
	if c.ID != "existing-id" {
		t.Errorf("ID changed from %q to %q", "existing-id", c.ID)
	}
}
