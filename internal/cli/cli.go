// Package cli implements the command-line interface and subcommands.
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

type UsageError struct {
	msg string
}

func (e *UsageError) Error() string { return e.msg }

func SetOutput(w io.Writer) {
	rootOut = w
}

var rootOut io.Writer

type ReviewRunFunc func(deckDir string) error

var reviewRun ReviewRunFunc = defaultReviewRun

func SetReviewRun(fn ReviewRunFunc) {
	reviewRun = fn
}

type EditorRunFunc func(file string) error

var editorRun EditorRunFunc = defaultEditorRun

func SetEditorRun(fn EditorRunFunc) {
	editorRun = fn
}

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

func MakeRateFunc(s *store.Store) tui.RateFunc {
	return func(c *card.Card, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error) {
		prevState := fsrs.CardState{
			State:      fsrs.NormalizeState(c.State),
			Due:       fsrs.ParseTime(c.Due),
			Stability:  c.Stability,
			Difficulty: c.Difficulty,
			Reps:      c.Reps,
			Lapses:    c.Lapses,
		}

		nextState, previews, err := fsrs.Rate(prevState, rating, now)
		if err != nil {
			return fsrs.CardState{}, nil, err
		}

		store.EnsureID(c)

		entry := store.LogEntry{
			Schema: 1,
			TS:      now,
			CardID:  c.ID,
			Rating:  rating,
			Prev:    prevState,
			Next:    nextState,
		}

		c.State = string(nextState.State)
		c.Due = nextState.Due.Format(time.RFC3339)
		c.Stability = nextState.Stability
		c.Difficulty = nextState.Difficulty
		c.Reps = nextState.Reps
		c.Lapses = nextState.Lapses

		if err := s.Persist(entry, c.FilePath, c); err != nil {
			return nextState, previews, fmt.Errorf("persist: %w", err)
		}

		return nextState, previews, nil
	}
}

func defaultReviewRun(deckDir string) error {
	cards, err := deck.BuildQueue(deckDir)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	deckSlug := filepath.Base(deckDir)
	stateDir := filepath.Join(paths.StateHome(), "srs")
	s := store.NewStore(stateDir, deckSlug)
	rateFunc := MakeRateFunc(s)

	m := tui.NewReviewModel(cards, rateFunc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

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

func Execute() int {
	return ExecuteWithArgs(nil)
}

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
