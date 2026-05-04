// Package store persists review logs and card state for SRS decks.
// It writes JSONL review entries and performs atomic rewrites of
// Markdown card files so that crashes never leave data partially updated.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

// LogEntry is a single review event recorded as one JSON line.
type LogEntry struct {
	Schema int       `json:"schema"`
	TS     time.Time `json:"ts"`
	CardID string    `json:"card_id"`
	// ClozeGroup is the cloze deletion group key (e.g. "c1") when reviewing a
	// cloze card; nil for basic cards.
	ClozeGroup *string `json:"cloze_group,omitempty"`
	Rating     int     `json:"rating"`
	// DurationMs is reserved for future review-duration tracking; currently unused.
	DurationMs int64          `json:"duration_ms"`
	Prev       fsrs.CardState `json:"prev"`
	Next       fsrs.CardState `json:"next"`
}

// Store manages the on-disk state for one deck.
type Store struct {
	stateDir string
	deckSlug string
	logPath  string
}

// NewStore creates a Store that persists data under stateDir for deckSlug.
// Review logs are written to stateDir/deckSlug.jsonl.
func NewStore(stateDir, deckSlug string) *Store {
	return &Store{
		stateDir: stateDir,
		deckSlug: deckSlug,
		logPath:  filepath.Join(stateDir, deckSlug+".jsonl"),
	}
}

// AppendLog marshals entry as JSON and appends it to the review log
// file, creating the state directory and log file if necessary.
func (s *Store) AppendLog(entry LogEntry) error {
	if err := os.MkdirAll(s.stateDir, 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		return err
	}
	return f.Sync()
}

// AtomicWriteFile writes data to path by creating a temporary file in the
// same directory, syncing it, and renaming it over path. This guarantees
// that path never contains a partially written file.
func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic write: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic write: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic write: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic write: rename temp: %w", err)
	}
	return nil
}

// RewriteCard serializes c and atomically overwrites cardPath.
func (s *Store) RewriteCard(cardPath string, c *card.Card) error {
	return AtomicWriteFile(cardPath, c.Serialize())
}

// Persist records a review log entry and updates the on-disk card file.
// Both operations must succeed; the log is written before the card is rewritten.
func (s *Store) Persist(entry LogEntry, cardPath string, c *card.Card) error {
	if err := s.AppendLog(entry); err != nil {
		return fmt.Errorf("store: persist log: %w", err)
	}
	if err := s.RewriteCard(cardPath, c); err != nil {
		return fmt.Errorf("store: persist card: %w", err)
	}
	return nil
}

// EnsureID assigns a UUID v7 to c if it does not already have an ID.
// It returns true when an ID was assigned and false if the card already had one.
func EnsureID(c *card.Card) bool {
	if c.ID != "" {
		return false
	}
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.Must(uuid.NewV7())
	}
	c.ID = id.String()
	return true
}

// StateDir returns the directory where review logs are stored.
func (s *Store) StateDir() string { return s.stateDir }

// DeckSlug returns the identifier used for this deck's log file name.
func (s *Store) DeckSlug() string { return s.deckSlug }

// LogPath returns the full path to the JSONL review log file.
func (s *Store) LogPath() string { return s.logPath }

// NewCountToday counts log entries where the card was previously in the "new"
// state and the entry timestamp falls on the same local-calendar day as now.
// It reads the JSONL log file associated with this store's deck.
func (s *Store) NewCountToday(now time.Time) (int, error) {
	f, err := os.Open(s.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer func() { _ = f.Close() }()

	today := now.Local().Truncate(24 * time.Hour)
	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Prev.State != fsrs.StateNew {
			continue
		}
		if entry.TS.Local().Truncate(24 * time.Hour).Equal(today) {
			count++
		}
	}
	return count, scanner.Err()
}
