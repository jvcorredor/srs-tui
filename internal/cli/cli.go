package cli

import (
	"fmt"
	"io"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/paths"
	"github.com/jvcorredor/srs-tui/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func SetVersion(v, c, d string) {
	version = v
	commit = c
	date = d
}

func SetOutput(w io.Writer) {
	rootOut = w
}

var rootOut io.Writer

type ReviewRunFunc func(deckDir string) error

var reviewRun ReviewRunFunc = defaultReviewRun

func SetReviewRun(fn ReviewRunFunc) {
	reviewRun = fn
}

func defaultReviewRun(deckDir string) error {
	cards, err := deck.BuildQueue(deckDir)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}
	m := tui.NewReviewModel(cards)
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

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "srs %s\ncommit: %s\ndate:   %s\n", version, commit, date)
			return nil
		},
	}
}

func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		return 1
	}
	return 0
}
