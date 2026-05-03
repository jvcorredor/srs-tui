package cli_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/cli"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/store"
	"github.com/jvcorredor/srs-tui/internal/version"
)

// noBuildInfo returns nil to simulate a binary built without debug info.
func noBuildInfo() (*debug.BuildInfo, bool) { return nil, false }

// TestVersionCommandPrintsVersion checks that the version subcommand prints the
// version string, commit hash, and build date in text format.
func TestVersionCommandPrintsVersion(t *testing.T) {
	defer version.SwapForTest("0.0.0-dev", "abc1234", "2026-01-01", noBuildInfo)()

	buf := new(bytes.Buffer)
	cli.SetOutput(buf)

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

// TestVersionCommandJSONFormat verifies that version --format=json outputs
// valid JSON matching the version.Info struct.
func TestVersionCommandJSONFormat(t *testing.T) {
	defer version.SwapForTest("v0.2.0", "abc1234", "2026-05-03T12:00:00Z", noBuildInfo)()

	buf := new(bytes.Buffer)
	cli.SetOutput(buf)

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"version", "--format=json"})
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version --format=json failed: %v", err)
	}

	var got version.Info
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%q", err, buf.String())
	}

	want := version.Info{Version: "v0.2.0", Commit: "abc1234", Date: "2026-05-03T12:00:00Z", Source: "ldflags"}
	if got != want {
		t.Errorf("Info = %+v, want %+v", got, want)
	}
}

// TestVersionCommandRejectsUnknownFormat checks that version fails when given
// an unsupported --format value.
func TestVersionCommandRejectsUnknownFormat(t *testing.T) {
	defer version.SwapForTest("v0.2.0", "abc", "2026-05-03", noBuildInfo)()

	buf := new(bytes.Buffer)
	cli.SetOutput(buf)

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"version", "--format=yaml"})
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown --format value")
	}
}

// TestExecuteReturnsZero verifies that Execute returns 0 when no subcommand
// is given (the root command launches the deck picker).
func TestExecuteReturnsZero(t *testing.T) {
	cli.SetOutput(io.Discard)
	fakePickerRun := func(decksRoot string) error { return nil }
	cli.SetPickerRun(fakePickerRun)
	code := cli.Execute()
	if code != 0 {
		t.Errorf("Execute() = %d, want 0", code)
	}
}

// TestReviewCommandWithoutDeckLaunchesPicker verifies that review with no
// deck argument launches the picker instead of failing.
func TestReviewCommandWithoutDeckLaunchesPicker(t *testing.T) {
	cli.SetOutput(io.Discard)
	var pickerCalledWith string
	cli.SetPickerRun(func(decksRoot string) error {
		pickerCalledWith = decksRoot
		return nil
	})
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"review"})
	cmd.SetOut(io.Discard)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pickerCalledWith == "" {
		t.Error("picker should have been called with decks root")
	}
}

// TestReviewCommandAcceptsDeckArg verifies that review succeeds when a deck
// argument is provided.
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

// TestMakeRateFuncPersistsRating verifies that MakeRateFunc updates the card's
// FSRS state, writes the new state back to the card file, and appends a log entry
// to the store's JSONL file.
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
	if err := os.WriteFile(c.FilePath, c.Serialize(), 0o644); err != nil {
		t.Fatalf("write card file: %v", err)
	}

	s := store.NewStore(stateDir, "testdeck")
	rateFunc := cli.MakeRateFunc(s)

	now := time.Now()
	nextState, previews, err := rateFunc(&deck.ReviewItem{Card: c}, 3, now)
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
	defer func() { _ = f.Close() }()

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

// TestMakeRateFuncUpdatesOnlyActiveClozeGroup verifies that rating a cloze card
// updates only the active group's FSRS state, leaves other groups untouched,
// writes a JSONL log entry with the cloze_group field, and persists the card
// with per-group frontmatter.
func TestMakeRateFuncUpdatesOnlyActiveClozeGroup(t *testing.T) {
	cardDir := t.TempDir()
	stateDir := t.TempDir()

	c := &card.Card{
		Meta: card.Meta{
			Schema: 1,
			ID:     "cloze-1",
			Type:   card.Cloze,
			Clozes: map[string]card.ClozeGroup{
				"c1": {State: "new"},
				"c2": {State: "new"},
			},
		},
		Body:     "{{c1::A}} and {{c2::B}}\n",
		FilePath: filepath.Join(cardDir, "cloze-1.md"),
	}
	if err := os.WriteFile(c.FilePath, c.Serialize(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := store.NewStore(stateDir, "testdeck")
	rateFunc := cli.MakeRateFunc(s)

	now := time.Now()
	item := &deck.ReviewItem{Card: c, ClozeGroup: "c1"}
	nextState, _, err := rateFunc(item, 3, now)
	if err != nil {
		t.Fatalf("rateFunc() error: %v", err)
	}

	// c1 should be updated, c2 should remain "new"
	if c.Clozes["c1"].State == "" || c.Clozes["c1"].State == "new" {
		t.Errorf("c1 state should be updated, got %q", c.Clozes["c1"].State)
	}
	if c.Clozes["c2"].State != "new" {
		t.Errorf("c2 state should remain 'new', got %q", c.Clozes["c2"].State)
	}
	if c.Clozes["c2"].Stability != 0 {
		t.Errorf("c2 stability should remain 0, got %v", c.Clozes["c2"].Stability)
	}

	// Log should contain cloze_group "c1"
	logPath := filepath.Join(stateDir, "testdeck.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry store.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if entry.ClozeGroup == nil || *entry.ClozeGroup != "c1" {
			t.Errorf("cloze_group = %v, want %q", entry.ClozeGroup, "c1")
		}
		if entry.Next.State != nextState.State {
			t.Errorf("next.state = %q, want %q", entry.Next.State, nextState.State)
		}
	}

	// Card file should persist the per-group frontmatter
	parsed, err := card.ParseFile(c.FilePath)
	if err != nil {
		t.Fatalf("ParseFile after rate: %v", err)
	}
	if parsed.Clozes["c1"].State != c.Clozes["c1"].State {
		t.Errorf("roundtrip c1 state = %q, want %q", parsed.Clozes["c1"].State, c.Clozes["c1"].State)
	}
	if parsed.Clozes["c2"].State != "new" {
		t.Errorf("roundtrip c2 state = %q, want %q", parsed.Clozes["c2"].State, "new")
	}
}

// TestNewCommandCreatesCardFileWithPrefilledFrontmatter verifies that the new
// command creates a Markdown card file with the correct frontmatter fields.
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

// TestNewCommandWithClozeFlagCreatesClozeCard checks that the --cloze flag
// produces a card with type "cloze" and a cloze-deletion syntax hint.
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

// TestNewCommandRefusesOverwrite verifies that the new command fails when the
// target card file already exists.
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

// TestNewCommandLaunchesEditor checks that the new command invokes the editor
// runner with the path of the newly created card file.
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

// TestNewCommandCreatesDeckDirectoryIfMissing verifies that the new command
// creates the deck directory when it does not already exist.
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

// TestNewCommandAtomicWriteNoTmpArtifacts checks that the new command does not
// leave temporary files in the deck directory after creating a card.
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

// TestNewCommandUsageErrorReturnsExitCode2 verifies that ExecuteWithArgs returns
// exit code 2 when the new command is called with missing arguments.
func TestNewCommandUsageErrorReturnsExitCode2(t *testing.T) {
	cli.SetOutput(io.Discard)
	code := cli.ExecuteWithArgs([]string{"new"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 for usage error", code)
	}
}

// TestNewCommandRuntimeErrorReturnsExitCode1 verifies that ExecuteWithArgs returns
// exit code 1 when the new command encounters a runtime error (file exists).
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

// TestMakeRateFuncAssignsID checks that MakeRateFunc generates a card ID when
// the card does not already have one.
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
	if err := os.WriteFile(c.FilePath, c.Serialize(), 0o644); err != nil {
		t.Fatalf("write card file: %v", err)
	}

	s := store.NewStore(stateDir, "testdeck")
	rateFunc := cli.MakeRateFunc(s)

	_, _, err := rateFunc(&deck.ReviewItem{Card: c}, 3, time.Now())
	if err != nil {
		t.Fatalf("rateFunc() error: %v", err)
	}
	if c.ID == "" {
		t.Error("card should have ID assigned after first rating")
	}
}

// TestRunInitWritesDefaultConfig verifies that RunInit writes a config.toml
// containing the expected default settings.
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

// TestRunInitCreatesDecksRoot checks that RunInit creates the decks root
// directory.
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

// TestRunInitRefusesOverwriteWithoutForce verifies that RunInit fails when
// config.toml already exists and force is false.
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

// TestRunInitOverwritesWithForce checks that RunInit overwrites an existing
// config.toml when force is true.
func TestRunInitOverwritesWithForce(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, false, &stdout, &stderr); err != nil {
		t.Fatalf("first RunInit() error: %v", err)
	}

	configPath := filepath.Join(configDir, "srs", "config.toml")
	original, _ := os.ReadFile(configPath)

	if err := os.WriteFile(configPath, append(original, []byte("# extra\n")...), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var stdout2, stderr2 bytes.Buffer
	if err := cli.RunInit(configDir, dataDir, true, &stdout2, &stderr2); err != nil {
		t.Fatalf("RunInit with force: %v", err)
	}

	after, _ := os.ReadFile(configPath)
	if strings.Contains(string(after), "# extra") {
		t.Error("force should overwrite with default template, but old content remains")
	}
}

// TestRunInitPrintsSuccessSummary verifies that RunInit prints the paths of
// the created config file and decks directory to stdout.
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

// TestRunInitIdempotentWithForce checks that running RunInit twice with force
// produces the same config content both times.
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

// TestInitSubcommandCreatesFiles verifies that the init subcommand creates
// config.toml and the decks root directory using the standard XDG paths.
func TestInitSubcommandCreatesFiles(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", configDir); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("XDG_DATA_HOME", dataDir); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()
	defer func() {
		if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()

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
