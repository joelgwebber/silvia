package operations

import (
	"time"

	"silvia/internal/graph"
)

// Operations provides a unified interface for all business operations
type Operations struct {
	Entity *EntityOps
	Queue  *QueueOps
	Source *SourceOps
	Search *SearchOps
	LLM    *LLMOps
}

// MergeResult contains the result of merging two entities
type MergeResult struct {
	MergedEntity    *graph.Entity
	UpdatedFiles    []string
	DeletedEntityID string
}

// RenameResult contains the result of renaming an entity
type RenameResult struct {
	OldID        string
	NewID        string
	UpdatedFiles []string
}

// IngestResult contains the result of ingesting a source
type IngestResult struct {
	SourceURL         string
	ArchivedPath      string
	ExtractedEntities []ExtractedEntity
	ExtractedLinks    []ExtractedLink
	ProcessingTime    time.Duration
}

// ExtractedEntity represents an entity extracted from a source
type ExtractedEntity struct {
	ID          string
	Type        string
	Name        string
	Description string
	Content     string
	IsNew       bool
	WasUpdated  bool
}

// ExtractedLink represents a link extracted from a source
type ExtractedLink struct {
	URL         string
	Title       string
	Description string
	Category    string
	Relevance   string
}

// SearchResult contains search results
type SearchResult struct {
	Query   string
	Results []SearchMatch
	Total   int
}

// SearchMatch represents a single search result
type SearchMatch struct {
	Entity  *graph.Entity
	Score   float64
	Snippet string
	Matches []string
}

// RelatedEntitiesResult contains related entities organized by relationship type
type RelatedEntitiesResult struct {
	Entity         *graph.Entity
	OutgoingByType map[string][]*graph.Entity
	IncomingByType map[string][]*graph.Entity
	BrokenLinks    []string
	All            []*graph.Entity
}

// QueueItem represents an item in the source queue
type QueueItem struct {
	URL         string
	Priority    int
	AddedAt     time.Time
	FromSource  string
	Description string
	Status      string
}

// QueueStatus represents the current state of the queue
type QueueStatus struct {
	Items      []QueueItem
	TotalCount int
	ByPriority map[int]int
	OldestItem *QueueItem
	NewestItem *QueueItem
}

// OperationError represents an error from an operation
type OperationError struct {
	Operation string
	Entity    string
	Cause     error
}

func (e *OperationError) Error() string {
	if e.Entity != "" {
		return e.Operation + " failed for " + e.Entity + ": " + e.Cause.Error()
	}
	return e.Operation + " failed: " + e.Cause.Error()
}

// NewOperationError creates a new operation error
func NewOperationError(operation, entity string, cause error) error {
	return &OperationError{
		Operation: operation,
		Entity:    entity,
		Cause:     cause,
	}
}
