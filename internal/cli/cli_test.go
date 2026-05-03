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

func TestRunInitWritesDefaultConfig(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}

	configPath := filepath.Join(configDir, "srs", "config.toml")
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}

	content := string(b)
	if !strings.Contains(content, "decks_root") {
		t.Error("config template missing decks_root")
	}
	if !strings.Contains(content, "new_per_day") {
		t.Error("config template missing new_per_day")
	}
	if !strings.Contains(content, "command") {
		t.Error("config template missing editor command")
	}
	if !strings.Contains(content, "style") {
		t.Error("config template missing render style")
	}
}

func TestRunInitCreatesDecksRoot(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}

	decksRoot := filepath.Join(dataDir, "srs", "decks")
	info, err := os.Stat(decksRoot)
	if err != nil {
		t.Fatalf("stat decks_root: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("decks_root %q is not a directory", decksRoot)
	}
}

func TestRunInitRefusesOverwriteWithoutForce(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr); err != nil {
		t.Fatalf("first RunInit() error: %v", err)
	}

	var stdout2, stderr2 bytes.Buffer
	err := cli.RunInit(configDir, dataDir, false, &stdout2, &stderr2)
	if err == nil {
		t.Fatal("expected error when config already exists without --force")
	}

	if !strings.Contains(stderr2.String(), "already exists") {
		t.Errorf("stderr = %q, want mention of already exists", stderr2.String())
	}
}

func TestRunInitOverwritesWithForce(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr); err != nil {
		t.Fatalf("first RunInit() error: %v", err)
	}

	configPath := filepath.Join(configDir, "srs", "config.toml")
	original, _ := os.ReadFile(configPath)

	os.WriteFile(configPath, append(original, []byte("# extra\n")...), 0o644)

	var stdout2, stderr2 bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, true, &stdout2, &stderr2); err != nil {
		t.Fatalf("RunInit with force: %v", err)
	}

	after, _ := os.ReadFile(configPath)
	if strings.Contains(string(after), "# extra") {
		t.Error("force should overwrite with default template, but old content remains")
	}
}

func TestRunInitPrintsSuccessSummary(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr); err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}

	out := stdout.String()
	configPath := filepath.Join(configDir, "srs", "config.toml")
	if !strings.Contains(out, configPath) {
		t.Errorf("stdout missing config path %q: got %q", configPath, out)
	}
	decksRoot := filepath.Join(dataDir, "srs", "decks")
	if !strings.Contains(out, decksRoot) {
		t.Errorf("stdout missing decks_root %q: got %q", decksRoot, out)
	}
	if stderr.Len() > 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}
}

func TestRunInitIdempotentWithForce(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout1, stderr1 bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, false, &stdout1, &stderr1); err != nil {
		t.Fatalf("first RunInit(): %v", err)
	}

	firstConfig, _ := os.ReadFile(filepath.Join(configDir, "srs", "config.toml"))

	var stdout2, stderr2 bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, true, &stdout2, &stderr2); err != nil {
		t.Fatalf("second RunInit() with force: %v", err)
	}

	secondConfig, _ := os.ReadFile(filepath.Join(configDir, "srs", "config.toml"))

	if string(firstConfig) != string(secondConfig) {
		t.Error("config content differs between first and second run with --force")
	}
}

func TestInitSubcommandCreatesFiles(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", configDir)
	os.Setenv("XDG_DATA_HOME", dataDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	defer os.Unsetenv("XDG_DATA_HOME")

	buf := new(bytes.Buffer)
	cli.SetOutput(buf)
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"init"})
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	configPath := filepath.Join(configDir, "srs", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config.toml not created")
	}
	decksRoot := filepath.Join(dataDir, "srs", "decks")
	if _, err := os.Stat(decksRoot); os.IsNotExist(err) {
		t.Error("decks_root directory not created")
	}
}
