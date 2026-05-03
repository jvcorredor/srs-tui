package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jvcorredor/srs-tui/internal/card"
	"github.com/jvcorredor/srs-tui/internal/tui"
)

func TestReviewFlipOnSpace(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards)
	if m.ShowingBack() {
		t.Error("should start showing front")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(tui.ReviewModel)
	if !m.ShowingBack() {
		t.Error("space should flip to back")
	}
}

func TestReviewFlipOnEnter(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ReviewModel)
	if !m.ShowingBack() {
		t.Error("enter should flip to back")
	}
}

func TestReviewQuitOnQ(t *testing.T) {
	cards := []*card.Card{
		{Meta: card.Meta{ID: "1", Type: card.Basic}, Front: "Q", Back: "A"},
	}
	m := tui.NewReviewModel(cards)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q should trigger quit")
	}
}
