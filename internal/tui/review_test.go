// Package tui_test contains integration tests for the review TUI.
//
// Tests exercise the ReviewModel through its public Update and View methods
// rather than internal state, matching the project’s behavior-first testing
// philosophy.
package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/deck"
	"github.com/jvcorredor/srs-tui/internal/fsrs"
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
	m := tui.NewReviewModel(items, nil)
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
	m := tui.NewReviewModel(items, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !asReview(updated).ShowingBack() {
		t.Error("enter should flip to back")
	}
}

// TestReviewQuitOnQ verifies that pressing 'q' returns a tea.Quit command.
func TestReviewQuitOnQ(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit")
	}
}

// TestReviewQuitOnQWhenDone verifies that pressing 'q' quits even after the
// session is complete (m.done == true).
func TestReviewQuitOnQWhenDone(t *testing.T) {
	items := []deck.ReviewItem{basicItem("1", "Q", "A")}
	m := tui.NewReviewModel(items, fakeRateFunc)
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
func fakeRateFunc(item *deck.ReviewItem, rating int, now time.Time) (fsrs.CardState, []fsrs.IntervalPreview, error) {
	next := fsrs.CardState{State: fsrs.StateLearning, Stability: 1.5}
	previews := []fsrs.IntervalPreview{
		{Rating: 1, State: fsrs.StateLearning, Interval: 1 * time.Minute},
		{Rating: 2, State: fsrs.StateLearning, Interval: 5 * time.Minute},
		{Rating: 3, State: fsrs.StateLearning, Interval: 10 * time.Minute},
		{Rating: 4, State: fsrs.StateReview, Interval: 24 * time.Hour},
	}
	return next, previews, nil
}

// TestRatingKeyAdvancesCard checks that rating a flipped card moves the
// session to the next card and resets the view to the front side.
func TestRatingKeyAdvancesCard(t *testing.T) {
	items := []deck.ReviewItem{
		basicItem("1", "Q1", "A1"),
		basicItem("2", "Q2", "A2"),
	}
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)

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
	m := tui.NewReviewModel(items, nil)
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
	m := tui.NewReviewModel(items, nil)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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
	m := tui.NewReviewModel(items, fakeRateFunc)
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

// TestClozeNoHintShowsEllipsis checks that a cloze marker without a hint
// renders as [...] on the question side.
func TestClozeNoHintShowsEllipsis(t *testing.T) {
	items := []deck.ReviewItem{
		clozeItem("1", "The answer is {{c1::42}}.", "c1"),
	}
	m := tui.NewReviewModel(items, nil)
	view := m.View()
	if strings.Contains(view, "42") {
		t.Errorf("question should hide answer 42, got:\n%s", view)
	}
	if !strings.Contains(view, "...") {
		t.Errorf("question should show placeholder containing '...', got:\n%s", view)
	}
}
