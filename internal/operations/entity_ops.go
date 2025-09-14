package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"silvia/internal/graph"
	"silvia/internal/llm"
)

// EntityOps handles all entity-related operations
type EntityOps struct {
	graph   *graph.Manager
	llm     *llm.Client
	dataDir string
}

// NewEntityOps creates a new entity operations handler
func NewEntityOps(graphManager *graph.Manager, llmClient *llm.Client, dataDir string) *EntityOps {
	return &EntityOps{
		graph:   graphManager,
		llm:     llmClient,
		dataDir: dataDir,
	}
}

// ReadEntity loads an entity by ID
func (e *EntityOps) ReadEntity(id string) (*graph.Entity, error) {
	entity, err := e.graph.LoadEntity(id)
	if err != nil {
		return nil, NewOperationError("read entity", id, err)
	}
	return entity, nil
}

// CreateEntity creates a new entity
func (e *EntityOps) CreateEntity(entityType, id, title, content string) (*graph.Entity, error) {
	// Validate entity type
	validTypes := []string{"person", "organization", "concept", "work", "event"}
	isValid := slices.Contains(validTypes, entityType)
	if !isValid {
		return nil, NewOperationError("create entity", id,
			fmt.Errorf("invalid entity type: %s (must be one of: %s)",
				entityType, strings.Join(validTypes, ", ")))
	}

	// Check if entity already exists
	if e.graph.EntityExists(id) {
		return nil, NewOperationError("create entity", id, fmt.Errorf("entity already exists"))
	}

	// Create the entity
	entity := &graph.Entity{
		Title:   title,
		Content: content,
		Metadata: graph.Metadata{
			ID:      id,
			Type:    graph.EntityType(entityType),
			Created: time.Now(),
			Updated: time.Now(),
		},
	}

	// Save the entity
	if err := e.graph.SaveEntity(entity); err != nil {
		return nil, NewOperationError("create entity", id, err)
	}

	return entity, nil
}

// UpdateEntity updates an existing entity's content
func (e *EntityOps) UpdateEntity(id string, title, content string) (*graph.Entity, error) {
	// Load existing entity
	entity, err := e.graph.LoadEntity(id)
	if err != nil {
		return nil, NewOperationError("update entity", id, err)
	}

	// Update fields if provided
	if title != "" {
		entity.Title = title
	}
	if content != "" {
		entity.Content = content
	}

	// Update timestamp
	entity.Metadata.Updated = time.Now()

	// Save the updated entity
	if err := e.graph.SaveEntity(entity); err != nil {
		return nil, NewOperationError("update entity", id, err)
	}

	return entity, nil
}

// MergeEntities merges entity2 into entity1, updating all references
func (e *EntityOps) MergeEntities(ctx context.Context, entity1ID, entity2ID string) (*MergeResult, error) {
	// Validate both entities exist
	entity1, err := e.graph.LoadEntity(entity1ID)
	if err != nil {
		return nil, NewOperationError("merge entities", entity1ID, fmt.Errorf("first entity not found: %w", err))
	}

	entity2, err := e.graph.LoadEntity(entity2ID)
	if err != nil {
		return nil, NewOperationError("merge entities", entity2ID, fmt.Errorf("second entity not found: %w", err))
	}

	// Track which files get updated
	updatedFiles := []string{}

	// Get all entities that reference entity2
	referencingEntities := e.getEntitiesReferencingTarget(entity2ID)

	// Update all references from entity2 to entity1
	for _, refEntityID := range referencingEntities {
		refEntity, err := e.graph.LoadEntity(refEntityID)
		if err != nil {
			continue
		}

		// Replace wiki-links in content
		oldLink := fmt.Sprintf("[[%s]]", entity2ID)
		newLink := fmt.Sprintf("[[%s]]", entity1ID)
		if strings.Contains(refEntity.Content, oldLink) {
			refEntity.Content = strings.ReplaceAll(refEntity.Content, oldLink, newLink)
			if err := e.graph.SaveEntity(refEntity); err == nil {
				updatedFiles = append(updatedFiles, refEntityID)
			}
		}
	}

	// Merge the actual entities using LLM if available
	if e.llm != nil {
		mergedContent, err := e.llm.MergeEntities(ctx, entity1.Content, entity2.Content, "")
		if err == nil {
			entity1.Content = mergedContent
		}
	} else {
		// Simple concatenation if no LLM
		entity1.Content = entity1.Content + "\n\n" + entity2.Content
	}

	// Merge metadata
	if len(entity1.Metadata.Sources) == 0 {
		entity1.Metadata.Sources = entity2.Metadata.Sources
	} else if len(entity2.Metadata.Sources) > 0 {
		// Merge sources, avoiding duplicates
		sourceMap := make(map[string]bool)
		for _, s := range entity1.Metadata.Sources {
			sourceMap[s] = true
		}
		for _, s := range entity2.Metadata.Sources {
			if !sourceMap[s] {
				entity1.Metadata.Sources = append(entity1.Metadata.Sources, s)
			}
		}
	}

	// Update timestamp
	entity1.Metadata.Updated = time.Now()

	// Save the merged entity
	if err := e.graph.SaveEntity(entity1); err != nil {
		return nil, NewOperationError("merge entities", entity1ID, fmt.Errorf("failed to save merged entity: %w", err))
	}

	// Delete entity2 (by removing its file)
	if err := e.deleteEntity(entity2ID); err != nil {
		return nil, NewOperationError("merge entities", entity2ID, fmt.Errorf("failed to delete source entity: %w", err))
	}

	// Rebuild back-references
	if err := e.graph.RebuildAllBackReferences(); err != nil {
		// Non-fatal error, log but continue
		fmt.Printf("Warning: failed to rebuild back-references: %v\n", err)
	}

	return &MergeResult{
		MergedEntity:    entity1,
		UpdatedFiles:    updatedFiles,
		DeletedEntityID: entity2ID,
	}, nil
}

// RenameEntity renames an entity and updates all references
func (e *EntityOps) RenameEntity(oldID, newID string) (*RenameResult, error) {
	// Validate old entity exists
	if _, err := e.graph.LoadEntity(oldID); err != nil {
		return nil, NewOperationError("rename entity", oldID, fmt.Errorf("entity not found"))
	}

	// Check if new ID already exists
	if e.graph.EntityExists(newID) {
		return nil, NewOperationError("rename entity", newID, fmt.Errorf("target entity already exists"))
	}

	// Use graph's rename function which handles all the complexity
	if err := e.graph.RenameEntity(oldID, newID); err != nil {
		return nil, NewOperationError("rename entity", oldID, err)
	}

	// Get list of updated files (all entities that referenced the old ID)
	updatedFiles := e.getEntitiesReferencingTarget(newID)

	return &RenameResult{
		OldID:        oldID,
		NewID:        newID,
		UpdatedFiles: updatedFiles,
	}, nil
}

// DeleteEntity deletes an entity
func (e *EntityOps) DeleteEntity(id string) error {
	// Check if entity exists
	if !e.graph.EntityExists(id) {
		return NewOperationError("delete entity", id, fmt.Errorf("entity not found"))
	}

	// Check for references
	referencingEntities := e.getEntitiesReferencingTarget(id)
	if len(referencingEntities) > 0 {
		return NewOperationError("delete entity", id,
			fmt.Errorf("cannot delete: entity is referenced by %d other entities", len(referencingEntities)))
	}

	// Delete the entity
	if err := e.deleteEntity(id); err != nil {
		return NewOperationError("delete entity", id, err)
	}

	return nil
}

// RefineEntity uses LLM to refine an entity's content based on its sources
func (e *EntityOps) RefineEntity(ctx context.Context, id string, guidance string) (*graph.Entity, error) {
	if e.llm == nil {
		return nil, NewOperationError("refine entity", id, fmt.Errorf("LLM client not available"))
	}

	// Load the entity
	entity, err := e.graph.LoadEntity(id)
	if err != nil {
		return nil, NewOperationError("refine entity", id, err)
	}

	// Build context from sources
	var sourceContext strings.Builder
	sourceContext.WriteString("Current entity content:\n")
	sourceContext.WriteString(entity.Content)
	sourceContext.WriteString("\n\nSources:\n")

	for _, sourceURL := range entity.Metadata.Sources {
		// In a full implementation, we would fetch and include source content
		sourceContext.WriteString(fmt.Sprintf("- %s\n", sourceURL))
	}

	// Create refinement prompt
	systemPrompt := "You are a knowledge graph entity refiner. Improve the entity description based on the sources and guidance provided. Preserve all wiki-links in [[entity-id]] format."

	userPrompt := sourceContext.String()
	if guidance != "" {
		userPrompt += "\n\nGuidance: " + guidance
	}

	// Get refined content from LLM
	refinedContent, err := e.llm.CompleteWithSystem(ctx, systemPrompt, userPrompt, "")
	if err != nil {
		return nil, NewOperationError("refine entity", id, fmt.Errorf("LLM refinement failed: %w", err))
	}

	// Update entity content
	entity.Content = refinedContent

	// Update timestamp
	entity.Metadata.Updated = time.Now()

	// Save the refined entity
	if err := e.graph.SaveEntity(entity); err != nil {
		return nil, NewOperationError("refine entity", id, err)
	}

	return entity, nil
}

// Helper methods

// getEntitiesReferencingTarget finds all entities that reference a target entity
func (e *EntityOps) getEntitiesReferencingTarget(targetID string) []string {
	referencingEntities := []string{}

	// Walk through all entities and check for references
	graphDir := filepath.Join(e.dataDir, "graph")
	filepath.Walk(graphDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check if it contains a wiki-link to the target
		if strings.Contains(string(content), fmt.Sprintf("[[%s]]", targetID)) {
			// Extract entity ID from path
			relPath, _ := filepath.Rel(graphDir, path)
			entityID := strings.TrimSuffix(relPath, ".md")
			referencingEntities = append(referencingEntities, entityID)
		}

		return nil
	})

	return referencingEntities
}

// deleteEntity removes an entity file from disk
func (e *EntityOps) deleteEntity(id string) error {
	// Construct file path
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid entity ID format: %s", id)
	}

	filePath := filepath.Join(e.dataDir, "graph", parts[0], parts[1]+".md")

	// Remove the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete entity file: %w", err)
	}

	return nil
}
