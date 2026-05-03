// Package fsrs provides scheduling logic for spaced-repetition cards using the
// FSRS (Free Spaced Repetition Scheduler) algorithm. It wraps the open-spaced-repetition
// go-fsrs library with domain-specific types and helpers used by the application.
package fsrs

import (
	"fmt"
	"time"

	fsrslib "github.com/open-spaced-repetition/go-fsrs/v4"
)

// State represents the learning stage of a card in the FSRS algorithm.
type State string

const (
	// StateNew indicates the card has never been reviewed.
	StateNew State = "new"
	// StateLearning indicates the card is in the initial learning phase.
	StateLearning State = "learning"
	// StateReview indicates the card is in the regular review phase.
	StateReview State = "review"
	// StateRelearning indicates the card has lapsed and is being re-learned.
	StateRelearning State = "relearning"
)

// CardState holds the scheduling state of a card at a point in time.
type CardState struct {
	State      State
	Due        time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
}

// IntervalPreview shows the projected next state and interval for a given
// user rating without actually updating the card.
type IntervalPreview struct {
	Rating   int
	State    State
	Due      time.Time
	Interval time.Duration
}

// stateFromLib converts a go-fsrs State to the package State type.
func stateFromLib(s fsrslib.State) State {
	switch s {
	case fsrslib.New:
		return StateNew
	case fsrslib.Learning:
		return StateLearning
	case fsrslib.Review:
		return StateReview
	case fsrslib.Relearning:
		return StateRelearning
	}
	return ""
}

// stateToLib converts a package State to the go-fsrs State type.
func stateToLib(s State) fsrslib.State {
	switch s {
	case StateNew:
		return fsrslib.New
	case StateLearning:
		return fsrslib.Learning
	case StateReview:
		return fsrslib.Review
	case StateRelearning:
		return fsrslib.Relearning
	}
	return fsrslib.New
}

// toLibCard converts a CardState to the go-fsrs Card type.
func toLibCard(c CardState) fsrslib.Card {
	return fsrslib.Card{
		Due:        c.Due,
		Stability:  c.Stability,
		Difficulty: c.Difficulty,
		Reps:       uint64(c.Reps),
		Lapses:     uint64(c.Lapses),
		State:      stateToLib(c.State),
	}
}

// fromLibCard converts a go-fsrs Card to the package CardState type.
func fromLibCard(c fsrslib.Card) CardState {
	return CardState{
		State:      stateFromLib(c.State),
		Due:        c.Due,
		Stability:  c.Stability,
		Difficulty: c.Difficulty,
		Reps:       int(c.Reps),
		Lapses:     int(c.Lapses),
	}
}

// defaultFSRS is the shared FSRS scheduler instance configured with default
// parameters.
var defaultFSRS = fsrslib.NewFSRS(fsrslib.DefaultParam())

// Preview returns the possible next states and intervals for a card if it were
// reviewed right now. The returned slice contains one entry for each of the
// four FSRS ratings (Again, Hard, Good, Easy). It does not modify the card.
func Preview(card CardState, now time.Time) []IntervalPreview {
	libCard := toLibCard(card)
	log := defaultFSRS.Repeat(libCard, now)

	previews := make([]IntervalPreview, 0, 4)
	for _, r := range []fsrslib.Rating{fsrslib.Again, fsrslib.Hard, fsrslib.Good, fsrslib.Easy} {
		info, ok := log[r]
		if !ok {
			continue
		}
		interval := info.Card.Due.Sub(now)
		if interval < 0 {
			interval = 0
		}
		previews = append(previews, IntervalPreview{
			Rating:   int(r),
			State:    stateFromLib(info.Card.State),
			Due:      info.Card.Due,
			Interval: interval,
		})
	}
	return previews
}

// ParseTime parses an RFC3339 string into a time.Time. It returns the zero
// value for an empty string or on parse failure.
func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NormalizeState converts a raw string into a State. It returns StateNew for
// an empty string.
func NormalizeState(s string) State {
	if s == "" {
		return StateNew
	}
	return State(s)
}

// Rate applies a user rating (1–4) to a card at the given time and returns the
// updated card state, a preview of all four rating outcomes, and any error.
// Rating values map to FSRS: 1 = Again, 2 = Hard, 3 = Good, 4 = Easy.
func Rate(card CardState, rating int, now time.Time) (CardState, []IntervalPreview, error) {
	if rating < 1 || rating > 4 {
		return CardState{}, nil, fmt.Errorf("fsrs: rating %d out of range [1,4]", rating)
	}

	libCard := toLibCard(card)
	log := defaultFSRS.Repeat(libCard, now)

	previews := make([]IntervalPreview, 0, 4)
	for _, r := range []fsrslib.Rating{fsrslib.Again, fsrslib.Hard, fsrslib.Good, fsrslib.Easy} {
		info, ok := log[r]
		if !ok {
			continue
		}
		interval := info.Card.Due.Sub(now)
		if interval < 0 {
			interval = 0
		}
		previews = append(previews, IntervalPreview{
			Rating:   int(r),
			State:    stateFromLib(info.Card.State),
			Due:      info.Card.Due,
			Interval: interval,
		})
	}

	chosen := fsrslib.Rating(rating)
	info, ok := log[chosen]
	if !ok {
		return CardState{}, nil, fmt.Errorf("fsrs: no scheduling info for rating %d", rating)
	}

	return fromLibCard(info.Card), previews, nil
}
