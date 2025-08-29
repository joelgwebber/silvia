package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// QueueAction represents an action to take on queue items
type QueueAction int

const (
	QueueActionNone QueueAction = iota
	QueueActionProcess
	QueueActionSkip
	QueueActionPreview
	QueueActionReorder
)

// queueItem represents a queue item for display
type queueItem struct {
	source      *QueuedSource
	action      QueueAction
	title       string
	description string
}

func (i queueItem) Title() string       { return i.title }
func (i queueItem) Description() string { return i.description }
func (i queueItem) FilterValue() string { return i.source.URL }

// queueExplorerModel is the model for queue exploration
type queueExplorerModel struct {
	list          list.Model
	items         []queueItem
	selectedItems map[int]QueueAction
	quitting      bool
	aborted       bool
	message       string
	ctx           context.Context
	cli           *CLI
}

// queueKeyMap defines custom key bindings for queue explorer
type queueKeyMap struct {
	process    key.Binding
	skip       key.Binding
	preview    key.Binding
	toggleMode key.Binding
	execute    key.Binding
	quit       key.Binding
	help       key.Binding
}

func newQueueKeyMap() *queueKeyMap {
	return &queueKeyMap{
		process: key.NewBinding(
			key.WithKeys("p", "enter"),
			key.WithHelp("p/enter", "process"),
		),
		skip: key.NewBinding(
			key.WithKeys("s", "delete"),
			key.WithHelp("s/del", "skip"),
		),
		preview: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "preview"),
		),
		toggleMode: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "batch mode"),
		),
		execute: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "execute batch"),
		),
		quit: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "quit"),
		),
		help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

func (m queueExplorerModel) Init() tea.Cmd {
	return nil
}

func (m queueExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keys := newQueueKeyMap()

		// Handle key presses
		switch {
		case key.Matches(msg, keys.quit):
			m.quitting = true
			m.aborted = true
			return m, tea.Quit

		case key.Matches(msg, keys.process):
			idx := m.list.Index()
			if idx < len(m.items) {
				// Mark for processing
				m.selectedItems[idx] = QueueActionProcess
				m.updateItemDisplay(idx)
				// Move to next item
				if idx < len(m.items)-1 {
					m.list.CursorDown()
				}
			}

		case key.Matches(msg, keys.skip):
			idx := m.list.Index()
			if idx < len(m.items) {
				// Mark for skipping
				m.selectedItems[idx] = QueueActionSkip
				m.updateItemDisplay(idx)
				// Move to next item
				if idx < len(m.items)-1 {
					m.list.CursorDown()
				}
			}

		case key.Matches(msg, keys.preview):
			idx := m.list.Index()
			if idx < len(m.items) {
				// Mark for preview
				m.selectedItems[idx] = QueueActionPreview
				m.updateItemDisplay(idx)
			}

		case key.Matches(msg, keys.execute):
			// Execute all marked actions
			m.quitting = true
			return m, tea.Quit

		case msg.String() == "ctrl+c":
			m.quitting = true
			m.aborted = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m queueExplorerModel) View() string {
	if m.quitting {
		return ""
	}

	// Count actions
	processCount := 0
	skipCount := 0
	previewCount := 0
	for _, action := range m.selectedItems {
		switch action {
		case QueueActionProcess:
			processCount++
		case QueueActionSkip:
			skipCount++
		case QueueActionPreview:
			previewCount++
		}
	}

	header := HeaderStyle.Render(fmt.Sprintf("Queue Explorer (%d items)", len(m.items)))

	status := fmt.Sprintf("\n%s Process: %d | %s Skip: %d | %s Preview: %d",
		SuccessStyle.Render("✓"),
		processCount,
		WarningStyle.Render("⊘"),
		skipCount,
		InfoStyle.Render("👁"),
		previewCount,
	)

	help := helpStyle.Render("\n[p] process • [s] skip • [v] preview • [x] execute • [q] quit")

	return fmt.Sprintf("%s%s\n\n%s%s", header, status, m.list.View(), help)
}

func (m *queueExplorerModel) updateItemDisplay(idx int) {
	if idx >= len(m.items) {
		return
	}

	item := &m.items[idx]
	action := m.selectedItems[idx]

	// Update title with action indicator
	baseTitle := item.source.URL
	if len(baseTitle) > 60 {
		baseTitle = baseTitle[:57] + "..."
	}

	switch action {
	case QueueActionProcess:
		item.title = SuccessStyle.Render("✓ ") + URLStyle.Render(baseTitle)
	case QueueActionSkip:
		item.title = WarningStyle.Render("⊘ ") + DimStyle.Render(baseTitle)
	case QueueActionPreview:
		item.title = InfoStyle.Render("👁 ") + URLStyle.Render(baseTitle)
	default:
		item.title = "  " + URLStyle.Render(baseTitle)
	}

	// Update the list
	items := make([]list.Item, len(m.items))
	for i := range m.items {
		items[i] = m.items[i]
	}
	m.list.SetItems(items)
}

// InteractiveQueueExplorer shows an interactive interface for managing the queue
func (c *CLI) InteractiveQueueExplorer(ctx context.Context) error {
	queueItems := c.queue.GetAll()
	if len(queueItems) == 0 {
		fmt.Println(DimStyle.Render("Queue is empty."))
		return nil
	}

	// Create items for display
	items := make([]queueItem, len(queueItems))
	for i, source := range queueItems {
		displayURL := source.URL
		if len(displayURL) > 60 {
			displayURL = displayURL[:57] + "..."
		}

		desc := fmt.Sprintf("[%s]", FormatPriority(source.Priority))
		if source.Description != "" {
			desc += " - " + source.Description
		}

		items[i] = queueItem{
			source:      source,
			action:      QueueActionNone,
			title:       "  " + URLStyle.Render(displayURL),
			description: desc,
		}
	}

	// Create list items
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Create list
	const defaultHeight = 20
	l := list.New(listItems, list.NewDefaultDelegate(), 0, defaultHeight)
	l.Title = "Queue Management"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.SetStatusBarItemName("source", "sources")

	// Create model
	m := queueExplorerModel{
		list:          l,
		items:         items,
		selectedItems: make(map[int]QueueAction),
		ctx:           ctx,
		cli:           c,
	}

	// Run the program
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	// Process results
	finalModel := result.(queueExplorerModel)
	if finalModel.aborted {
		fmt.Println(InfoStyle.Render("Queue exploration cancelled."))
		return nil
	}

	// Execute actions
	return c.executeQueueActions(ctx, finalModel.items, finalModel.selectedItems)
}

// executeQueueActions processes the selected queue actions
func (c *CLI) executeQueueActions(ctx context.Context, items []queueItem, actions map[int]QueueAction) error {
	// Count actions
	totalActions := 0
	for _, action := range actions {
		if action != QueueActionNone {
			totalActions++
		}
	}

	if totalActions == 0 {
		fmt.Println(DimStyle.Render("No actions selected."))
		return nil
	}

	fmt.Println()
	fmt.Println(HeaderStyle.Render(fmt.Sprintf("Executing %d actions", totalActions)))
	fmt.Println()

	processedCount := 0
	skippedCount := 0
	errors := []string{}

	for i, item := range items {
		action := actions[i]
		if action == QueueActionNone {
			continue
		}

		switch action {
		case QueueActionProcess:
			fmt.Printf("%s %s\n",
				InfoStyle.Render("Processing:"),
				URLStyle.Render(item.source.URL))

			// Remove from queue
			c.queue.Remove(item.source.URL)

			// Process the source
			if err := c.ingestSource(ctx, item.source.URL); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", item.source.URL, err))
				fmt.Println(FormatError(fmt.Sprintf("Failed: %v", err)))
			} else {
				processedCount++
			}
			fmt.Println()

		case QueueActionSkip:
			fmt.Printf("%s %s\n",
				WarningStyle.Render("Skipping:"),
				DimStyle.Render(item.source.URL))

			// Remove from queue
			c.queue.Remove(item.source.URL)
			skippedCount++

		case QueueActionPreview:
			fmt.Printf("%s %s\n",
				InfoStyle.Render("Preview:"),
				URLStyle.Render(item.source.URL))

			// Preview implementation
			if err := c.previewSource(ctx, item.source.URL); err != nil {
				fmt.Println(FormatWarning(fmt.Sprintf("Preview failed: %v", err)))
			}
			fmt.Println()
		}
	}

	// Save queue after changes
	c.queue.SaveToFile()

	// Summary
	fmt.Println()
	fmt.Println(HeaderStyle.Render("Summary"))
	if processedCount > 0 {
		fmt.Println(FormatSuccess(fmt.Sprintf("Processed %d sources", processedCount)))
	}
	if skippedCount > 0 {
		fmt.Println(FormatWarning(fmt.Sprintf("Skipped %d sources", skippedCount)))
	}
	if len(errors) > 0 {
		fmt.Println(FormatError(fmt.Sprintf("Failed %d sources", len(errors))))
		for _, err := range errors {
			fmt.Printf("  %s\n", DimStyle.Render(err))
		}
	}

	remainingCount := c.queue.Len()
	if remainingCount > 0 {
		fmt.Println(InfoStyle.Render(fmt.Sprintf("Queue has %d remaining sources", remainingCount)))
	} else {
		fmt.Println(DimStyle.Render("Queue is now empty"))
	}

	return nil
}

// previewSource shows a preview of a source without ingesting it
func (c *CLI) previewSource(ctx context.Context, url string) error {
	// Fetch the source
	source, err := c.sources.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch source: %w", err)
	}

	// Display preview
	fmt.Println()
	fmt.Println(SubheaderStyle.Render("Title: ") + source.Title)
	fmt.Println(SubheaderStyle.Render("URL: ") + URLStyle.Render(source.URL))

	// Show first part of content
	content := source.Content
	if len(content) > 500 {
		content = content[:500] + "..."
	}
	fmt.Println()
	fmt.Println(SubheaderStyle.Render("Content Preview:"))
	fmt.Println(DimStyle.Render(content))

	// Show links found
	if len(source.Links) > 0 {
		fmt.Println()
		fmt.Println(SubheaderStyle.Render(fmt.Sprintf("Links Found (%d):", len(source.Links))))
		previewCount := 5
		if len(source.Links) < previewCount {
			previewCount = len(source.Links)
		}
		for i := 0; i < previewCount; i++ {
			fmt.Printf("  %s %s\n",
				DimStyle.Render("•"),
				URLStyle.Render(source.Links[i]))
		}
		if len(source.Links) > previewCount {
			fmt.Printf("  %s\n", DimStyle.Render(fmt.Sprintf("... and %d more", len(source.Links)-previewCount)))
		}
	}

	return nil
}
