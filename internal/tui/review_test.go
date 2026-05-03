package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/tui"
)

func asReview(m tea.Model) tui.ReviewModel {
	return m.(tui.ReviewModel)
}

func TestReviewFlipOnSpace(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards, nil)
	if m.ShowingBack() {
		t.Error("should start showing front")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !asReview(updated).ShowingBack() {
		t.Error("space should flip to back")
	}
}

func TestReviewFlipOnEnter(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !asReview(updated).ShowingBack() {
		t.Error("enter should flip to back")
	}
}

func TestReviewQuitOnQ(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit")
	}
}

func fakeRateFunc(c *card.Card, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error) {
	next := fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5}
	previews := []fsrs.IntervalPreview{
		{Rating: 1, State: fsrs.StateLearning, Interval: 1 * time.Minute},
		{Rating: 2, State: fsrs.StateLearning, Interval: 5 * time.Minute},
		{Rating: 3, State: fsrs.StateLearning, Interval: 10 * time.Minute},
		{Rating: 4, State: fsrs.StateReview, Interval: 24 * time.Hour},
	}
	return next, previews, nil
}

func TestRatingKeyAdvancesCard(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q1", Back: "A1"},
		{Meta: card.Meta{ID: "2", Type: card.Basic}, Front: "Q2", Back: "A2"},
	}
	m := tui.NewReviewModel(cards, fakeRateFunc)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)
	if m.CurrentIndex() != 1 {
		t.Errorf("after rating card 0, index = %d, want 1", m.CurrentIndex())
	}
	if m.ShowingBack() {
		t.Error("after advancing, should show front of next card")
	}
}

func TestRatingKeyShowsIntervalPreviewsOnBack(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards, fakeRateFunc)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)

	view := m.View()
	if !strings.Contains(view, "Again") || !strings.Contains(view, "Hard") {
		t.Errorf("answer screen should show rating options, got:\n%s", view)
	}
}

func TestAllFourRatingKeysAccepted(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q1", Back: "A1"},
		{Meta: card.Meta{ID: "2", Type: card.Basic}, Front: "Q2", Back: "A2"},
		{Meta: card.Meta{ID: "3", Type: card.Basic}, Front: "Q3", Back: "A3"},
		{Meta: card.Meta{ID: "4", Type: card.Basic}, Front: "Q4", Back: "A4"},
	}
	m := tui.NewReviewModel(cards, fakeRateFunc)

	for _, key := range []rune{'1', '2', '3', '4'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = asReview(updated)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = asReview(updated)
	}
	if m.CurrentIndex() != 4 {
		t.Errorf("after rating 4 cards, index = %d, want 4", m.CurrentIndex())
	}
}
