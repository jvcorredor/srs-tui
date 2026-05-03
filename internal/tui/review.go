// Package tui provides the interactive terminal review session.
package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

type RateFunc func(c *card.Card, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error)

type ReviewModel struct {
	cards       []*card.Card
	index       int
	showingBack bool
	renderer    *glamour.TermRenderer
	rateFunc    RateFunc
	previews    []fsrs.IntervalPreview
	done        bool
}

func NewReviewModel(cards []*card.Card, rateFunc RateFunc) ReviewModel {
	r, _ := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"))
	return ReviewModel{
		cards:    cards,
		renderer: r,
		rateFunc: rateFunc,
	}
}

func (m ReviewModel) ShowingBack() bool {
	return m.showingBack
}

func (m ReviewModel) CurrentIndex() int {
	return m.index
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

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
				if m.index < len(m.cards) {
					c := m.cards[m.index]
					cs := cardStateFromCard(c)
					m.previews = fsrs.Preview(cs, time.Now())
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "1", "2", "3", "4":
			if m.showingBack && m.rateFunc != nil && m.index < len(m.cards) {
				rating := int(msg.String()[0] - '0')
				c := m.cards[m.index]
				_, _, err := m.rateFunc(c, rating, time.Now())
				if err == nil {
					m.index++
					m.showingBack = false
					m.previews = nil
					if m.index >= len(m.cards) {
						m.done = true
					}
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func (m ReviewModel) View() string {
	if len(m.cards) == 0 {
		return "No cards in this deck.\nPress q to quit."
	}
	if m.done {
		return "Session complete!\nPress q to quit."
	}
	c := m.cards[m.index]
	content := c.Front
	if m.showingBack {
		content = c.Back
	}
	rendered, _ := m.renderer.Render(content)
	if m.showingBack && len(m.previews) > 0 {
		rendered += formatPreviews(m.previews)
	}
	return rendered
}

func cardStateFromCard(c *card.Card) fsrs.CardState {
	return fsrs.CardState{
		State:      fsrs.NormalizeState(c.State),
		Due:       fsrs.ParseTime(c.Due),
		Stability:  c.Stability,
		Difficulty: c.Difficulty,
		Reps:      c.Reps,
		Lapses:    c.Lapses,
	}
}

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
