package graph

import (
	"context"
	"fmt"
	"strings"

	"silvia/internal/llm"
)

// GraphOperations provides high-level graph operations that can be shared
// between different interfaces (CLI, chat, API, etc.)
type GraphOperations struct {
	manager *Manager
	llm     *llm.Client
}

// NewGraphOperations creates a new graph operations instance
func NewGraphOperations(manager *Manager, llmClient *llm.Client) *GraphOperations {
	return &GraphOperations{
		manager: manager,
		llm:     llmClient,
	}
}

// ReadEntity loads an entity by ID
func (g *GraphOperations) ReadEntity(id string) (*Entity, error) {
	entity, err := g.manager.LoadEntity(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity %s: %w", id, err)
	}
	return entity, nil
}

// UpdateEntity updates an existing entity's content and metadata
func (g *GraphOperations) UpdateEntity(id string, content string, metadata *Metadata) (*Entity, error) {
	// Load existing entity
	entity, err := g.manager.LoadEntity(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity %s: %w", id, err)
	}

	// Update fields
	if content != "" {
		entity.Content = content
	}
	if metadata != nil {
		// Merge metadata (preserve existing values not in update)
		if metadata.Type != "" {
			entity.Metadata.Type = metadata.Type
		}
		if len(metadata.Aliases) > 0 {
			entity.Metadata.Aliases = metadata.Aliases
		}
		if len(metadata.Tags) > 0 {
			entity.Metadata.Tags = metadata.Tags
		}
		if len(metadata.Sources) > 0 {
			entity.Metadata.Sources = metadata.Sources
		}
	}

	// Save updated entity
	if err := g.manager.SaveEntity(entity); err != nil {
		return nil, fmt.Errorf("failed to save entity %s: %w", id, err)
	}

	return entity, nil
}

// CreateEntity creates a new entity
func (g *GraphOperations) CreateEntity(entityType EntityType, id, title, content string) (*Entity, error) {
	// Check if entity already exists
	if g.manager.EntityExists(id) {
		return nil, fmt.Errorf("entity %s already exists", id)
	}

	// Create new entity
	entity := NewEntity(id, entityType)
	entity.Title = title
	if content != "" {
		entity.Content = content
	}

	// Save entity
	if err := g.manager.SaveEntity(entity); err != nil {
		return nil, fmt.Errorf("failed to save entity: %w", err)
	}

	return entity, nil
}

// SearchEntities searches for entities matching a query
func (g *GraphOperations) SearchEntities(query string) ([]*Entity, error) {
	entities, err := g.manager.SearchEntities(query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return entities, nil
}

// CreateLink creates a relationship between two entities
func (g *GraphOperations) CreateLink(sourceID, relType, targetID string, note string) error {
	// Load source entity
	source, err := g.manager.LoadEntity(sourceID)
	if err != nil {
		return fmt.Errorf("failed to load source entity %s: %w", sourceID, err)
	}

	// Verify target exists
	if !g.manager.EntityExists(targetID) {
		return fmt.Errorf("target entity %s does not exist", targetID)
	}

	// Add relationship
	source.AddRelationship(relType, targetID, nil, note)

	// Save updated entity
	if err := g.manager.SaveEntity(source); err != nil {
		return fmt.Errorf("failed to save entity: %w", err)
	}

	return nil
}

// MergeEntities merges two entities into one
func (g *GraphOperations) MergeEntities(ctx context.Context, entity1ID, entity2ID string) (*Entity, error) {
	if g.llm == nil {
		return nil, fmt.Errorf("LLM client required for merge operation")
	}

	err := g.manager.MergeEntities(ctx, entity1ID, entity2ID, g.llm)
	if err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	// Load and return the merged entity
	return g.manager.LoadEntity(entity1ID)
}

// RenameEntity renames an entity and updates all references
func (g *GraphOperations) RenameEntity(oldID, newID string) error {
	err := g.manager.RenameEntity(oldID, newID)
	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	return nil
}

// GetRelatedEntities retrieves entities related to a given entity
func (g *GraphOperations) GetRelatedEntities(entityID string) (*RelatedEntitiesResult, error) {
	result, err := g.manager.GetRelatedEntities(entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get related entities: %w", err)
	}
	return result, nil
}

// ListEntitiesByType lists all entities of a specific type
func (g *GraphOperations) ListEntitiesByType(entityType EntityType) ([]*Entity, error) {
	entities, err := g.manager.FindEntitiesByType(entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}
	return entities, nil
}

// RebuildBackReferences rebuilds all back-references in the graph
func (g *GraphOperations) RebuildBackReferences() error {
	return g.manager.RebuildAllBackReferences()
}

// EntityExists checks if an entity exists
func (g *GraphOperations) EntityExists(id string) bool {
	return g.manager.EntityExists(id)
}

// GenerateEntityID generates a standardized entity ID from a name and type
func (g *GraphOperations) GenerateEntityID(name string, entityType EntityType) string {
	// Clean up the name
	cleanName := strings.ToLower(name)
	cleanName = strings.TrimSpace(cleanName)

	// Replace spaces and special characters with hyphens
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		"'", "",
		"\"", "",
		".", "",
		",", "",
		"!", "",
		"?", "",
		":", "",
		";", "",
		"(", "",
		")", "",
		"[", "",
		"]", "",
		"{", "",
		"}", "",
		"/", "-",
		"\\", "-",
	)
	cleanName = replacer.Replace(cleanName)

	// Remove multiple consecutive hyphens
	for strings.Contains(cleanName, "--") {
		cleanName = strings.ReplaceAll(cleanName, "--", "-")
	}

	// Trim hyphens from start and end
	cleanName = strings.Trim(cleanName, "-")

	// Construct the full ID with the type prefix
	return fmt.Sprintf("%s/%s", entityType, cleanName)
}
