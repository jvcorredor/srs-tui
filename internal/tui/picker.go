package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// DeckEntry describes a single deck shown in the picker.
type DeckEntry struct {
	Name     string
	Path     string
	DueCount int
}

// deckItem adapts a DeckEntry for the bubbles/list component.
type deckItem struct{ entry DeckEntry }

func (d deckItem) Title() string { return d.entry.Name }
func (d deckItem) Description() string {
	if d.entry.DueCount == 0 {
		return "no cards due"
	}
	if d.entry.DueCount == 1 {
		return "1 card due"
	}
	return fmt.Sprintf("%d cards due", d.entry.DueCount)
}
func (d deckItem) FilterValue() string { return d.entry.Name }

// OnSelectFunc is called when the user selects a deck in the picker.
// It returns the new tea.Model and tea.Cmd to replace the picker with.
type OnSelectFunc func(entry DeckEntry) (tea.Model, tea.Cmd)

// PickerModel is a Bubble Tea model that shows a list of decks with their
// due-card counts and lets the user pick one to review.
type PickerModel struct {
	list     list.Model
	onSelect OnSelectFunc
	empty    bool
	items    []DeckEntry
}

// SelectedIndex returns the index of the currently highlighted deck.
func (m PickerModel) SelectedIndex() int {
	if m.empty || len(m.items) == 0 {
		return 0
	}
	return m.list.Index()
}

// NewPickerModel creates a PickerModel. If decks is empty or nil, the picker
// shows an empty-state message. onSelect is called when the user selects a deck.
func NewPickerModel(decks []DeckEntry, onSelect OnSelectFunc) PickerModel {
	var items []list.Item
	for _, d := range decks {
		items = append(items, deckItem{entry: d})
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 40, 20)
	l.Title = "Decks"
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("esc"))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
			key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "up")),
		}
	}

	return PickerModel{
		list:     l,
		onSelect: onSelect,
		empty:    len(decks) == 0,
		items:    decks,
	}
}

// Init implements tea.Model.
func (m PickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.empty {
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j":
			m.list.CursorDown()
			return m, nil
		case "k":
			m.list.CursorUp()
			return m, nil
		case "enter":
			if m.onSelect != nil {
				i := m.list.SelectedItem()
				if i == nil {
					return m, nil
				}
				entry := i.(deckItem).entry
				return m.onSelect(entry)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m PickerModel) View() string {
	if m.empty {
		return "No decks found.\nRun srs init to get started, then srs new to create cards.\nPress q to quit."
	}
	return m.list.View()
}
