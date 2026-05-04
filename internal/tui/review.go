// Package tui implements the Bubble Tea review interface for spaced-repetition
// cards. A review session follows a three-state lifecycle:
//
//  1. Front — the question side is displayed.
//  2. Back — pressing Space or Enter reveals the answer and shows interval
//     previews (again, hard, good, easy) computed by the scheduler.
//  3. Rate — pressing a rating key (1–4) applies the score via RateFunc,
//     advances to the next card, and returns to the front state.
//
// When every card has been rated the session ends and View signals completion.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/store"
)

// RateFunc applies a user rating to a review item and returns the resulting
// state, interval previews for all possible ratings, and any error.
type RateFunc func(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, store.LogEntry, error)

// ReviewModel is a Bubble Tea model that drives a flash-card review session.
// It manages a deck of review items, tracks which side is visible, and
// coordinates with a RateFunc to schedule cards after each rating.
type ReviewModel struct {
	items        []deck.ReviewItem
	index        int
	showingBack  bool
	showingHelp  bool
	quitConfirm  bool
	renderer     *glamour.TermRenderer
	renderStyle  string
	rateFunc     RateFunc
	previews     []fsrs.IntervalPreview
	done         bool
	ratingCounts map[int]int
	skippedCount int
	editorCmd    EditorCmdFunc
	cardReadFunc func(string) (*card.Card, error)
	editErr      string
	undoFunc     UndoFunc
	undoable     *undoInfo
}

type undoInfo struct {
	entry    store.LogEntry
	cardPath string
	card     *card.Card
	rating   int
}

// EditorCmdFunc returns a tea.Cmd that opens the card at path in an editor.
type EditorCmdFunc func(path string) tea.Cmd

// UndoFunc reverses a persisted rating by truncating the log and rewriting
// the card to its prior FSRS state.
type UndoFunc func(entry store.LogEntry, cardPath string, c *card.Card) error

type modelOption func(*ReviewModel)

// WithEditorCmd sets the editor command function used when pressing 'e'.
func WithEditorCmd(fn EditorCmdFunc) modelOption {
	return func(m *ReviewModel) { m.editorCmd = fn }
}

// WithCardReadFunc sets the function used to re-read a card from disk after
// the editor exits.
func WithCardReadFunc(fn func(string) (*card.Card, error)) modelOption {
	return func(m *ReviewModel) { m.cardReadFunc = fn }
}

// WithUndoFunc sets the function used to reverse the last persisted rating.
func WithUndoFunc(fn UndoFunc) modelOption {
	return func(m *ReviewModel) { m.undoFunc = fn }
}

// WithRenderStyle sets the Glamour style used to render card bodies.
func WithRenderStyle(style string) modelOption {
	return func(m *ReviewModel) {
		if style != "" {
			m.renderStyle = style
		}
	}
}

// EditFinishedMsg is sent when the external editor finishes editing a card.
type EditFinishedMsg struct {
	Path string
	Err  error
}

// NewReviewModel creates a ReviewModel for the given review items. The
// rateFunc is invoked each time the user presses a rating key (1–4) while
// the back side is visible. Optional model options configure editor, undo,
// and render style.
func NewReviewModel(items []deck.ReviewItem, rateFunc RateFunc, opts ...modelOption) ReviewModel {
	m := ReviewModel{
		items:        items,
		renderStyle:  "auto",
		rateFunc:     rateFunc,
		ratingCounts: make(map[int]int),
	}
	for _, opt := range opts {
		opt(&m)
	}
	if m.editorCmd == nil {
		m.editorCmd = EditorExecCmd
	}
	if m.cardReadFunc == nil {
		m.cardReadFunc = card.ParseFile
	}
	r, _ := glamour.NewTermRenderer(glamour.WithStandardStyle(m.renderStyle))
	m.renderer = r
	return m
}

func findEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

// EditorExecCmd returns a tea.Cmd that opens the card at path in the system
// editor, suspending the TUI while the editor runs.
func EditorExecCmd(path string) tea.Cmd {
	editor := findEditor()
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditFinishedMsg{Path: path, Err: err}
	})
}

// RenderStyle returns the Glamour style name used to render card bodies.
func (m ReviewModel) RenderStyle() string {
	return m.renderStyle
}

// ShowingBack reports whether the answer side of the current card is visible.
func (m ReviewModel) ShowingBack() bool {
	return m.showingBack
}

// CurrentIndex returns the position of the card currently being reviewed.
func (m ReviewModel) CurrentIndex() int {
	return m.index
}

// SessionStats holds in-memory counts for a review session.
type SessionStats struct {
	RatingCounts  map[int]int
	SkippedCount  int
	TotalReviewed int
}

// Stats returns a snapshot of the session statistics.
func (m ReviewModel) Stats() SessionStats {
	return SessionStats{
		RatingCounts:  m.ratingCounts,
		SkippedCount:  m.skippedCount,
		TotalReviewed: m.index,
	}
}

// Skip increments the skipped-card counter. It is called when the user
// defers the current card without rating it.
func (m *ReviewModel) Skip() {
	m.skippedCount++
}

// Init implements tea.Model.
func (m ReviewModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles the review lifecycle:
//
//   - Space / Enter — flip the current card to its back side and compute
//     interval previews via the fsrs scheduler.
//   - 1–4 — rate the card (only valid while the back is showing), advance
//     to the next card, and clear previews.
//   - q — emit tea.Quit to exit the application.
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showingHelp {
			if msg.String() == "?" || msg.Type == tea.KeyEsc {
				m.showingHelp = false
			}
			return m, nil
		}
		if m.quitConfirm {
			switch msg.String() {
			case "y":
				return m, tea.Quit
			case "n", "N":
				m.quitConfirm = false
			}
			return m, nil
		}
		if msg.String() == "q" {
			if m.showingBack && !m.done {
				m.quitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		}
		if msg.String() == "?" {
			m.showingHelp = true
			return m, nil
		}
		if m.done {
			if msg.Type == tea.KeyEnter {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.Type {
		case tea.KeySpace, tea.KeyEnter:
			if !m.showingBack {
				m.showingBack = true
				if m.index < len(m.items) {
					it := &m.items[m.index]
					cs := cardStateFromItem(it)
					m.previews = fsrs.Preview(cs, time.Now())
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "1", "2", "3", "4":
			if m.showingBack && m.rateFunc != nil && m.index < len(m.items) {
				rating := int(msg.String()[0] - '0')
				it := &m.items[m.index]
				prevCard := *it.Card
				_, _, entry, err := m.rateFunc(it, rating, time.Now())
				if err == nil {
					m.ratingCounts[rating]++
					m.undoable = &undoInfo{
						entry:    entry,
						cardPath: it.Card.FilePath,
						card:     &prevCard,
						rating:   rating,
					}
					m.index++
					m.showingBack = false
					m.previews = nil
					if m.index >= len(m.items) {
						m.done = true
					}
				}
			}
			return m, nil
		case "s":
			if m.index < len(m.items) {
				m.skippedCount++
				m.items = append(m.items, m.items[m.index])
				m.items = append(m.items[:m.index], m.items[m.index+1:]...)
				m.showingBack = false
				m.previews = nil
			}
			return m, nil
		case "e":
			if m.index < len(m.items) && m.items[m.index].Card.FilePath != "" {
				path := m.items[m.index].Card.FilePath
				return m, m.editorCmd(path)
			}
			return m, nil
		case "u":
			if m.undoable != nil && m.undoFunc != nil {
				if err := m.undoFunc(m.undoable.entry, m.undoable.cardPath, m.undoable.card); err == nil {
					m.ratingCounts[m.undoable.rating]--
					m.index--
					m.done = false
					m.showingBack = false
					m.previews = nil
					m.items[m.index].Card = m.undoable.card
					m.undoable = nil
				}
			}
			return m, nil
		}
	}
	switch msg := msg.(type) {
	case EditFinishedMsg:
		m.editErr = ""
		if msg.Err != nil {
			m.editErr = fmt.Sprintf("editor error: %v", msg.Err)
			return m, nil
		}
		updated, err := m.cardReadFunc(msg.Path)
		if err != nil {
			m.editErr = fmt.Sprintf("error re-reading card: %v", err)
			return m, nil
		}
		if updated != nil && m.index < len(m.items) {
			m.items[m.index].Card = updated
			m.editErr = ""
		}
		return m, nil
	}
	return m, nil
}

// View implements tea.Model. It renders the front or back of the current
// card (markdown formatted via glamour) and, when the back is showing,
// appends the interval previews returned by the scheduler.
func (m ReviewModel) View() string {
	if len(m.items) == 0 {
		return "No cards in this deck.\nPress q to quit."
	}
	if m.showingHelp {
		return renderHelpOverlay()
	}
	if m.quitConfirm {
		return "Rating in progress - quit anyway? (y/N)"
	}
	if m.done {
		return m.renderSummary()
	}
	it := &m.items[m.index]
	var content string
	if it.Card.Type == card.Cloze {
		content = renderCloze(it.Card.Body, it.ClozeGroup, m.showingBack)
	} else {
		content = it.Card.Front
		if m.showingBack {
			content = it.Card.Back
		}
	}
	rendered, _ := m.renderer.Render(content)
	if m.showingBack && len(m.previews) > 0 {
		rendered += formatPreviews(m.previews)
	}
	if m.editErr != "" {
		rendered += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(m.editErr) + "\n"
	}
	return rendered
}

// cardStateFromItem extracts the FSRS state from a review item. For cloze
// cards it uses the per-group state; for basic cards it uses the flat fields.
func cardStateFromItem(it *deck.ReviewItem) fsrs.CardState {
	if it.Card.Type == card.Cloze && it.ClozeGroup != "" {
		if g, ok := it.Card.Clozes[it.ClozeGroup]; ok {
			return fsrs.CardState{
				State:      fsrs.NormalizeState(g.State),
				Due:        fsrs.ParseTime(g.Due),
				Stability:  g.Stability,
				Difficulty: g.Difficulty,
				Reps:       g.Reps,
				Lapses:     g.Lapses,
			}
		}
	}
	return fsrs.CardState{
		State:      fsrs.NormalizeState(it.Card.State),
		Due:        fsrs.ParseTime(it.Card.Due),
		Stability:  it.Card.Stability,
		Difficulty: it.Card.Difficulty,
		Reps:       it.Card.Reps,
		Lapses:     it.Card.Lapses,
	}
}

var renderClozeRe = regexp.MustCompile(`\{\{c(\d+)::([^}]+)\}\}`)

// renderCloze processes a cloze card body for display. If activeGroup is set
// and showAnswer is false, that group's deletions are replaced with a
// placeholder ([...] or [hint] if present). All other deletions are revealed.
func renderCloze(body, activeGroup string, showAnswer bool) string {
	return renderClozeRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := renderClozeRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		group := "c" + parts[1]
		inner := parts[2]
		// inner may be "answer" or "answer::hint"
		var answer, hint string
		if i := strings.Index(inner, "::"); i >= 0 {
			answer = inner[:i]
			hint = inner[i+2:]
		} else {
			answer = inner
		}
		if showAnswer || group != activeGroup {
			return answer
		}
		if hint != "" {
			return "[" + hint + "]"
		}
		return "[...]"
	})
}

// formatPreviews renders a list of interval previews as rating labels with
// human-readable intervals (e.g. "1 Again (1m)").
func formatPreviews(previews []fsrs.IntervalPreview) string {
	labels := map[int]string{1: "Again", 2: "Hard", 3: "Good", 4: "Easy"}
	var s string
	for _, p := range previews {
		label := labels[p.Rating]
		s += fmt.Sprintf("\n  %d %s (%s)", p.Rating, label, formatDuration(p.Interval))
	}
	s += "\n"
	return s
}

// formatDuration converts a time.Duration into a compact string:
// "< 1m" for sub-minute, "%dm" for minutes, "%dh" for hours, and "%dd" for days.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

var (
	summaryTitle = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	summaryTotal = lipgloss.NewStyle().Bold(true)
	summaryLabel = lipgloss.NewStyle().Width(8)
	summaryHint  = lipgloss.NewStyle().Faint(true).MarginTop(1)
)

func (m ReviewModel) renderSummary() string {
	labels := map[int]string{1: "Again", 2: "Hard", 3: "Good", 4: "Easy"}
	s := summaryTitle.Render("Session complete!")
	s += summaryTotal.Render(fmt.Sprintf("Cards reviewed: %d", m.index)) + "\n"
	for _, r := range []int{1, 2, 3, 4} {
		s += summaryLabel.Render(fmt.Sprintf("  %s:", labels[r])) + fmt.Sprintf(" %d\n", m.ratingCounts[r])
	}
	if m.skippedCount > 0 {
		s += summaryLabel.Render("  Skipped:") + fmt.Sprintf(" %d\n", m.skippedCount)
	}
	s += summaryHint.Render("Press q or Enter to quit.")
	return s
}

var helpTitle = lipgloss.NewStyle().Bold(true).MarginBottom(1)

func renderHelpOverlay() string {
	s := helpTitle.Render("Keybindings")
	s += "  space/enter  flip card\n"
	s += "  1-4          rate (again/hard/good/easy)\n"
	s += "  e            edit card\n"
	s += "  u            undo last rating\n"
	s += "  s            skip card\n"
	s += "  ?            toggle help\n"
	s += "  q            quit\n"
	return s
}
