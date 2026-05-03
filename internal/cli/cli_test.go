package cli_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/cli"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/store"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	cli.SetOutput(buf)
	cli.SetVersion("0.0.0-dev", "abc1234", "2026-01-01")

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"version"})
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.0.0-dev") {
		t.Errorf("version output missing version string: got %q", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("version output missing commit: got %q", out)
	}
	if !strings.Contains(out, "2026-01-01") {
		t.Errorf("version output missing date: got %q", out)
	}
}

func TestExecuteReturnsZero(t *testing.T) {
	cli.SetOutput(io.Discard)
	code := cli.Execute()
	if code != 0 {
		t.Errorf("Execute() = %d, want 0", code)
	}
}

func TestReviewCommandRequiresDeckArg(t *testing.T) {
	buf := new(bytes.Buffer)
	cli.SetOutput(buf)
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"review"})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when deck arg missing")
	}
}

func TestReviewCommandAcceptsDeckArg(t *testing.T) {
	cli.SetOutput(io.Discard)
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"review", "french"})
	cmd.SetOut(io.Discard)
	fakeRun := func(deckDir string) error { return nil }
	cli.SetReviewRun(fakeRun)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMakeRateFuncPersistsRating(t *testing.T) {
	cardDir := t.TempDir()
	stateDir := t.TempDir()

	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			ID:     "card-1",
			Type:   card.Basic,
		},
		Front:    "Q\n",
		Back:     "A\n",
		FilePath: filepath.Join(cardDir, "card-1.md"),
	}
	os.WriteFile(c.FilePath, c.Serialize(), 0o644)

	s := store.NewStore(stateDir, "testdeck")
	rateFunc := cli.MakeRateFunc(s)

	now := time.Now()
	nextState, previews, err := rateFunc(c, 3, now)
	if err != nil {
		t.Fatalf("rateFunc() error: %v", err)
	}
	if nextState.State == "" {
		t.Error("next state should not be empty")
	}
	if len(previews) != 4 {
		t.Errorf("previews length = %d, want 4", len(previews))
	}

	logPath := filepath.Join(stateDir, "testdeck.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var entry store.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal line %d: %v", lineCount, err)
		}
		if entry.CardID != "card-1" {
			t.Errorf("card_id = %q, want %q", entry.CardID, "card-1")
		}
		if entry.Rating != 3 {
			t.Errorf("rating = %d, want 3", entry.Rating)
		}
		if entry.Prev.State != fsrs.StateNew {
			t.Errorf("prev.state = %q, want %q", entry.Prev.State, fsrs.StateNew)
		}
	}
	if lineCount != 1 {
		t.Errorf("log line count = %d, want 1", lineCount)
	}

	parsed, err := card.ParseFile(c.FilePath)
	if err != nil {
		t.Fatalf("ParseFile after rate: %v", err)
	}
	if parsed.State == "" {
		t.Error("card state should be set after rating")
	}
	if parsed.Stability <= 0 {
		t.Error("card stability should be positive after rating")
	}
}

func TestNewCommandCreatesCardFileWithPrefilledFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	cardPath := filepath.Join(tmpDir, "french", "bonjour.md")
	c, err := card.ParseFile(cardPath)
	if err != nil {
		t.Fatalf("parse created card: %v", err)
	}
	if c.Schema != 1 {
		t.Errorf("schema = %d, want 1", c.Schema)
	}
	if c.ID == "" {
		t.Error("id should not be empty")
	}
	if c.Type != card.Basic {
		t.Errorf("type = %q, want %q", c.Type, card.Basic)
	}
	if c.Created == "" {
		t.Error("created should not be empty")
	}
	if len(c.Tags) != 0 {
		t.Errorf("tags = %v, want empty", c.Tags)
	}
}

func TestNewCommandWithClozeFlagCreatesClozeCard(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "med", "tibia", "--cloze", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	cardPath := filepath.Join(tmpDir, "med", "tibia.md")
	c, err := card.ParseFile(cardPath)
	if err != nil {
		t.Fatalf("parse created card: %v", err)
	}
	if c.Type != card.Cloze {
		t.Errorf("type = %q, want %q", c.Type, card.Cloze)
	}

	raw, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("read card file: %v", err)
	}
	if !strings.Contains(string(raw), "{{c1::") {
		t.Error("cloze card body should contain cloze syntax hint")
	}
}

func TestNewCommandRefusesOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first new command failed: %v", err)
	}

	cmd2 := cli.NewRootCmd()
	cmd2.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd2.SetOut(io.Discard)
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error when file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}

func TestNewCommandLaunchesEditor(t *testing.T) {
	tmpDir := t.TempDir()
	var editorCalledWith string
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(file string) error {
		editorCalledWith = file
		return nil
	})

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	cardPath := filepath.Join(tmpDir, "french", "bonjour.md")
	if editorCalledWith != cardPath {
		t.Errorf("editor called with %q, want %q", editorCalledWith, cardPath)
	}
}

func TestNewCommandCreatesDeckDirectoryIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "brand-new-deck", "card1", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	deckDir := filepath.Join(tmpDir, "brand-new-deck")
	info, err := os.Stat(deckDir)
	if err != nil {
		t.Fatalf("deck directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("deck path is not a directory")
	}

	cardPath := filepath.Join(deckDir, "card1.md")
	if _, err := os.Stat(cardPath); err != nil {
		t.Fatalf("card file not created: %v", err)
	}
}

func TestNewCommandAtomicWriteNoTmpArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	deckDir := filepath.Join(tmpDir, "french")
	entries, err := os.ReadDir(deckDir)
	if err != nil {
		t.Fatalf("read deck dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp artifact left behind: %s", e.Name())
		}
	}
}

func TestNewCommandUsageErrorReturnsExitCode2(t *testing.T) {
	cli.SetOutput(io.Discard)
	code := cli.ExecuteWithArgs([]string{"new"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 for usage error", code)
	}
}

func TestNewCommandRuntimeErrorReturnsExitCode1(t *testing.T) {
	tmpDir := t.TempDir()
	cli.SetOutput(io.Discard)
	cli.SetEditorRun(func(_ string) error { return nil })

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first new command failed: %v", err)
	}

	code := cli.ExecuteWithArgs([]string{"new", "french", "bonjour", "--decks-root", tmpDir})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for runtime error (file exists)", code)
	}
}

func TestMakeRateFuncAssignsID(t *testing.T) {
	cardDir := t.TempDir()
	stateDir := t.TempDir()

	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			Type:   card.Basic,
		},
		Front:    "Q\n",
		Back:     "A\n",
		FilePath: filepath.Join(cardDir, "noid.md"),
	}
	os.WriteFile(c.FilePath, c.Serialize(), 0o644)

	s := store.NewStore(stateDir, "testdeck")
	rateFunc := cli.MakeRateFunc(s)

	_, _, err := rateFunc(c, 3, time.Now())
	if err != nil {
		t.Fatalf("rateFunc() error: %v", err)
	}
	if c.ID == "" {
		t.Error("card should have ID assigned after first rating")
	}
}
