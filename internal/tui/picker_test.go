package tui_test

import (
	"os"
	"path/filepath"
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
// a helpful message when there are no decks, pointing the user at the N
// keybinding and srs init.
func TestPickerEmptyStateShowsFriendlyMessage(t *testing.T) {
	m := tui.NewPickerModel(nil, nil)
	view := m.View()
	if !strings.Contains(view, "srs init") {
		t.Errorf("empty picker should mention 'srs init', got:\n%s", view)
	}
	if !strings.Contains(view, "`N`") {
		t.Errorf("empty picker should mention the N keybinding, got:\n%s", view)
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

// typePicker feeds each rune of s to the model as a key message, mimicking a
// user typing into the deck-name textinput.
func typePicker(m tui.PickerModel, s string) tui.PickerModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return asPicker(updated)
}

// pressN sends a Shift+N key message to the model.
func pressN(m tui.PickerModel) tui.PickerModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	return asPicker(updated)
}

// TestPickerNActivatesTextinput verifies that pressing N opens the inline
// deck-name textinput overlay.
func TestPickerNActivatesTextinput(t *testing.T) {
	decks := []tui.DeckEntry{{Name: "french", Path: "/tmp/french"}}
	m := tui.NewPickerModel(decks, nil, tui.WithDecksRoot(t.TempDir()))
	m = pressN(m)
	if !strings.Contains(m.View(), "New deck") {
		t.Errorf("after N, view should show the deck-name input, got:\n%s", m.View())
	}
}

// TestPickerCreateDeckAddsToListAndJumpsCursor verifies that submitting a
// deck name creates the directory, adds the deck to the list, and moves the
// cursor onto the new deck.
func TestPickerCreateDeckAddsToListAndJumpsCursor(t *testing.T) {
	root := t.TempDir()
	decks := []tui.DeckEntry{
		{Name: "french", Path: filepath.Join(root, "french")},
		{Name: "golang", Path: filepath.Join(root, "golang")},
	}
	m := tui.NewPickerModel(decks, nil, tui.WithDecksRoot(root))
	m = pressN(m)
	m = typePicker(m, "Spanish Vocab")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = asPicker(updated)

	deckDir := filepath.Join(root, "spanish-vocab")
	if info, err := os.Stat(deckDir); err != nil || !info.IsDir() {
		t.Fatalf("deck directory %s should exist, err=%v", deckDir, err)
	}
	view := m.View()
	if !strings.Contains(view, "spanish-vocab") {
		t.Errorf("new deck should appear in picker list, got:\n%s", view)
	}
	if m.SelectedIndex() != 2 {
		t.Errorf("cursor should be on the new deck (index 2), got %d", m.SelectedIndex())
	}
}

// TestPickerCreateFirstDeckFromEmpty verifies that N works on the empty
// picker and creates the very first deck.
func TestPickerCreateFirstDeckFromEmpty(t *testing.T) {
	root := t.TempDir()
	m := tui.NewPickerModel(nil, nil, tui.WithDecksRoot(root))
	m = pressN(m)
	m = typePicker(m, "my first deck")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = asPicker(updated)

	if info, err := os.Stat(filepath.Join(root, "my-first-deck")); err != nil || !info.IsDir() {
		t.Fatalf("first deck directory should exist, err=%v", err)
	}
	if !strings.Contains(m.View(), "my-first-deck") {
		t.Errorf("first deck should appear in picker list, got:\n%s", m.View())
	}
}

// TestPickerEscDismissesTextinput verifies that Esc cancels the deck-name
// overlay and returns to the normal picker without creating anything.
func TestPickerEscDismissesTextinput(t *testing.T) {
	root := t.TempDir()
	decks := []tui.DeckEntry{{Name: "french", Path: filepath.Join(root, "french")}}
	m := tui.NewPickerModel(decks, nil, tui.WithDecksRoot(root))
	m = pressN(m)
	m = typePicker(m, "discarded")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = asPicker(updated)

	if strings.Contains(m.View(), "New deck") {
		t.Errorf("Esc should dismiss the deck-name input, got:\n%s", m.View())
	}
	if _, err := os.Stat(filepath.Join(root, "discarded")); !os.IsNotExist(err) {
		t.Errorf("Esc should not create a deck directory")
	}
}

// TestPickerLowercaseNDoesNothingOnEmpty verifies that the lowercase n key
// does nothing on the empty picker.
func TestPickerLowercaseNDoesNothingOnEmpty(t *testing.T) {
	m := tui.NewPickerModel(nil, nil, tui.WithDecksRoot(t.TempDir()))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = asPicker(updated)
	if strings.Contains(m.View(), "New deck") {
		t.Errorf("lowercase n should not open the deck-name input, got:\n%s", m.View())
	}
}
