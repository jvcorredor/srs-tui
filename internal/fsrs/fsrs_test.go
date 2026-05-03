package fsrs_test

import (
	"testing"
	"time"

	"github.com/jvcorredor/srs-tui/internal/fsrs"
)

func TestRateNewCardReturnsNextStateAndPreviews(t *testing.T) {
	card := fsrs.CardState{State: fsrs.StateNew}
	now := time.Now()

	next, previews, err := fsrs.Rate(card, 3, now)
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}
	if next.State == "" {
		t.Error("next state should not be empty after rating")
	}
	if next.Stability <= 0 {
		t.Error("stability should be positive after rating")
	}
	if len(previews) != 4 {
		t.Fatalf("previews length = %d, want 4", len(previews))
	}
	for _, p := range previews {
		if p.Rating < 1 || p.Rating > 4 {
			t.Errorf("preview rating = %d, want 1-4", p.Rating)
		}
		if p.Interval < 0 {
			t.Errorf("preview interval = %v, want non-negative", p.Interval)
		}
	}
}

func TestRateNewCardAgainGoesToLearning(t *testing.T) {
	card := fsrs.CardState{State: fsrs.StateNew}
	now := time.Now()

	next, _, err := fsrs.Rate(card, 1, now)
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}
	if next.State != fsrs.StateLearning {
		t.Errorf("rating Again on new card: state = %q, want %q", next.State, fsrs.StateLearning)
	}
}

func TestRateNewCardGoodGoesToLearning(t *testing.T) {
	card := fsrs.CardState{State: fsrs.StateNew}
	now := time.Now()

	next, _, err := fsrs.Rate(card, 3, now)
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}
	if next.State != fsrs.StateLearning {
		t.Errorf("rating Good on new card: state = %q, want %q", next.State, fsrs.StateLearning)
	}
}

func TestRateNewCardEasyGoesToReview(t *testing.T) {
	card := fsrs.CardState{State: fsrs.StateNew}
	now := time.Now()

	next, _, err := fsrs.Rate(card, 4, now)
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}
	if next.State != fsrs.StateReview {
		t.Errorf("rating Easy on new card: state = %q, want %q", next.State, fsrs.StateReview)
	}
}

func TestPreviewReturnsFourIntervals(t *testing.T) {
	card := fsrs.CardState{State: fsrs.StateNew}
	now := time.Now()

	previews := fsrs.Preview(card, now)
	if len(previews) != 4 {
		t.Fatalf("Preview() returned %d previews, want 4", len(previews))
	}
	ratings := map[int]bool{}
	for _, p := range previews {
		ratings[p.Rating] = true
		if p.Interval < 0 {
			t.Errorf("preview for rating %d has negative interval: %v", p.Rating, p.Interval)
		}
	}
	for _, r := range []int{1, 2, 3, 4} {
		if !ratings[r] {
			t.Errorf("missing preview for rating %d", r)
		}
	}
}

func TestNormalizeState(t *testing.T) {
	if fsrs.NormalizeState("") != fsrs.StateNew {
		t.Errorf("NormalizeState(\"\") = %q, want %q", fsrs.NormalizeState(""), fsrs.StateNew)
	}
	if fsrs.NormalizeState("review") != fsrs.StateReview {
		t.Errorf("NormalizeState(\"review\") = %q, want %q", fsrs.NormalizeState("review"), fsrs.StateReview)
	}
}
