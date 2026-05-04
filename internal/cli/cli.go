// Package cli implements the command-line interface for srs-tui.
// It defines the cobra commands (review, new, version, init) and
// the glue functions that wire the terminal UI, card storage, and
// scheduling logic together.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/config"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/paths"
	"github.com/jvcorredor/srs-tui/internal/store"
	"github.com/jvcorredor/srs-tui/internal/tui"
	"github.com/jvcorredor/srs-tui/internal/version"
)

// UsageError signals a CLI usage mistake (wrong arguments, missing flags, etc).
// ExecuteWithArgs returns exit code 2 when the error chain contains a UsageError.
type UsageError struct {
	msg string
}

// Error returns the usage error message.
func (e *UsageError) Error() string { return e.msg }

// SetOutput redirects the root command's stdout/stderr to w for testing.
func SetOutput(w io.Writer) {
	rootOut = w
}

var rootOut io.Writer

// ReviewRunFunc is the function used by the review command to start a review session.
// It is swapped out in tests to avoid launching the interactive TUI.
type ReviewRunFunc func(deckDir string) error

var reviewRun ReviewRunFunc = defaultReviewRun

// SetReviewRun replaces the default review runner with fn (used in tests).
func SetReviewRun(fn ReviewRunFunc) {
	reviewRun = fn
}

// EditorRunFunc is the function used by the new command to open a card in an editor.
// It is swapped out in tests to avoid launching an external editor.
type EditorRunFunc func(file string) error

var editorRun EditorRunFunc = defaultEditorRun

// SetEditorRun replaces the default editor runner with fn (used in tests).
func SetEditorRun(fn EditorRunFunc) {
	editorRun = fn
}

// PickerRunFunc is the function used to launch the deck picker TUI.
// It is swapped out in tests to avoid launching the interactive TUI.
type PickerRunFunc func(decksRoot string) error

var pickerRun PickerRunFunc = defaultPickerRun

// SetPickerRun replaces the default picker runner with fn (used in tests).
func SetPickerRun(fn PickerRunFunc) {
	pickerRun = fn
}

// defaultEditorRun opens file in the editor defined by the EDITOR environment
// variable, falling back to vi if EDITOR is not set.
func defaultEditorRun(file string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MakeRateFunc builds a tui.RateFunc that rates a review item using the FSRS
// algorithm, persists the resulting state to the store's JSONL log and the
// card's Markdown file, and returns the next state together with interval
// previews. For cloze cards, only the active group's state is updated.
func MakeRateFunc(s *store.Store) tui.RateFunc {
	return func(it *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, store.LogEntry, error) {
		var prevState fsrs.CardState
		if it.Card.Type == card.Cloze && it.ClozeGroup != "" {
			if g, ok := it.Card.Clozes[it.ClozeGroup]; ok {
				prevState = fsrs.CardState{
					State:      fsrs.NormalizeState(g.State),
					Due:        fsrs.ParseTime(g.Due),
					Stability:  g.Stability,
					Difficulty: g.Difficulty,
					Reps:       g.Reps,
					Lapses:     g.Lapses,
				}
			}
		} else {
			prevState = fsrs.CardState{
				State:      fsrs.NormalizeState(it.Card.State),
				Due:        fsrs.ParseTime(it.Card.Due),
				Stability:  it.Card.Stability,
				Difficulty: it.Card.Difficulty,
				Reps:       it.Card.Reps,
				Lapses:     it.Card.Lapses,
			}
		}

		nextState, previews, err := fsrs.Rate(prevState, rating, now)
		if err != nil {
			return fsrs.CardState{}, nil, store.LogEntry{}, err
		}

		store.EnsureID(it.Card)

		entry := store.LogEntry{
			Schema: 1,
			TS:     now,
			CardID: it.Card.ID,
			Rating: rating,
			Prev:   prevState,
			Next:   nextState,
		}
		if it.ClozeGroup != "" {
			entry.ClozeGroup = &it.ClozeGroup
		}

		if it.Card.Type == card.Cloze && it.ClozeGroup != "" {
			g := it.Card.Clozes[it.ClozeGroup]
			g.State = string(nextState.State)
			g.Due = nextState.Due.Format(time.RFC3339)
			g.Stability = nextState.Stability
			g.Difficulty = nextState.Difficulty
			g.Reps = nextState.Reps
			g.Lapses = nextState.Lapses
			it.Card.Clozes[it.ClozeGroup] = g
		} else {
			it.Card.State = string(nextState.State)
			it.Card.Due = nextState.Due.Format(time.RFC3339)
			it.Card.Stability = nextState.Stability
			it.Card.Difficulty = nextState.Difficulty
			it.Card.Reps = nextState.Reps
			it.Card.Lapses = nextState.Lapses
		}

		if err := s.Persist(entry, it.Card.FilePath, it.Card); err != nil {
			return nextState, previews, entry, fmt.Errorf("persist: %w", err)
		}

		return nextState, previews, entry, nil
	}
}

// MakeUndoFunc builds a tui.UndoFunc that reverses the last rating by truncating
// the JSONL log and rewriting the card file to its prior FSRS state.
func MakeUndoFunc(s *store.Store) tui.UndoFunc {
	return func(entry store.LogEntry, cardPath string, c *card.Card) error {
		if err := s.TruncateLastLog(entry); err != nil {
			return fmt.Errorf("undo: truncate log: %w", err)
		}
		if err := s.RewriteCard(cardPath, c); err != nil {
			return fmt.Errorf("undo: rewrite card: %w", err)
		}
		return nil
	}
}

// defaultReviewRun builds the review queue for deckDir, opens the interactive
// Bubble Tea review session, and persists ratings via MakeRateFunc.
func loadConfig() (*config.Config, error) {
	cfg, warnings, err := config.Load(paths.ConfigHome())
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = config.Defaults()
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	return cfg, nil
}

func defaultReviewRun(deckDir string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	now := time.Now()
	deckSlug := filepath.Base(deckDir)
	stateDir := filepath.Join(paths.StateHome(), "srs")
	s := store.NewStore(stateDir, deckSlug)

	items, err := deck.BuildQueue(deckDir, deck.QueueConfig{
		NewPerDay: cfg.Review.NewPerDay,
		Now:       now,
		NewCount:  s.NewCountToday,
	})
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	rateFunc := MakeRateFunc(s)
	undoFunc := MakeUndoFunc(s)

	m := tui.NewReviewModel(items, rateFunc,
		tui.WithEditorCmd(tui.EditorExecCmd),
		tui.WithUndoFunc(undoFunc),
		tui.WithRenderStyle(cfg.Render.Style),
	)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// defaultPickerRun discovers decks under decksRoot, builds a picker model,
// and launches the interactive TUI. When the user selects a deck, it
// transitions directly into the review session without restarting.
func defaultPickerRun(decksRoot string) error {
	deckPaths, err := deck.Discover(decksRoot)
	if err != nil {
		return fmt.Errorf("picker: %w", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	now := time.Now()
	var entries []tui.DeckEntry
	for _, dp := range deckPaths {
		count, _ := deck.DueCount(dp, now)
		entries = append(entries, tui.DeckEntry{
			Name:     filepath.Base(dp),
			Path:     dp,
			DueCount: count,
		})
	}

	stateDir := filepath.Join(paths.StateHome(), "srs")
	onSelect := func(e tui.DeckEntry) (tea.Model, tea.Cmd) {
		deckSlug := filepath.Base(e.Path)
		s := store.NewStore(stateDir, deckSlug)
		items, qErr := deck.BuildQueue(e.Path, deck.QueueConfig{
			NewPerDay: cfg.Review.NewPerDay,
			Now:       now,
			NewCount:  s.NewCountToday,
		})
		if qErr != nil {
			return nil, tea.Quit
		}
		rateFunc := MakeRateFunc(s)
		undoFunc := MakeUndoFunc(s)
		return tui.NewReviewModel(items, rateFunc,
			tui.WithEditorCmd(tui.EditorExecCmd),
			tui.WithUndoFunc(undoFunc),
			tui.WithRenderStyle(cfg.Render.Style),
		), nil
	}

	m := tui.NewPickerModel(entries, onSelect)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// NewRootCmd creates the root "srs" cobra command and attaches all subcommands.
// When invoked with no arguments, it launches the deck picker.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "srs",
		Short: "Spaced repetition in the terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			decksRoot := paths.DecksRoot("")
			return pickerRun(decksRoot)
		},
	}
	root.SetOut(rootOut)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newReviewCmd())
	root.AddCommand(newNewCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newDecksCmd())
	return root
}

// newReviewCmd creates the "review [deck]" command. With no deck argument,
// it launches the picker; with a deck name, it goes straight to review.
func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review [deck]",
		Short: "Review a deck of flashcards",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			decksRoot := paths.DecksRoot("")
			if len(args) == 0 {
				return pickerRun(decksRoot)
			}
			deckDir := filepath.Join(decksRoot, args[0])
			return reviewRun(deckDir)
		},
	}
}

// newNewCmd creates the "new <deck> <name>" command for adding cards.
func newNewCmd() *cobra.Command {
	var cloze bool
	var decksRoot string

	cmd := &cobra.Command{
		Use:   "new <deck> <name>",
		Short: "Create a new card and open it in your editor",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return &UsageError{msg: fmt.Sprintf("accepts 2 arg(s), received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			deckName := args[0]
			cardName := args[1]

			root := paths.DecksRoot(decksRoot)
			deckDir := filepath.Join(root, deckName)
			cardPath := filepath.Join(deckDir, cardName+".md")

			if _, err := os.Stat(cardPath); err == nil {
				return fmt.Errorf("new: %s already exists", cardPath)
			}

			if err := os.MkdirAll(deckDir, 0o755); err != nil {
				return fmt.Errorf("new: create deck dir: %w", err)
			}

			cardType := card.Basic
			if cloze {
				cardType = card.Cloze
			}
			c := card.NewCard(cardType, time.Now())

			if err := store.AtomicWriteFile(cardPath, c.SerializeNew()); err != nil {
				return fmt.Errorf("new: %w", err)
			}

			return editorRun(cardPath)
		},
	}

	cmd.Flags().BoolVar(&cloze, "cloze", false, "create a cloze-deletion card")
	cmd.Flags().StringVar(&decksRoot, "decks-root", "", "root directory for decks")

	return cmd
}

// newVersionCmd creates the "version" command.
func newVersionCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := version.Get()
			switch format {
			case "text":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "srs %s\ncommit: %s\ndate:   %s\n", info.Version, info.Commit, info.Date)
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(info)
			default:
				return &UsageError{msg: fmt.Sprintf("--format: must be \"text\" or \"json\", got %q", format)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", `output format: "text" or "json"`)
	return cmd
}

// newInitCmd creates the "init" command that scaffolds config and decks directories.
func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold default config and decks directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunInit(
				paths.ConfigHome(),
				paths.DataHome(),
				force,
				cmd.OutOrStdout(),
				cmd.OutOrStderr(),
			)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config.toml")
	return cmd
}

func newDecksCmd() *cobra.Command {
	var decksRoot string
	cmd := &cobra.Command{
		Use:   "decks",
		Short: "List deck names",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDecks(paths.DecksRoot(decksRoot), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&decksRoot, "decks-root", "", "root directory for decks")
	return cmd
}

// RunDecks discovers decks under decksRoot and prints their names sorted
// alphabetically, one per line, to stdout. Diagnostics go to stderr.
func RunDecks(decksRoot string, stdout, stderr io.Writer) error {
	paths, err := deck.Discover(decksRoot)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "decks: %v\n", err)
		return err
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	sort.Strings(names)
	for _, n := range names {
		_, _ = fmt.Fprintln(stdout, n)
	}
	return nil
}

// RunInit scaffolds the default config.toml in configDir and the decks directory
// in dataDir. If config.toml already exists and force is false, it prints a
// warning to stderr and returns an error.
func RunInit(configDir, dataDir string, force bool, stdout, stderr io.Writer) error {
	srsConfigDir := filepath.Join(configDir, "srs")
	configPath := filepath.Join(srsConfigDir, "config.toml")

	if _, err := os.Stat(configPath); err == nil && !force {
		_, _ = fmt.Fprintf(stderr, "config.toml already exists; use --force to overwrite\n")
		return fmt.Errorf("config.toml already exists")
	}

	if err := os.MkdirAll(srsConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(config.DefaultConfigContent()), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	decksRoot := filepath.Join(dataDir, "srs", "decks")
	if err := os.MkdirAll(decksRoot, 0o755); err != nil {
		return fmt.Errorf("create decks root: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "Created %s\nCreated %s\n", configPath, decksRoot)
	return nil
}

// Execute runs the root command with os.Args and returns an exit code.
func Execute() int {
	return ExecuteWithArgs(nil)
}

// ExecuteWithArgs runs the root command with the provided args (nil means os.Args)
// and returns an exit code: 0 success, 1 runtime error, 2 usage error.
func ExecuteWithArgs(args []string) int {
	root := NewRootCmd()
	if args != nil {
		root.SetArgs(args)
	}
	root.SetOut(rootOut)
	root.SetErr(rootOut)
	err := root.Execute()
	if err != nil {
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			return 2
		}
		var fieldErr config.FieldError
		if errors.As(err, &fieldErr) {
			_, _ = fmt.Fprintln(os.Stderr, fieldErr.Error())
			return 2
		}
		return 1
	}
	return 0
}
