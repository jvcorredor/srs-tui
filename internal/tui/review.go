// Package tui implements the Bubble Tea review interface for spaced-repetition
// cards. A review session follows a three-state lifecycle:
//
//   1. Front — the question side is displayed.
//   2. Back — pressing Space or Enter reveals the answer and shows interval
//      previews (again, hard, good, easy) computed by the scheduler.
//   3. Rate — pressing a rating key (1–4) applies the score via RateFunc,
//      advances to the next card, and returns to the front state.
//
// When every card has been rated the session ends and View signals completion.
package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

// RateFunc applies a user rating to a review item and returns the resulting
// state, interval previews for all possible ratings, and any error.
type RateFunc func(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error)

// ReviewModel is a Bubble Tea model that drives a flash-card review session.
// It manages a deck of review items, tracks which side is visible, and
// coordinates with a RateFunc to schedule cards after each rating.
type ReviewModel struct {
	items       []deck.ReviewItem
	index       int
	showingBack bool
	renderer    *glamour.TermRenderer
	rateFunc    RateFunc
	previews    []fsrs.IntervalPreview
	done        bool
}

// NewReviewModel creates a ReviewModel for the given review items. The
// rateFunc is invoked each time the user presses a rating key (1–4) while
// the back side is visible.
func NewReviewModel(items []deck.ReviewItem, rateFunc RateFunc) ReviewModel {
	r, _ := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"))
	return ReviewModel{
		items:    items,
		renderer: r,
		rateFunc: rateFunc,
	}
}

// ShowingBack reports whether the answer side of the current card is visible.
func (m ReviewModel) ShowingBack() bool {
	return m.showingBack
}

// CurrentIndex returns the position of the card currently being reviewed.
func (m ReviewModel) CurrentIndex() int {
	return m.index
}

// Init implements tea.Model.
func (m ReviewModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles the review lifecycle:
//
//   • Space / Enter — flip the current card to its back side and compute
//     interval previews via the fsrs scheduler.
//   • 1–4 — rate the card (only valid while the back is showing), advance
//     to the next card, and clear previews.
//   • q — emit tea.Quit to exit the application.
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		if m.done {
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
				_, _, err := m.rateFunc(it, rating, time.Now())
				if err == nil {
					m.index++
					m.showingBack = false
					m.previews = nil
					if m.index >= len(m.items) {
						m.done = true
					}
				}
			}
			return m, nil
		}
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
	if m.done {
		return "Session complete!\nPress q to quit."
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
