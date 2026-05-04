package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
	"github.com/jvcorredor/srs-tui/internal/store"
	"github.com/jvcorredor/srs-tui/internal/tui"
)

// basicItem builds a single basic ReviewItem for tests.
func pickerBasicItem(id, front, back string) deck.ReviewItem {
	return deck.ReviewItem{
		Card: &card.Card{Meta: card.Meta{ID: id, Type: card.Basic}, Front: front, Back: back},
	}
}

// pickerFakeRateFunc is a stub RateFunc for picker tests.
func pickerFakeRateFunc(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, store.LogEntry, error) {
	next := fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5}
	previews := []fsrs.IntervalPreview{
		{Rating: 1, State: fsrs.StateLearning, Interval: 1 * time.Minute},
		{Rating: 2, State: fsrs.StateLearning, Interval: 5 * time.Minute},
		{Rating: 3, State: fsrs.StateLearning, Interval: 10 * time.Minute},
		{Rating: 4, State: fsrs.StateReview, Interval: 24 * time.Hour},
	}
	entry := store.LogEntry{Schema: 1, TS: now, CardID: item.Card.ID, Rating: rating, Prev: fsrs.CardState{State: fsrs.StateNew}, Next: next}
	return next, previews, entry, nil
}

// asPicker asserts that a tea.Model is a tui.PickerModel and returns it.
func asPicker(m tea.Model) tui.PickerModel {
	return m.(tui.PickerModel)
}

// TestPickerEmptyStateShowsFriendlyMessage verifies that the picker displays
// a helpful message when there are no decks, pointing the user at srs init
// and srs new.
func TestPickerEmptyStateShowsFriendlyMessage(t *testing.T) {
	m := tui.NewPickerModel(nil, nil)
	view := m.View()
	if !strings.Contains(view, "srs init") {
		t.Errorf("empty picker should mention 'srs init', got:\n%s", view)
	}
	if !strings.Contains(view, "srs new") {
		t.Errorf("empty picker should mention 'srs new', got:\n%s", view)
	}
}

// TestPickerQuitOnQ verifies that pressing 'q' on the picker returns a
// tea.Quit command.
func TestPickerQuitOnQ(t *testing.T) {
	m := tui.NewPickerModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit on picker")
	}
}

// TestPickerShowsDeckNameAndDueCount verifies that the picker view includes
// each deck's name and its due-card count when decks are present.
func TestPickerShowsDeckNameAndDueCount(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 5},
		{Name: "golang", Path: "/tmp/golang", DueCount: 0},
	}
	m := tui.NewPickerModel(decks, nil)
	view := m.View()
	if !strings.Contains(view, "french") {
		t.Errorf("picker should show deck name 'french', got:\n%s", view)
	}
	if !strings.Contains(view, "golang") {
		t.Errorf("picker should show deck name 'golang', got:\n%s", view)
	}
	if !strings.Contains(view, "5") {
		t.Errorf("picker should show due count for french, got:\n%s", view)
	}
}

// TestPickerNavigateDownWithJ verifies that pressing 'j' moves the cursor
// down to the next deck in the list.
func TestPickerNavigateDownWithJ(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 5},
		{Name: "golang", Path: "/tmp/golang", DueCount: 3},
	}
	m := tui.NewPickerModel(decks, nil)
	if m.SelectedIndex() != 0 {
		t.Errorf("initial selection = %d, want 0", m.SelectedIndex())
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = asPicker(updated)
	if m.SelectedIndex() != 1 {
		t.Errorf("after j, selection = %d, want 1", m.SelectedIndex())
	}
}

// TestPickerNavigateUpWithK verifies that pressing 'k' moves the cursor
// up to the previous deck in the list.
func TestPickerNavigateUpWithK(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 5},
		{Name: "golang", Path: "/tmp/golang", DueCount: 3},
	}
	m := tui.NewPickerModel(decks, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = asPicker(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = asPicker(updated)
	if m.SelectedIndex() != 0 {
		t.Errorf("after j then k, selection = %d, want 0", m.SelectedIndex())
	}
}

// TestPickerSelectCallsOnSelect verifies that pressing Enter calls the
// OnSelect callback with the currently selected deck entry.
func TestPickerSelectCallsOnSelect(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 5},
		{Name: "golang", Path: "/tmp/golang", DueCount: 3},
	}
	var selected tui.DeckEntry
	m := tui.NewPickerModel(decks, func(e tui.DeckEntry) (tea.Model, tea.Cmd) {
		selected = e
		return nil, nil
	})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected.Name != "french" {
		t.Errorf("onSelect called with %q, want %q", selected.Name, "french")
	}
}

// TestPickerQuitOnQWithDecks verifies that pressing 'q' on a non-empty
// picker returns a tea.Quit command.
func TestPickerQuitOnQWithDecks(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 5},
	}
	m := tui.NewPickerModel(decks, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit on non-empty picker")
	}
}

// TestPickerSelectTransitionsToReviewModel verifies that selecting a deck
// via Enter returns a ReviewModel through the OnSelectFunc callback.
func TestPickerSelectTransitionsToReviewModel(t *testing.T) {
	decks := []tui.DeckEntry{
		{Name: "french", Path: "/tmp/french", DueCount: 1},
	}
	var modelReturned tea.Model
	m := tui.NewPickerModel(decks, func(e tui.DeckEntry) (tea.Model, tea.Cmd) {
		items := []deck.ReviewItem{pickerBasicItem("1", "Q", "A")}
		review := tui.NewReviewModel(items, pickerFakeRateFunc, tui.WithRenderStyle("dark"))
		modelReturned = review
		return review, nil
	})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if modelReturned == nil {
		t.Fatal("onSelect should have been called")
	}
	if _, ok := modelReturned.(tui.ReviewModel); !ok {
		t.Errorf("expected tui.ReviewModel, got %T", modelReturned)
	}
}
