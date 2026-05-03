// Package fsrs wraps the Free Spaced Repetition Scheduler for card scheduling.
package fsrs

import (
	"fmt"
	"time"

	fsrslib "github.com/open-spaced-repetition/go-fsrs/v4"
)

type State string

const (
	StateNew        State = "new"
	StateLearning   State = "learning"
	StateReview     State = "review"
	StateRelearning State = "relearning"
)

type CardState struct {
	State      State
	Due       time.Time
	Stability  float64
	Difficulty float64
	Reps      int
	Lapses    int
}

type IntervalPreview struct {
	Rating   int
	State    State
	Due      time.Time
	Interval time.Duration
}

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

func fromLibCard(c fsrslib.Card) CardState {
	return CardState{
		State:      stateFromLib(c.State),
		Due:       c.Due,
		Stability:  c.Stability,
		Difficulty: c.Difficulty,
		Reps:      int(c.Reps),
		Lapses:    int(c.Lapses),
	}
}

var defaultFSRS = fsrslib.NewFSRS(fsrslib.DefaultParam())

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

func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func NormalizeState(s string) State {
	if s == "" {
		return StateNew
	}
	return State(s)
}

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
