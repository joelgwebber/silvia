package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"silvia/internal/llm"
)

// cacheEntry stores an entity with metadata about when it was cached
type cacheEntry struct {
	entity   *Entity
	loadedAt time.Time
}

// Manager handles graph operations and maintains consistency
type Manager struct {
	baseDir string
	mu      sync.RWMutex
	cache   map[string]*cacheEntry // Cache with timestamp tracking
}

// NewManager creates a new graph manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
		cache:   make(map[string]*cacheEntry),
	}
}

// LoadEntity loads an entity by ID
func (m *Manager) LoadEntity(id string) (*Entity, error) {
	filePath := m.getEntityPath(id)

	// Check if file exists and get its modification time
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat entity file %s: %w", id, err)
	}
	fileModTime := fileInfo.ModTime()

	// Check cache and validate it's not stale
	m.mu.RLock()
	if cached, ok := m.cache[id]; ok {
		// If cached version is newer than or equal to file modification time, use it
		if !cached.loadedAt.Before(fileModTime) {
			m.mu.RUnlock()
			return cached.entity, nil
		}
	}
	m.mu.RUnlock()

	// Load from disk (either not cached or stale)
	entity, err := LoadEntityFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity %s: %w", id, err)
	}

	// Update cache with current time
	m.mu.Lock()
	m.cache[id] = &cacheEntry{
		entity:   entity,
		loadedAt: time.Now(),
	}
	m.mu.Unlock()

	return entity, nil
}

// SaveEntity saves an entity and updates back-references
func (m *Manager) SaveEntity(entity *Entity) error {
	if err := entity.Validate(); err != nil {
		return fmt.Errorf("invalid entity: %w", err)
	}

	filePath := m.getEntityPath(entity.Metadata.ID)
	if err := SaveEntityToFile(entity, filePath); err != nil {
		return fmt.Errorf("failed to save entity: %w", err)
	}

	// Update cache
	m.mu.Lock()
	m.cache[entity.Metadata.ID] = &cacheEntry{
		entity:   entity,
		loadedAt: time.Now(),
	}
	m.mu.Unlock()

	// Update back-references in related entities
	if err := m.updateBackReferences(entity); err != nil {
		return fmt.Errorf("failed to update back-references: %w", err)
	}

	return nil
}

// EntityExists checks if an entity with the given ID exists
func (m *Manager) EntityExists(id string) bool {
	filePath := m.getEntityPath(id)
	_, err := os.Stat(filePath)
	return err == nil
}

// FindEntitiesByType returns all entities of a specific type
func (m *Manager) FindEntitiesByType(entityType EntityType) ([]*Entity, error) {
	var entities []*Entity

	graphDir := filepath.Join(m.baseDir, "graph")
	err := filepath.Walk(graphDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		entity, err := LoadEntityFromFile(path)
		if err != nil {
			// Log error but continue walking
			fmt.Printf("Warning: failed to load %s: %v\n", path, err)
			return nil
		}

		if entity.Metadata.Type == entityType {
			entities = append(entities, entity)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk graph directory: %w", err)
	}

	return entities, nil
}

// SearchEntities searches for entities by name or alias
func (m *Manager) SearchEntities(query string) ([]*Entity, error) {
	query = strings.ToLower(query)
	var matches []*Entity

	graphDir := filepath.Join(m.baseDir, "graph")
	err := filepath.Walk(graphDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		entity, err := LoadEntityFromFile(path)
		if err != nil {
			return nil
		}

		// Check title
		if strings.Contains(strings.ToLower(entity.Title), query) {
			matches = append(matches, entity)
			return nil
		}

		// Check aliases
		for _, alias := range entity.Metadata.Aliases {
			if strings.Contains(strings.ToLower(alias), query) {
				matches = append(matches, entity)
				return nil
			}
		}

		// Check ID
		if strings.Contains(strings.ToLower(entity.Metadata.ID), query) {
			matches = append(matches, entity)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	return matches, nil
}

// ListAllEntities returns all entities in the graph
func (m *Manager) ListAllEntities() ([]*Entity, error) {
	var entities []*Entity

	graphDir := filepath.Join(m.baseDir, "graph")
	err := filepath.Walk(graphDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		entity, err := LoadEntityFromFile(path)
		if err != nil {
			return nil // Skip invalid files
		}

		entities = append(entities, entity)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}

	return entities, nil
}

// RelatedEntitiesResult contains categorized related entities
type RelatedEntitiesResult struct {
	Entity         *Entity              // The source entity
	OutgoingByType map[string][]*Entity // Outgoing links grouped by type
	IncomingByType map[string][]*Entity // Incoming links grouped by type
	BrokenLinks    []string             // Links to non-existent entities
	All            []*Entity            // All valid related entities
}

// GetRelatedEntities returns all entities directly related to the given entity
func (m *Manager) GetRelatedEntities(entityID string) (*RelatedEntitiesResult, error) {
	entity, err := m.LoadEntity(entityID)
	if err != nil {
		return nil, err
	}

	result := &RelatedEntitiesResult{
		Entity:         entity,
		OutgoingByType: make(map[string][]*Entity),
		IncomingByType: make(map[string][]*Entity),
		BrokenLinks:    []string{},
		All:            []*Entity{},
	}

	// Track which entities we've seen
	seenEntities := make(map[string]*Entity)

	// Process all outgoing links
	outgoingLinks := entity.GetAllOutgoingLinks()
	for _, link := range outgoingLinks {
		if relEntity, err := m.LoadEntity(link.Target); err == nil {
			// Valid entity found
			if _, seen := seenEntities[link.Target]; !seen {
				seenEntities[link.Target] = relEntity
				result.All = append(result.All, relEntity)
			}
			// Add to categorized results
			result.OutgoingByType[link.Type] = append(result.OutgoingByType[link.Type], relEntity)
		} else if m.EntityExists(link.Target) == false {
			// Track broken links
			result.BrokenLinks = append(result.BrokenLinks, link.Target)
		}
	}

	// Process incoming relationships (back-references)
	for _, backRef := range entity.BackRefs {
		if relEntity, err := m.LoadEntity(backRef.Source); err == nil {
			// Valid entity found
			if _, seen := seenEntities[backRef.Source]; !seen {
				seenEntities[backRef.Source] = relEntity
				result.All = append(result.All, relEntity)
			}
			// Add to categorized results
			relType := backRef.Type
			if relType == "" {
				relType = "referenced_by"
			}
			result.IncomingByType[relType] = append(result.IncomingByType[relType], relEntity)
		}
	}

	return result, nil
}

// GetRelatedEntitiesSimple returns a simple list of related entities (for backward compatibility)
func (m *Manager) GetRelatedEntitiesSimple(entityID string) ([]*Entity, error) {
	result, err := m.GetRelatedEntities(entityID)
	if err != nil {
		return nil, err
	}
	return result.All, nil
}

// updateBackReferences updates back-references in entities that this entity points to
func (m *Manager) updateBackReferences(entity *Entity) error {
	// Get all outgoing links from this entity
	outgoingLinks := entity.GetAllOutgoingLinks()

	// Process each outgoing link
	for _, link := range outgoingLinks {
		if !m.EntityExists(link.Target) {
			// Target entity doesn't exist yet, skip
			continue
		}

		targetEntity, err := m.LoadEntity(link.Target)
		if err != nil {
			// Skip entities that can't be loaded
			continue
		}

		// Add back-reference with appropriate type
		targetEntity.AddBackReference(entity.Metadata.ID, link.Type, link.Note)

		// Save target entity
		filePath := m.getEntityPath(targetEntity.Metadata.ID)
		if err := SaveEntityToFile(targetEntity, filePath); err != nil {
			// Log error but continue with other references
			fmt.Printf("Warning: failed to save back-reference to %s: %v\n", link.Target, err)
			continue
		}

		// Update cache
		m.mu.Lock()
		m.cache[targetEntity.Metadata.ID] = &cacheEntry{
			entity:   targetEntity,
			loadedAt: time.Now(),
		}
		m.mu.Unlock()
	}

	// Note: We're not removing old back-references for now since we'd need to track
	// what changed. This could be added later with a more sophisticated diff system.

	return nil
}

// removeOldBackReferences removes back-references that are no longer valid
func (m *Manager) removeOldBackReferences(entity *Entity) error {
	// This would need to track previous state to know what to remove
	// For now, we'll implement a simpler version that recalculates all back-refs
	// In production, we'd want to track the diff between old and new relationships
	return nil
}

// getEntityPath returns the file path for an entity
func (m *Manager) getEntityPath(id string) string {
	return filepath.Join(m.baseDir, "graph", id+".md")
}

// InitializeDirectories creates the necessary directory structure
func (m *Manager) InitializeDirectories() error {
	dirs := []string{
		filepath.Join(m.baseDir, "graph"),
		filepath.Join(m.baseDir, "graph", "people"),
		filepath.Join(m.baseDir, "graph", "organizations"),
		filepath.Join(m.baseDir, "graph", "concepts"),
		filepath.Join(m.baseDir, "graph", "works"),
		filepath.Join(m.baseDir, "graph", "events"),
		filepath.Join(m.baseDir, "sources"),
		filepath.Join(m.baseDir, "sources", "bsky"),
		filepath.Join(m.baseDir, "sources", "web"),
		filepath.Join(m.baseDir, "sources", "pdfs"),
		filepath.Join(m.baseDir, "config"),
		filepath.Join(m.baseDir, ".silvia"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// ClearCache clears the in-memory entity cache
func (m *Manager) ClearCache() {
	m.mu.Lock()
	m.cache = make(map[string]*cacheEntry)
	m.mu.Unlock()
}

// RebuildAllBackReferences rebuilds all back-references in the graph
func (m *Manager) RebuildAllBackReferences() error {
	fmt.Println("Rebuilding all back-references...")

	// First, clear all existing back-references
	entities, err := m.ListAllEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities: %w", err)
	}

	// Clear back-references in all entities
	for _, entity := range entities {
		if len(entity.BackRefs) > 0 {
			entity.BackRefs = []BackReference{}
			filePath := m.getEntityPath(entity.Metadata.ID)
			if err := SaveEntityToFile(entity, filePath); err != nil {
				fmt.Printf("Warning: failed to clear back-refs for %s: %v\n", entity.Metadata.ID, err)
			}
		}
	}

	// Now rebuild all back-references
	processed := 0
	for _, entity := range entities {
		// Get all outgoing links
		outgoingLinks := entity.GetAllOutgoingLinks()

		// Add back-references to target entities
		for _, link := range outgoingLinks {
			if !m.EntityExists(link.Target) {
				continue
			}

			targetEntity, err := m.LoadEntity(link.Target)
			if err != nil {
				continue
			}

			// Add the back-reference
			targetEntity.AddBackReference(entity.Metadata.ID, link.Type, link.Note)

			// Save the target entity
			filePath := m.getEntityPath(targetEntity.Metadata.ID)
			if err := SaveEntityToFile(targetEntity, filePath); err != nil {
				fmt.Printf("Warning: failed to save back-ref to %s: %v\n", link.Target, err)
			}

			// Clear from cache to ensure fresh load next time
			m.mu.Lock()
			delete(m.cache, targetEntity.Metadata.ID)
			m.mu.Unlock()
		}

		processed++
		if processed%10 == 0 {
			fmt.Printf("  Processed %d/%d entities\n", processed, len(entities))
		}
	}

	// Clear the entire cache to ensure fresh loads
	m.ClearCache()

	fmt.Printf("Rebuilt back-references for %d entities\n", len(entities))
	return nil
}

// MergeEntities merges entity2 into entity1, updating all references
func (m *Manager) MergeEntities(ctx context.Context, entity1ID, entity2ID string, llmClient *llm.Client) error {
	// Load both entities
	entity1, err := m.LoadEntity(entity1ID)
	if err != nil {
		return fmt.Errorf("failed to load entity %s: %w", entity1ID, err)
	}

	entity2, err := m.LoadEntity(entity2ID)
	if err != nil {
		return fmt.Errorf("failed to load entity %s: %w", entity2ID, err)
	}

	fmt.Printf("Merging %s into %s...\n", entity2ID, entity1ID)

	// Use LLM to merge the content
	mergedContent, err := llmClient.MergeEntities(ctx, entity1.Content, entity2.Content, "")
	if err != nil {
		return fmt.Errorf("failed to merge content: %w", err)
	}

	// Create the merged entity
	merged := &Entity{
		Metadata: entity1.Metadata,
		Title:    entity1.Title,
		Content:  mergedContent,
		BackRefs: entity1.BackRefs, // Will be rebuilt
	}

	// Merge metadata
	// Combine aliases
	aliasSet := make(map[string]bool)
	for _, alias := range entity1.Metadata.Aliases {
		aliasSet[alias] = true
	}
	for _, alias := range entity2.Metadata.Aliases {
		aliasSet[alias] = true
	}
	// Add the old entity's ID as an alias
	aliasSet[entity2ID] = true

	merged.Metadata.Aliases = []string{}
	for alias := range aliasSet {
		merged.Metadata.Aliases = append(merged.Metadata.Aliases, alias)
	}

	// Combine sources
	sourceSet := make(map[string]bool)
	for _, source := range entity1.Metadata.Sources {
		sourceSet[source] = true
	}
	for _, source := range entity2.Metadata.Sources {
		sourceSet[source] = true
	}
	merged.Metadata.Sources = []string{}
	for source := range sourceSet {
		merged.Metadata.Sources = append(merged.Metadata.Sources, source)
	}

	// Combine tags
	tagSet := make(map[string]bool)
	for _, tag := range entity1.Metadata.Tags {
		tagSet[tag] = true
	}
	for _, tag := range entity2.Metadata.Tags {
		tagSet[tag] = true
	}
	merged.Metadata.Tags = []string{}
	for tag := range tagSet {
		merged.Metadata.Tags = append(merged.Metadata.Tags, tag)
	}

	// Update timestamp
	merged.Metadata.Updated = time.Now()

	// Find all entities that reference entity2 and update them to reference entity1
	fmt.Println("Updating references...")
	allEntities, err := m.ListAllEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities: %w", err)
	}

	updatedCount := 0
	for _, entity := range allEntities {
		if entity.Metadata.ID == entity1ID || entity.Metadata.ID == entity2ID {
			continue // Skip the entities being merged
		}

		modified := false

		// Check and update wiki-links in content
		if strings.Contains(entity.Content, fmt.Sprintf("[[%s]]", entity2ID)) {
			entity.Content = strings.ReplaceAll(entity.Content,
				fmt.Sprintf("[[%s]]", entity2ID),
				fmt.Sprintf("[[%s]]", entity1ID))
			modified = true
		}

		// Check and update sources
		for i, source := range entity.Metadata.Sources {
			if source == entity2ID {
				entity.Metadata.Sources[i] = entity1ID
				modified = true
			}
		}

		// Save if modified
		if modified {
			filePath := m.getEntityPath(entity.Metadata.ID)
			if err := SaveEntityToFile(entity, filePath); err != nil {
				fmt.Printf("Warning: failed to update references in %s: %v\n", entity.Metadata.ID, err)
			} else {
				updatedCount++
				// Clear from cache
				m.mu.Lock()
				delete(m.cache, entity.Metadata.ID)
				m.mu.Unlock()
			}
		}
	}

	fmt.Printf("Updated %d entities with new references\n", updatedCount)

	// Save the merged entity
	if err := m.SaveEntity(merged); err != nil {
		return fmt.Errorf("failed to save merged entity: %w", err)
	}

	// Delete entity2
	entity2Path := m.getEntityPath(entity2ID)
	if err := os.Remove(entity2Path); err != nil {
		return fmt.Errorf("failed to delete entity %s: %w", entity2ID, err)
	}

	// Clear entity2 from cache
	m.mu.Lock()
	delete(m.cache, entity2ID)
	m.mu.Unlock()

	// Rebuild back-references to ensure consistency
	fmt.Println("Rebuilding back-references...")
	if err := m.RebuildAllBackReferences(); err != nil {
		return fmt.Errorf("failed to rebuild back-references: %w", err)
	}

	fmt.Printf("Successfully merged %s into %s\n", entity2ID, entity1ID)
	return nil
}
