package operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// QueueOps handles source queue operations
type QueueOps struct {
	dataDir   string
	queuePath string
}

// NewQueueOps creates a new queue operations handler
func NewQueueOps(dataDir string) *QueueOps {
	return &QueueOps{
		dataDir:   dataDir,
		queuePath: filepath.Join(dataDir, ".silvia", "queue.json"),
	}
}

// GetQueue returns the current queue status
func (q *QueueOps) GetQueue() (*QueueStatus, error) {
	items, err := q.loadQueue()
	if err != nil {
		return nil, NewOperationError("get queue", "", err)
	}

	if len(items) == 0 {
		return &QueueStatus{
			Items:      []QueueItem{},
			TotalCount: 0,
			ByPriority: map[int]int{0: 0, 1: 0, 2: 0},
		}, nil
	}

	// Convert internal items to QueueItem
	queueItems := make([]QueueItem, len(items))
	byPriority := map[int]int{0: 0, 1: 0, 2: 0}

	var oldest, newest *QueueItem

	for i, item := range items {
		qi := QueueItem{
			URL:         item.URL,
			Priority:    item.Priority,
			AddedAt:     item.AddedAt,
			FromSource:  item.FromSource,
			Description: item.Description,
			Status:      "pending",
		}
		queueItems[i] = qi
		byPriority[item.Priority]++

		if oldest == nil || qi.AddedAt.Before(oldest.AddedAt) {
			oldest = &qi
		}
		if newest == nil || qi.AddedAt.After(newest.AddedAt) {
			newest = &qi
		}
	}

	return &QueueStatus{
		Items:      queueItems,
		TotalCount: len(queueItems),
		ByPriority: byPriority,
		OldestItem: oldest,
		NewestItem: newest,
	}, nil
}

// AddToQueue adds a source to the queue
func (q *QueueOps) AddToQueue(url string, priority int, fromSource, description string) error {
	if url == "" {
		return NewOperationError("add to queue", url, fmt.Errorf("URL cannot be empty"))
	}

	if priority < 0 || priority > 2 {
		return NewOperationError("add to queue", url, fmt.Errorf("invalid priority: %d (must be 0-2)", priority))
	}

	items, err := q.loadQueue()
	if err != nil {
		return NewOperationError("add to queue", url, err)
	}

	// Check if already in queue
	for _, item := range items {
		if item.URL == url {
			return NewOperationError("add to queue", url, fmt.Errorf("already in queue"))
		}
	}

	// Add new item
	newItem := &queuedSource{
		URL:         url,
		Priority:    priority,
		AddedAt:     time.Now(),
		FromSource:  fromSource,
		Description: description,
	}

	items = append(items, newItem)

	// Sort by priority (high to low) then by time (old to new)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].AddedAt.Before(items[j].AddedAt)
	})

	if err := q.saveQueue(items); err != nil {
		return NewOperationError("add to queue", url, err)
	}

	return nil
}

// RemoveFromQueue removes a source from the queue
func (q *QueueOps) RemoveFromQueue(url string) error {
	if url == "" {
		return NewOperationError("remove from queue", url, fmt.Errorf("URL cannot be empty"))
	}

	items, err := q.loadQueue()
	if err != nil {
		return NewOperationError("remove from queue", url, err)
	}

	// Find and remove the item
	found := false
	newItems := make([]*queuedSource, 0, len(items))
	for _, item := range items {
		if item.URL == url {
			found = true
		} else {
			newItems = append(newItems, item)
		}
	}

	if !found {
		return NewOperationError("remove from queue", url, fmt.Errorf("not found in queue"))
	}

	if err := q.saveQueue(newItems); err != nil {
		return NewOperationError("remove from queue", url, err)
	}

	return nil
}

// GetNextItem returns the highest priority item from the queue
func (q *QueueOps) GetNextItem() (*QueueItem, error) {
	items, err := q.loadQueue()
	if err != nil {
		return nil, NewOperationError("get next item", "", err)
	}

	if len(items) == 0 {
		return nil, nil
	}

	// Items are already sorted by priority
	item := items[0]

	return &QueueItem{
		URL:         item.URL,
		Priority:    item.Priority,
		AddedAt:     item.AddedAt,
		FromSource:  item.FromSource,
		Description: item.Description,
		Status:      "pending",
	}, nil
}

// ProcessNextItem gets and removes the next item from the queue
func (q *QueueOps) ProcessNextItem() (*QueueItem, error) {
	item, err := q.GetNextItem()
	if err != nil {
		return nil, err
	}

	if item == nil {
		return nil, nil
	}

	// Remove from queue
	if err := q.RemoveFromQueue(item.URL); err != nil {
		return nil, err
	}

	return item, nil
}

// ClearQueue removes all items from the queue
func (q *QueueOps) ClearQueue() error {
	if err := q.saveQueue([]*queuedSource{}); err != nil {
		return NewOperationError("clear queue", "", err)
	}
	return nil
}

// UpdatePriority changes the priority of an item in the queue
func (q *QueueOps) UpdatePriority(url string, newPriority int) error {
	if newPriority < 0 || newPriority > 2 {
		return NewOperationError("update priority", url, fmt.Errorf("invalid priority: %d (must be 0-2)", newPriority))
	}

	items, err := q.loadQueue()
	if err != nil {
		return NewOperationError("update priority", url, err)
	}

	found := false
	for _, item := range items {
		if item.URL == url {
			item.Priority = newPriority
			found = true
			break
		}
	}

	if !found {
		return NewOperationError("update priority", url, fmt.Errorf("not found in queue"))
	}

	// Re-sort
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].AddedAt.Before(items[j].AddedAt)
	})

	if err := q.saveQueue(items); err != nil {
		return NewOperationError("update priority", url, err)
	}

	return nil
}

// Internal queue item structure (matches CLI's structure for compatibility)
type queuedSource struct {
	URL         string    `json:"url"`
	Priority    int       `json:"priority"`
	AddedAt     time.Time `json:"added_at"`
	FromSource  string    `json:"from_source,omitempty"`
	Description string    `json:"description,omitempty"`
}

// loadQueue loads the queue from disk
func (q *QueueOps) loadQueue() ([]*queuedSource, error) {
	data, err := os.ReadFile(q.queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, return empty queue
			return []*queuedSource{}, nil
		}
		return nil, fmt.Errorf("failed to read queue file: %w", err)
	}

	var items []*queuedSource
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse queue file: %w", err)
	}

	return items, nil
}

// saveQueue saves the queue to disk
func (q *QueueOps) saveQueue(items []*queuedSource) error {
	// Ensure directory exists
	dir := filepath.Dir(q.queuePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	if err := os.WriteFile(q.queuePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	return nil
}
