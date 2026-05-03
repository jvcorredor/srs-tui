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
	return func(it *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error) {
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
			return fsrs.CardState{}, nil, err
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
			return nextState, previews, fmt.Errorf("persist: %w", err)
		}

		return nextState, previews, nil
	}
}

// defaultReviewRun builds the review queue for deckDir, opens the interactive
// Bubble Tea review session, and persists ratings via MakeRateFunc.
func defaultReviewRun(deckDir string) error {
	items, err := deck.BuildQueue(deckDir)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	deckSlug := filepath.Base(deckDir)
	stateDir := filepath.Join(paths.StateHome(), "srs")
	s := store.NewStore(stateDir, deckSlug)
	rateFunc := MakeRateFunc(s)

	m := tui.NewReviewModel(items, rateFunc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// NewRootCmd creates the root "srs" cobra command and attaches all subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "srs",
		Short: "Spaced repetition in the terminal",
	}
	root.SetOut(rootOut)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newReviewCmd())
	root.AddCommand(newNewCmd())
	root.AddCommand(newInitCmd())
	return root
}

// newReviewCmd creates the "review <deck>" command.
func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <deck>",
		Short: "Review a deck of flashcards",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deckName := args[0]
			decksRoot := paths.DecksRoot("")
			deckDir := filepath.Join(decksRoot, deckName)
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
				fmt.Fprintf(cmd.OutOrStdout(), "srs %s\ncommit: %s\ndate:   %s\n", info.Version, info.Commit, info.Date)
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

// RunInit scaffolds the default config.toml in configDir and the decks directory
// in dataDir. If config.toml already exists and force is false, it prints a
// warning to stderr and returns an error.
func RunInit(configDir, dataDir string, force bool, stdout, stderr io.Writer) error {
	srsConfigDir := filepath.Join(configDir, "srs")
	configPath := filepath.Join(srsConfigDir, "config.toml")

	if _, err := os.Stat(configPath); err == nil && !force {
		fmt.Fprintf(stderr, "config.toml already exists; use --force to overwrite\n")
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

	fmt.Fprintf(stdout, "Created %s\nCreated %s\n", configPath, decksRoot)
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
		return 1
	}
	return 0
}
