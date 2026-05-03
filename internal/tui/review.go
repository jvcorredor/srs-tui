package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/jvcorredor/srs-tui/internal/card"
)

type ReviewModel struct {
	cards       []*card.Card
	index       int
	showingBack bool
	renderer    *glamour.TermRenderer
}

func NewReviewModel(cards []*card.Card) ReviewModel {
	r, _ := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"))
	return ReviewModel{
		cards:    cards,
		renderer: r,
	}
}

func (m ReviewModel) ShowingBack() bool {
	return m.showingBack
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeySpace, tea.KeyEnter:
			m.showingBack = true
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ReviewModel) View() string {
	if len(m.cards) == 0 {
		return "No cards in this deck.\nPress q to quit."
	}
	c := m.cards[m.index]
	content := c.Front
	if m.showingBack {
		content = c.Back
	}
	rendered, _ := m.renderer.Render(content)
	return rendered
}
