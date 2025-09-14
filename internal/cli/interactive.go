package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the interactive UI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// selectableItem represents an item that can be selected
type selectableItem struct {
	title       string
	description string
	selected    bool
	value       any
}

func (i selectableItem) Title() string       { return i.title }
func (i selectableItem) Description() string { return i.description }
func (i selectableItem) FilterValue() string { return i.title }

// multiSelectModel is the model for multi-select lists
type multiSelectModel struct {
	list     list.Model
	items    []selectableItem
	selected map[int]bool
	quitting bool
	aborted  bool
}

// Custom key bindings
type listKeyMap struct {
	toggleItem key.Binding
	selectAll  key.Binding
	selectNone key.Binding
	confirm    key.Binding
	abort      key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		toggleItem: key.NewBinding(
			key.WithKeys(" ", "x"),
			key.WithHelp("space/x", "toggle"),
		),
		selectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		selectNone: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "select none"),
		),
		confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		abort: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "cancel"),
		),
	}
}

func (m multiSelectModel) Init() tea.Cmd {
	return nil
}

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keys := newListKeyMap()
		switch {
		case key.Matches(msg, keys.toggleItem):
			// Toggle current item
			idx := m.list.Index()
			m.selected[idx] = !m.selected[idx]
			// Update the item to show selection state
			items := m.list.Items()
			if idx < len(items) {
				item := m.items[idx]
				if m.selected[idx] {
					item.title = "✓ " + strings.TrimPrefix(item.title, "✓ ")
					item.title = strings.TrimPrefix(item.title, "  ")
				} else {
					item.title = "  " + strings.TrimPrefix(item.title, "✓ ")
					item.title = strings.TrimPrefix(item.title, "  ")
				}
				m.items[idx] = item
				m.list.SetItems(itemsToList(m.items))
			}

		case key.Matches(msg, keys.selectAll):
			// Select all items
			for i := range m.items {
				m.selected[i] = true
				m.items[i].title = "✓ " + strings.TrimPrefix(m.items[i].title, "✓ ")
				m.items[i].title = strings.TrimPrefix(m.items[i].title, "  ")
			}
			m.list.SetItems(itemsToList(m.items))

		case key.Matches(msg, keys.selectNone):
			// Deselect all items
			for i := range m.items {
				m.selected[i] = false
				m.items[i].title = "  " + strings.TrimPrefix(m.items[i].title, "✓ ")
				m.items[i].title = strings.TrimPrefix(m.items[i].title, "  ")
			}
			m.list.SetItems(itemsToList(m.items))

		case key.Matches(msg, keys.confirm):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.abort):
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m multiSelectModel) View() string {
	if m.quitting {
		return ""
	}

	selectedCount := 0
	for _, v := range m.selected {
		if v {
			selectedCount++
		}
	}

	header := titleStyle.Render(fmt.Sprintf("Select sources to add to queue (%d selected)", selectedCount))
	help := helpStyle.Render("\n[space] toggle • [a] all • [n] none • [enter] confirm • [esc] cancel")

	return fmt.Sprintf("%s\n\n%s%s", header, m.list.View(), help)
}

func itemsToList(items []selectableItem) []list.Item {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	return listItems
}

// InteractiveSourceSelect shows an interactive multi-select list for sources
func InteractiveSourceSelect(sources []string, descriptions []string) ([]int, error) {
	// Create items
	items := make([]selectableItem, len(sources))
	for i, source := range sources {
		desc := ""
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		// Truncate long URLs for display
		displayURL := source
		if len(displayURL) > 60 {
			displayURL = displayURL[:57] + "..."
		}
		items[i] = selectableItem{
			title:       "  " + displayURL,
			description: desc,
			value:       source,
		}
	}

	// Create list
	const defaultHeight = 20
	l := list.New(itemsToList(items), list.NewDefaultDelegate(), 0, defaultHeight)
	l.Title = "Linked Sources"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	// Create model
	m := multiSelectModel{
		list:     l,
		items:    items,
		selected: make(map[int]bool),
	}

	// Run the program
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, err
	}

	// Get results
	finalModel := result.(multiSelectModel)
	if finalModel.aborted {
		return []int{}, nil
	}

	// Collect selected indices
	var selectedIndices []int
	for i, selected := range finalModel.selected {
		if selected {
			selectedIndices = append(selectedIndices, i)
		}
	}

	return selectedIndices, nil
}
