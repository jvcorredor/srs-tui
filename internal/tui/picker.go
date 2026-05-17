package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jvcorredor/srs-tui/internal/slug"
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

// PickerOption configures a PickerModel at construction time.
type PickerOption func(*PickerModel)

// WithDecksRoot sets the directory under which the picker creates new deck
// directories when the user presses N. Without it, deck creation is disabled.
func WithDecksRoot(root string) PickerOption {
	return func(m *PickerModel) { m.decksRoot = root }
}

// PickerModel is a Bubble Tea model that shows a list of decks with their
// due-card counts and lets the user pick one to review.
type PickerModel struct {
	list      list.Model
	onSelect  OnSelectFunc
	empty     bool
	items     []DeckEntry
	decksRoot string
	creating  bool
	nameInput textinput.Model
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
func NewPickerModel(decks []DeckEntry, onSelect OnSelectFunc, opts ...PickerOption) PickerModel {
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
			key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "new deck")),
		}
	}

	ti := textinput.New()
	ti.Prompt = "New deck: "
	ti.Placeholder = "deck name"

	m := PickerModel{
		list:      l,
		onSelect:  onSelect,
		empty:     len(decks) == 0,
		items:     decks,
		nameInput: ti,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// Init implements tea.Model.
func (m PickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.creating {
		return m.updateCreating(msg)
	}

	keyMsg, isKey := msg.(tea.KeyMsg)

	if m.empty {
		if isKey {
			switch keyMsg.String() {
			case "q":
				return m, tea.Quit
			case "N":
				return m.startCreating()
			}
		}
		return m, nil
	}

	if isKey {
		switch keyMsg.String() {
		case "q":
			return m, tea.Quit
		case "N":
			return m.startCreating()
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

// startCreating opens the inline deck-name textinput overlay.
func (m PickerModel) startCreating() (tea.Model, tea.Cmd) {
	m.creating = true
	m.nameInput.SetValue("")
	cmd := m.nameInput.Focus()
	return m, cmd
}

// updateCreating handles input while the deck-name textinput overlay is active.
func (m PickerModel) updateCreating(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.creating = false
			m.nameInput.Blur()
			m.nameInput.SetValue("")
			return m, nil
		case "enter":
			name := slug.Slugify(m.nameInput.Value())
			if name == "" {
				// Nothing slug-worthy was typed; keep the overlay open.
				return m, nil
			}
			m.creating = false
			m.nameInput.Blur()
			m.nameInput.SetValue("")
			return m.createDeck(name)
		}
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// createDeck creates the deck directory for the slugified name and adds it to
// the picker list with the cursor on it. If a deck with that name already
// exists, the cursor simply jumps to it.
func (m PickerModel) createDeck(name string) (tea.Model, tea.Cmd) {
	path := filepath.Join(m.decksRoot, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		// Surfacing errors would require a status line the picker does not
		// yet have; for now leave the list unchanged on failure.
		return m, nil
	}

	for i, e := range m.items {
		if e.Name == name {
			m.list.Select(i)
			return m, nil
		}
	}

	entry := DeckEntry{Name: name, Path: path}
	m.items = append(m.items, entry)
	idx := len(m.items) - 1
	cmd := m.list.InsertItem(idx, deckItem{entry: entry})
	m.list.Select(idx)
	m.empty = false
	return m, cmd
}

// View implements tea.Model.
func (m PickerModel) View() string {
	if m.creating {
		if m.empty {
			return m.nameInput.View() + "\nPress Esc to cancel."
		}
		return m.list.View() + "\n" + m.nameInput.View()
	}
	if m.empty {
		return "No decks found. Press `N` to create one, or run `srs init` to get started. Press `q` to quit."
	}
	return m.list.View()
}
