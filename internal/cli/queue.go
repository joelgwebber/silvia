package cli

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SourcePriority represents the priority level of a source
type SourcePriority int

const (
	PriorityLow SourcePriority = iota
	PriorityMedium
	PriorityHigh
)

// QueuedSource represents a source waiting to be processed
type QueuedSource struct {
	URL         string         `json:"url"`
	Priority    SourcePriority `json:"priority"`
	AddedAt     time.Time      `json:"added_at"`
	FromSource  string         `json:"from_source,omitempty"`
	Description string         `json:"description,omitempty"`
	index       int            // Used by heap
}

// SourceQueue manages sources to be explored
type SourceQueue struct {
	items    []*QueuedSource
	itemMap  map[string]bool // Track URLs already in queue
	filePath string
}

// NewSourceQueue creates a new source queue
func NewSourceQueue() *SourceQueue {
	return &SourceQueue{
		items:   make([]*QueuedSource, 0),
		itemMap: make(map[string]bool),
	}
}

// LoadQueue loads the queue from disk
func (q *SourceQueue) LoadFromFile(filePath string) error {
	q.filePath = filePath

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's OK
			return nil
		}
		return fmt.Errorf("failed to read queue file: %w", err)
	}

	var items []*QueuedSource
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("failed to parse queue file: %w", err)
	}

	q.items = items
	q.itemMap = make(map[string]bool)
	for _, item := range items {
		q.itemMap[item.URL] = true
	}

	// Re-heapify
	heap.Init(q)

	return nil
}

// SaveQueue saves the queue to disk
func (q *SourceQueue) SaveToFile() error {
	if q.filePath == "" {
		return nil // No file path set
	}

	// Ensure directory exists
	dir := filepath.Dir(q.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(q.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	if err := os.WriteFile(q.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	return nil
}

// Add adds a source to the queue
func (q *SourceQueue) Add(url string, priority SourcePriority, fromSource, description string) bool {
	if q.itemMap[url] {
		return false // Already in queue
	}

	item := &QueuedSource{
		URL:         url,
		Priority:    priority,
		AddedAt:     time.Now(),
		FromSource:  fromSource,
		Description: description,
	}

	heap.Push(q, item)
	q.itemMap[url] = true
	return true
}

// PopItem removes and returns the highest priority source
func (q *SourceQueue) PopItem() *QueuedSource {
	if q.Len() == 0 {
		return nil
	}
	item := heap.Pop(q).(*QueuedSource)
	delete(q.itemMap, item.URL)
	return item
}

// Peek returns the highest priority source without removing it
func (q *SourceQueue) Peek() *QueuedSource {
	if q.Len() == 0 {
		return nil
	}
	return q.items[0]
}

// GetAll returns all queued sources in priority order
func (q *SourceQueue) GetAll() []*QueuedSource {
	// Make a copy to avoid modifying the heap
	result := make([]*QueuedSource, len(q.items))
	copy(result, q.items)

	// Sort by priority (heap is min-heap, but we want high priority first)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if q.Less(j, i) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Remove removes a specific URL from the queue
func (q *SourceQueue) Remove(url string) bool {
	if !q.itemMap[url] {
		return false
	}

	// Find and remove the item
	for i, item := range q.items {
		if item.URL == url {
			heap.Remove(q, i)
			delete(q.itemMap, url)
			return true
		}
	}

	return false
}

// Clear removes all items from the queue
func (q *SourceQueue) Clear() {
	q.items = make([]*QueuedSource, 0)
	q.itemMap = make(map[string]bool)
}

// Contains checks if a URL is already in the queue
func (q *SourceQueue) Contains(url string) bool {
	return q.itemMap[url]
}

// Heap interface implementation
func (q *SourceQueue) Len() int { return len(q.items) }

func (q *SourceQueue) Less(i, j int) bool {
	// Higher priority comes first
	if q.items[i].Priority != q.items[j].Priority {
		return q.items[i].Priority > q.items[j].Priority
	}
	// If same priority, older items come first
	return q.items[i].AddedAt.Before(q.items[j].AddedAt)
}

func (q *SourceQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	q.items[i].index = i
	q.items[j].index = j
}

func (q *SourceQueue) Push(x any) {
	item := x.(*QueuedSource)
	item.index = len(q.items)
	q.items = append(q.items, item)
}

func (q *SourceQueue) Pop() any {
	old := q.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	q.items = old[0 : n-1]
	return item
}

// Priority string representations
func (p SourcePriority) String() string {
	switch p {
	case PriorityHigh:
		return "HIGH"
	case PriorityMedium:
		return "MED"
	case PriorityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// ParsePriority parses a priority string
func ParsePriority(s string) SourcePriority {
	switch s {
	case "high", "HIGH", "h":
		return PriorityHigh
	case "medium", "MEDIUM", "med", "m":
		return PriorityMedium
	case "low", "LOW", "l":
		return PriorityLow
	default:
		return PriorityMedium
	}
}
