// Package tui_test contains integration tests for the review TUI.
//
// Tests exercise the ReviewModel through its public Update and View methods
// rather than internal state, matching the project’s behavior-first testing
// philosophy.
package tui_test

import (
	"fmt"
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

// asReview asserts that a tea.Model is a tui.ReviewModel and returns it.
// It is used by tests that need ReviewModel-specific getters after an Update.
func asReview(m tea.Model) tui.ReviewModel {
	return m.(tui.ReviewModel)
}

// basicItem is a helper that builds a single basic ReviewItem for tests.
func basicItem(id, front, back string) deck.ReviewItem {
	return deck.ReviewItem{
		Card: &card.Card{Meta: card.Meta{ID: id, Type: card.Basic}, Front: front, Back: back},
	}
}

// clozeItem is a helper that builds a cloze ReviewItem for tests.
func clozeItem(id, body, group string) deck.ReviewItem {
	return deck.ReviewItem{
		Card:       &card.Card{Meta: card.Meta{ID: id, Type: card.Cloze}, Body: body},
		ClozeGroup: group,
	}
}

// TestReviewFlipOnSpace verifies that pressing Space reveals the back of the
// current card.
func TestReviewFlipOnSpace(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	if m.ShowingBack() {
		t.Error("should start showing front")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !asReview(updated).ShowingBack() {
		t.Error("space should flip to back")
	}
}

// TestReviewFlipOnEnter verifies that pressing Enter also reveals the back.
func TestReviewFlipOnEnter(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !asReview(updated).ShowingBack() {
		t.Error("enter should flip to back")
	}
}

// TestReviewQuitOnQ verifies that pressing 'q' returns a tea.Quit command.
func TestReviewQuitOnQ(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit")
	}
}

// TestReviewQuitOnQWhenDone verifies that pressing 'q' quits even after the
// session is complete (m.done == true).
func TestReviewQuitOnQWhenDone(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	// Flip and rate the only card so the session ends.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)

	if m.CurrentIndex() != 1 {
		t.Fatalf("expected session to be done, index = %d", m.CurrentIndex())
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit even when session is done")
	}
}

// fakeRateFunc is a stub RateFunc that returns fixed interval previews for
// every rating, making tests deterministic and fast.
func fakeRateFunc(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, store.LogEntry, error) {
	next := fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5}
	previews := []fsrs.IntervalPreview{
		{Rating: 1, State: fsrs.StateLearning, Interval: 1 * time.Minute},
		{Rating: 2, State: fsrs.StateLearning, Interval: 5 * time.Minute},
		{Rating: 3, State: fsrs.StateLearning, Interval: 10 * time.Minute},
		{Rating: 4, State: fsrs.StateReview, Interval: 24 * time.Hour},
	}
	entry := store.LogEntry{
		Schema: 1,
		TS:     now,
		CardID: item.Card.ID,
		Rating: rating,
		Prev:   fsrs.CardState{State: fsrs.StateNew},
		Next:   next,
	}
	return next, previews, entry, nil
}

// TestRatingKeyAdvancesCard checks that rating a flipped card moves the
// session to the next card and resets the view to the front side.
func TestRatingKeyAdvancesCard(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
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

// TestRatingKeyShowsIntervalPreviewsOnBack confirms that the rendered view
// includes preview labels (Again, Hard, etc.) once the card is flipped.
func TestRatingKeyShowsIntervalPreviewsOnBack(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)

	view := m.View()
	if !strings.Contains(view, "Again") || !strings.Contains(view, "Hard") {
		t.Errorf("answer screen should show rating options, got:\n%s", view)
	}
}

// TestAllFourRatingKeysAccepted validates that every rating key (1–4) can be
// used to advance through a multi-card session without error.
func TestAllFourRatingKeysAccepted(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
		basicItem("3", "Q3", "A3"),
		basicItem("4", "Q4", "A4"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))

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

// TestClozeQuestionHidesActiveGroup verifies that the question side of a cloze
// card replaces the active group's deletions with a placeholder.
func TestClozeQuestionHidesActiveGroup(t *testing.T) {
	items := []deck.ReviewItem{
		clozeItem("1", "The {{c1::capital::city}} of France is {{c2::Paris}}.", "c1"),
	}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	view := m.View()
	if strings.Contains(view, "capital") {
		t.Errorf("question should hide active group c1, got:\n%s", view)
	}
	if !strings.Contains(view, "city") {
		t.Errorf("question should show hint placeholder containing 'city', got:\n%s", view)
	}
	if !strings.Contains(view, "Paris") {
		t.Errorf("question should reveal inactive group c2, got:\n%s", view)
	}
}

// TestClozeAnswerRevealsActiveGroup verifies that the answer side of a cloze
// card reveals all deletions including the active group.
func TestClozeAnswerRevealsActiveGroup(t *testing.T) {
	items := []deck.ReviewItem{
		clozeItem("1", "The {{c1::capital::city}} of France is {{c2::Paris}}.", "c1"),
	}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	view := m.View()
	if !strings.Contains(view, "capital") {
		t.Errorf("answer should reveal active group c1, got:\n%s", view)
	}
	if !strings.Contains(view, "Paris") {
		t.Errorf("answer should reveal inactive group c2, got:\n%s", view)
	}
	if strings.Contains(view, "city") {
		t.Errorf("answer should not show placeholder hint, got:\n%s", view)
	}
}

// TestRatingTracksCounts verifies that rating a card increments the
// appropriate count in the session statistics.
func TestRatingTracksCounts(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)
	stats := m.Stats()
	if stats.RatingCounts[3] != 1 {
		t.Errorf("after rating Good, RatingCounts[3] = %d, want 1", stats.RatingCounts[3])
	}
	if stats.TotalReviewed != 1 {
		t.Errorf("after rating 1 card, TotalReviewed = %d, want 1", stats.TotalReviewed)
	}
}

// TestSummaryShowsTotalAndBreakdown verifies that after exhausting the queue
// the summary view includes the total cards reviewed and per-rating counts.
func TestSummaryShowsTotalAndBreakdown(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
		basicItem("3", "Q3", "A3"),
		basicItem("4", "Q4", "A4"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	keys := []rune{'1', '2', '3', '4'}
	for _, key := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = asReview(updated)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = asReview(updated)
	}
	view := m.View()
	if !strings.Contains(view, "4") {
		t.Errorf("summary should show total 4 cards reviewed, got:\n%s", view)
	}
	if !strings.Contains(view, "Again") || !strings.Contains(view, "Hard") || !strings.Contains(view, "Good") || !strings.Contains(view, "Easy") {
		t.Errorf("summary should show all four rating labels, got:\n%s", view)
	}
}

// TestSummaryEnterQuits verifies that pressing Enter on the summary screen
// emits tea.Quit, just like pressing q.
func TestSummaryEnterQuits(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on summary screen should trigger quit")
	}
}

// TestSummarySkippedCardsShownWhenNonZero verifies that the summary view
// includes a Skipped count only when at least one card was skipped.
func TestSummarySkippedCardsShownWhenNonZero(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	m.Skip()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)
	view := m.View()
	if !strings.Contains(view, "Skipped") {
		t.Errorf("summary should show Skipped count when non-zero, got:\n%s", view)
	}
}

// TestSummarySkippedCardsHiddenWhenZero verifies that the summary view omits
// the Skipped line when no cards were skipped.
func TestSummarySkippedCardsHiddenWhenZero(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)
	view := m.View()
	if strings.Contains(view, "Skipped") {
		t.Errorf("summary should not show Skipped when count is zero, got:\n%s", view)
	}
}

// TestSummaryContainsStyledContent verifies that the summary view includes
// the key content (total, rating labels, exit hint) regardless of styling.
func TestSummaryContainsStyledContent(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithRenderStyle("dark"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = asReview(updated)
	view := m.View()
	for _, want := range []string{"2", "Again", "Easy", "Enter"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary should contain %q, got:\n%s", want, view)
		}
	}
}

// TestSkipMovesCardToEndOfQueue verifies that pressing 's' moves the current
// card to the end of the queue, increments the skipped counter, and shows
// the next card's front side.
func TestSkipMovesCardToEndOfQueue(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
		basicItem("3", "Q3", "A3"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = asReview(updated)

	if m.ShowingBack() {
		t.Error("after skipping, should show front of next card")
	}
	stats := m.Stats()
	if stats.SkippedCount != 1 {
		t.Errorf("after skipping 1 card, SkippedCount = %d, want 1", stats.SkippedCount)
	}

	view := m.View()
	if !strings.Contains(view, "Q2") {
		t.Errorf("after skipping Q1, should show Q2, got:\n%s", view)
	}
}

// TestSkipOnLastCardWrapsToSkippedCard verifies that skipping the last
// remaining card moves it to the end and the session continues with that
// same card (now at the back of the queue).
func TestSkipOnLastCardWrapsToSkippedCard(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = asReview(updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = asReview(updated)

	stats := m.Stats()
	if stats.SkippedCount != 2 {
		t.Errorf("after skipping 2 cards, SkippedCount = %d, want 2", stats.SkippedCount)
	}
	if m.CurrentIndex() != 0 {
		t.Errorf("after skipping all cards, index should wrap, got %d", m.CurrentIndex())
	}
}

// TestSkipDoesNotPersist verifies that skipping does not call the rateFunc.
func TestSkipDoesNotPersist(t *testing.T) {
	called := false
	rateFunc := func(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, store.LogEntry, error) {
		called = true
		return fsrs.CardState{}, nil, store.LogEntry{}, nil
	}
	items := []deck.ReviewItem{basicItem("1", "Q", "A"), basicItem("2", "Q2", "A2")}
	m := tui.NewReviewModel(items, rateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = asReview(updated)

	if called {
		t.Error("skip should not call rateFunc")
	}
	stats := m.Stats()
	if stats.TotalReviewed != 0 {
		t.Error("skip should not increment TotalReviewed")
	}
}

// TestSkipResetsFlipState verifies that skipping a card that has been
// flipped to the back side resets to showing the front of the next card.
func TestSkipResetsFlipState(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	if !m.ShowingBack() {
		t.Fatal("should be showing back before skip")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = asReview(updated)

	if m.ShowingBack() {
		t.Error("after skipping a flipped card, should show front of next card")
	}
}

// TestHelpOverlayTogglesOnQuestionMark verifies that pressing '?' shows
// the help overlay listing all keybindings.
func TestHelpOverlayTogglesOnQuestionMark(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = asReview(updated)

	view := m.View()
	if !strings.Contains(view, "Keybindings") {
		t.Errorf("help overlay should appear after pressing ?, got:\n%s", view)
	}
}

// TestHelpOverlayDismissesOnQuestionMark verifies that pressing '?' again
// hides the help overlay.
func TestHelpOverlayDismissesOnQuestionMark(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = asReview(updated)

	view := m.View()
	if strings.Contains(view, "Keybindings") {
		t.Errorf("help overlay should be dismissed after second press of ?, got:\n%s", view)
	}
}

// TestHelpOverlayDismissesOnEsc verifies that pressing Escape hides the
// help overlay.
func TestHelpOverlayDismissesOnEsc(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = asReview(updated)

	view := m.View()
	if strings.Contains(view, "Keybindings") {
		t.Errorf("help overlay should be dismissed after Esc, got:\n%s", view)
	}
}

// TestHelpOverlayListsAllKeybindings verifies that the help overlay includes
// all expected keybinding labels.
func TestHelpOverlayListsAllKeybindings(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = asReview(updated)

	view := m.View()
	for _, want := range []string{"space/enter", "1-4", "e", "u", "s", "?", "q"} {
		if !strings.Contains(view, want) {
			t.Errorf("help overlay should list keybinding %q, got:\n%s", want, view)
		}
	}
}

// TestQuitConfirmsWhenCardFlippedNotRated verifies that pressing 'q' while a
// card is flipped (back showing) but not yet rated shows a confirmation prompt
// instead of quitting immediately.
func TestQuitConfirmsWhenCardFlippedNotRated(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = asReview(updated)
	if cmd != nil {
		t.Error("q while flipped should not quit immediately, should show confirm prompt")
	}
	view := m.View()
	if !strings.Contains(view, "quit anyway") {
		t.Errorf("should show quit confirmation prompt, got:\n%s", view)
	}
}

// TestQuitConfirmAcceptsY verifies that pressing 'y' at the quit confirmation
// prompt exits the application.
func TestQuitConfirmAcceptsY(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = asReview(updated)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Error("y at quit confirmation should trigger tea.Quit")
	}
}

// TestQuitConfirmRejectsN verifies that pressing 'n' at the quit confirmation
// prompt dismisses the prompt and returns to the review.
func TestQuitConfirmRejectsN(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = asReview(updated)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = asReview(updated)
	if cmd != nil {
		t.Error("n at quit confirmation should not quit")
	}
	view := m.View()
	if strings.Contains(view, "quit anyway") {
		t.Errorf("n should dismiss quit prompt, got:\n%s", view)
	}
}

// TestQuitNoConfirmOnFrontSide verifies that pressing 'q' on the front side
// (not flipped) quits immediately without a confirmation prompt.
func TestQuitNoConfirmOnFrontSide(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q on front side should quit immediately")
	}
}

// TestQuitNoConfirmAfterRating verifies that pressing 'q' after rating a card
// (on the next card's front side) quits immediately.
func TestQuitNoConfirmAfterRating(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q after rating should quit immediately (on front side of next card)")
	}
}

// TestEditOpensCurrentCardInEditor verifies that pressing 'e' invokes the
// editor command with the current card's file path.
func TestEditOpensCurrentCardInEditor(t *testing.T) {
	editorCmd := func(path string) tea.Cmd {
		return func() tea.Msg {
			return tui.EditFinishedMsg{Path: path, Err: nil}
		}
	}
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
	}
	items[0].Card.FilePath = "/tmp/test-card.md"

	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithEditorCmd(editorCmd))

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Error("e should return a command to run the editor")
	}
}

// TestEditReReadsCardFromDiskAfterEditor verifies that after the editor
// command completes, the card is re-read from disk and the view reflects
// the updated content.
func TestEditReReadsCardFromDiskAfterEditor(t *testing.T) {
	updatedFront := "Updated Q"
	cardReadFunc := func(path string) (*card.Card, error) {
		return &card.Card{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: updatedFront, Back: "A1", FilePath: path}, nil
	}

	editorCmd := func(path string) tea.Cmd {
		return func() tea.Msg {
			return tui.EditFinishedMsg{Path: path, Err: nil}
		}
	}

	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
	}
	items[0].Card.FilePath = "/tmp/test-card.md"

	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithEditorCmd(editorCmd), tui.WithCardReadFunc(cardReadFunc))

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = asReview(updated)

	if cmd == nil {
		t.Fatal("e should return a command")
	}

	msg := cmd()
	updated, _ = m.Update(msg)
	m = asReview(updated)

	view := m.View()
	if !strings.Contains(view, "Updated") {
		t.Errorf("after editor, view should show updated content, got:\n%s", view)
	}
}

// TestEditNonFatalErrorOnParseFailure verifies that when re-reading the card
// from disk fails, the session continues with the in-memory copy and a
// non-fatal error is shown.
func TestEditNonFatalErrorOnParseFailure(t *testing.T) {
	cardReadFunc := func(path string) (*card.Card, error) {
		return nil, fmt.Errorf("parse error")
	}

	editorCmd := func(path string) tea.Cmd {
		return func() tea.Msg {
			return tui.EditFinishedMsg{Path: path, Err: nil}
		}
	}

	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
	}
	items[0].Card.FilePath = "/tmp/test-card.md"

	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithEditorCmd(editorCmd), tui.WithCardReadFunc(cardReadFunc))

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = asReview(updated)
	msg := cmd()
	updated, _ = m.Update(msg)
	m = asReview(updated)

	view := m.View()
	if !strings.Contains(view, "Q1") {
		t.Errorf("after parse failure, should still show original content, got:\n%s", view)
	}
	if !strings.Contains(view, "error") {
		t.Errorf("should show non-fatal error message, got:\n%s", view)
	}
}

// TestUndoReversesLastRating verifies that pressing 'u' after rating a card
// decrements the index (returning to the undone card), decrements the rating
// count, and calls the undoFunc.
func TestUndoReversesLastRating(t *testing.T) {
	undoCalled := false
	undoFunc := func(entry store.LogEntry, cardPath string, c *card.Card) error {
		undoCalled = true
		return nil
	}

	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithUndoFunc(undoFunc))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)

	if m.CurrentIndex() != 1 {
		t.Fatalf("after rating, index should be 1, got %d", m.CurrentIndex())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = asReview(updated)

	if !undoCalled {
		t.Error("undo should call undoFunc")
	}
	if m.CurrentIndex() != 0 {
		t.Errorf("after undo, index should be 0, got %d", m.CurrentIndex())
	}
	stats := m.Stats()
	if stats.RatingCounts[3] != 0 {
		t.Errorf("after undo of Good rating, RatingCounts[3] should be 0, got %d", stats.RatingCounts[3])
	}
}

// TestUndoIsNoOpWhenNoRatingDone verifies that pressing 'u' before any
// rating has been made is a no-op.
func TestUndoIsNoOpWhenNoRatingDone(t *testing.T) {
	undoCalled := false
	undoFunc := func(entry store.LogEntry, cardPath string, c *card.Card) error {
		undoCalled = true
		return nil
	}

	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithUndoFunc(undoFunc))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = asReview(updated)

	if undoCalled {
		t.Error("undo before any rating should be a no-op")
	}
	if m.CurrentIndex() != 0 {
		t.Error("undo before any rating should not change index")
	}
}

// TestUndoIsSingleStepOnly verifies that pressing 'u' a second time after
// already undoing once is a no-op.
func TestUndoIsSingleStepOnly(t *testing.T) {
	undoCount := 0
	undoFunc := func(entry store.LogEntry, cardPath string, c *card.Card) error {
		undoCount++
		return nil
	}

	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithUndoFunc(undoFunc))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = asReview(updated)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

	if undoCount != 1 {
		t.Errorf("undo should only be possible once, got %d calls", undoCount)
	}
}

// TestUndoReturnsToShowFrontSide verifies that after undoing a rating, the
// undone card is shown on its front side (not back).
func TestUndoReturnsToShowFrontSide(t *testing.T) {
	undoFunc := func(entry store.LogEntry, cardPath string, c *card.Card) error {
		return nil
	}

	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc, tui.WithUndoFunc(undoFunc))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = asReview(updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = asReview(updated)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = asReview(updated)

	if m.ShowingBack() {
		t.Error("after undo, should show front side of undone card")
	}
}

// TestClozeNoHintShowsEllipsis checks that a cloze marker without a hint
// renders as [...] on the question side.
func TestClozeNoHintShowsEllipsis(t *testing.T) {
	items := []deck.ReviewItem{
		clozeItem("1", "The answer is {{c1::42}}.", "c1"),
	}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("dark"))
	view := m.View()
	if strings.Contains(view, "42") {
		t.Errorf("question should hide answer 42, got:\n%s", view)
	}
	if !strings.Contains(view, "...") {
		t.Errorf("question should show placeholder containing '...', got:\n%s", view)
	}
}

func TestReviewModelUsesProvidedStyle(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil, tui.WithRenderStyle("light"))
	if m.RenderStyle() != "light" {
		t.Errorf("RenderStyle() = %q, want %q", m.RenderStyle(), "light")
	}
}

func TestReviewModelDefaultStyleIsAuto(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil)
	if m.RenderStyle() != "auto" {
		t.Errorf("RenderStyle() = %q, want %q (default)", m.RenderStyle(), "auto")
	}
}
