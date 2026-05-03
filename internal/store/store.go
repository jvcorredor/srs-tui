package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

type LogEntry struct {
	Schema     int            `json:"schema"`
	TS         time.Time      `json:"ts"`
	CardID     string         `json:"card_id"`
	ClozeGroup *int           `json:"cloze_group,omitempty"`
	Rating     int            `json:"rating"`
	DurationMs int64          `json:"duration_ms"`
	Prev       fsrs.CardState `json:"prev"`
	Next       fsrs.CardState `json:"next"`
}

type Store struct {
	stateDir string
	deckSlug string
	logPath  string
}

func NewStore(stateDir, deckSlug string) *Store {
	return &Store{
		stateDir: stateDir,
		deckSlug: deckSlug,
		logPath:  filepath.Join(stateDir, deckSlug+".jsonl"),
	}
}

func (s *Store) AppendLog(entry LogEntry) error {
	if err := os.MkdirAll(s.stateDir, 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

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

func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("atomic write: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("atomic write: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic write: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic write: rename temp: %w", err)
	}
	return nil
}

func (s *Store) RewriteCard(cardPath string, c *card.Card) error {
	return AtomicWriteFile(cardPath, c.Serialize())
}

func (s *Store) Persist(entry LogEntry, cardPath string, c *card.Card) error {
	if err := s.AppendLog(entry); err != nil {
		return fmt.Errorf("store: persist log: %w", err)
	}
	if err := s.RewriteCard(cardPath, c); err != nil {
		return fmt.Errorf("store: persist card: %w", err)
	}
	return nil
}

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

func (s *Store) StateDir() string { return s.stateDir }
func (s *Store) DeckSlug() string { return s.deckSlug }
func (s *Store) LogPath() string  { return s.logPath }
