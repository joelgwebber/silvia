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
		modified := targetEntity.AddBackReference(entity.Metadata.ID, link.Type, link.Note)

		// Only save if the entity was actually modified
		if modified {
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

	entities, err := m.ListAllEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities: %w", err)
	}

	// Build a map to track new back-references for each entity
	newBackRefs := make(map[string][]BackReference)
	for _, entity := range entities {
		newBackRefs[entity.Metadata.ID] = []BackReference{}
	}

	// Compute all back-references
	for _, entity := range entities {
		outgoingLinks := entity.GetAllOutgoingLinks()
		for _, link := range outgoingLinks {
			if !m.EntityExists(link.Target) {
				continue
			}
			// Add this back-reference to the target entity's list
			newBackRefs[link.Target] = append(newBackRefs[link.Target], BackReference{
				Source: entity.Metadata.ID,
				Type:   link.Type,
				Note:   link.Note,
			})
		}
	}

	// Now update only entities whose back-references have changed
	updatedCount := 0
	for _, entity := range entities {
		newRefs := newBackRefs[entity.Metadata.ID]
		
		// Check if back-references have changed
		if !backReferencesEqual(entity.BackRefs, newRefs) {
			entity.BackRefs = newRefs
			entity.Metadata.Updated = time.Now()
			
			filePath := m.getEntityPath(entity.Metadata.ID)
			if err := SaveEntityToFile(entity, filePath); err != nil {
				fmt.Printf("Warning: failed to update %s: %v\n", entity.Metadata.ID, err)
			} else {
				updatedCount++
			}
			
			// Clear from cache
			m.mu.Lock()
			delete(m.cache, entity.Metadata.ID)
			m.mu.Unlock()
		}
	}

	// Clear the entire cache to ensure fresh loads
	m.ClearCache()
	fmt.Printf("Updated back-references for %d entities (out of %d total)\n", updatedCount, len(entities))
	return nil
}

// backReferencesEqual compares two slices of back-references for equality
func backReferencesEqual(a, b []BackReference) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Create maps for efficient comparison
	aMap := make(map[string]BackReference)
	for _, ref := range a {
		key := ref.Source + "|" + ref.Type + "|" + ref.Note
		aMap[key] = ref
	}
	
	for _, ref := range b {
		key := ref.Source + "|" + ref.Type + "|" + ref.Note
		if _, exists := aMap[key]; !exists {
			return false
		}
	}
	
	return true
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

// RenameEntity renames an entity and updates all references throughout the graph
func (m *Manager) RenameEntity(oldID, newID string) error {
	// Validate the new ID format
	if !strings.Contains(newID, "/") {
		return fmt.Errorf("invalid entity ID format: must be 'type/name' (e.g., 'people/john-doe')")
	}

	// Check if old entity exists
	oldEntity, err := m.LoadEntity(oldID)
	if err != nil {
		return fmt.Errorf("entity not found: %s", oldID)
	}

	// Check if new ID already exists
	if m.EntityExists(newID) {
		return fmt.Errorf("entity already exists: %s", newID)
	}

	// Extract the type from both IDs to ensure they match
	oldParts := strings.SplitN(oldID, "/", 2)
	newParts := strings.SplitN(newID, "/", 2)
	if len(oldParts) != 2 || len(newParts) != 2 {
		return fmt.Errorf("invalid entity ID format")
	}
	if oldParts[0] != newParts[0] {
		return fmt.Errorf("cannot change entity type during rename (from %s to %s)", oldParts[0], newParts[0])
	}

	fmt.Printf("Renaming %s to %s...\n", oldID, newID)

	// Create the renamed entity
	renamedEntity := &Entity{
		Metadata:      oldEntity.Metadata,
		Title:         oldEntity.Title,
		Content:       oldEntity.Content,
		Relationships: oldEntity.Relationships,
		BackRefs:      oldEntity.BackRefs,
	}

	// Update the entity ID
	renamedEntity.Metadata.ID = newID

	// Add the old ID as an alias if not already present
	hasOldIDAsAlias := false
	for _, alias := range renamedEntity.Metadata.Aliases {
		if alias == oldID {
			hasOldIDAsAlias = true
			break
		}
	}
	if !hasOldIDAsAlias {
		renamedEntity.Metadata.Aliases = append(renamedEntity.Metadata.Aliases, oldID)
	}

	// Update timestamp
	renamedEntity.Metadata.Updated = time.Now()

	// Find and update all references to the old ID
	fmt.Println("Updating references throughout the graph...")
	allEntities, err := m.ListAllEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities: %w", err)
	}

	updatedCount := 0
	for _, entity := range allEntities {
		if entity.Metadata.ID == oldID {
			continue // Skip the entity being renamed
		}

		modified := false

		// Check and update wiki-links in content
		if strings.Contains(entity.Content, fmt.Sprintf("[[%s]]", oldID)) {
			entity.Content = strings.ReplaceAll(entity.Content,
				fmt.Sprintf("[[%s]]", oldID),
				fmt.Sprintf("[[%s]]", newID))
			modified = true
		}

		// Check and update sources
		for i, source := range entity.Metadata.Sources {
			if source == oldID {
				entity.Metadata.Sources[i] = newID
				modified = true
			}
		}

		// Check and update back-references
		for i, backRef := range entity.BackRefs {
			if backRef.Source == oldID {
				entity.BackRefs[i].Source = newID
				modified = true
			}
		}

		// Check and update relationships
		for i, rel := range entity.Relationships {
			if rel.Target == oldID {
				entity.Relationships[i].Target = newID
				modified = true
			}
		}

		// Save if modified
		if modified {
			entity.Metadata.Updated = time.Now()
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

	// Save the renamed entity with the new ID
	newPath := m.getEntityPath(newID)
	if err := SaveEntityToFile(renamedEntity, newPath); err != nil {
		return fmt.Errorf("failed to save renamed entity: %w", err)
	}

	// Delete the old entity file
	oldPath := m.getEntityPath(oldID)
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("failed to delete old entity file: %w", err)
	}

	// Update cache
	m.mu.Lock()
	delete(m.cache, oldID)
	m.cache[newID] = &cacheEntry{
		entity:   renamedEntity,
		loadedAt: time.Now(),
	}
	m.mu.Unlock()

	fmt.Printf("Successfully renamed %s to %s\n", oldID, newID)
	return nil
}

// MoveEntity moves an entity to a new ID, allowing type changes
func (m *Manager) MoveEntity(oldID, newID string) error {
	// Validate the new ID format
	if !strings.Contains(newID, "/") {
		return fmt.Errorf("invalid entity ID format: must be 'type/name' (e.g., 'sources/article-name')")
	}

	// Check if old entity exists
	oldEntity, err := m.LoadEntity(oldID)
	if err != nil {
		return fmt.Errorf("entity not found: %s", oldID)
	}

	// Check if new ID already exists
	if m.EntityExists(newID) {
		return fmt.Errorf("entity already exists: %s", newID)
	}

	// Extract the type from new ID
	newParts := strings.SplitN(newID, "/", 2)
	if len(newParts) != 2 {
		return fmt.Errorf("invalid entity ID format")
	}
	newType := newParts[0]

	fmt.Printf("Moving %s to %s...\n", oldID, newID)

	// Create the moved entity
	movedEntity := &Entity{
		Metadata:      oldEntity.Metadata,
		Title:         oldEntity.Title,
		Content:       oldEntity.Content,
		Relationships: oldEntity.Relationships,
		BackRefs:      oldEntity.BackRefs,
	}

	// Update the entity ID and type
	movedEntity.Metadata.ID = newID
	movedEntity.Metadata.Type = EntityType(newType)

	// Add the old ID as an alias if not already present
	hasOldIDAsAlias := false
	for _, alias := range movedEntity.Metadata.Aliases {
		if alias == oldID {
			hasOldIDAsAlias = true
			break
		}
	}
	if !hasOldIDAsAlias {
		movedEntity.Metadata.Aliases = append(movedEntity.Metadata.Aliases, oldID)
	}

	// Update timestamp
	movedEntity.Metadata.Updated = time.Now()

	// Find and update all references to the old ID
	fmt.Println("Updating references throughout the graph...")
	allEntities, err := m.ListAllEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities: %w", err)
	}

	updatedCount := 0
	for _, entity := range allEntities {
		if entity.Metadata.ID == oldID {
			continue // Skip the entity being moved
		}

		modified := false

		// Update wiki-links in content
		if strings.Contains(entity.Content, "[["+oldID+"]]") {
			entity.Content = strings.ReplaceAll(entity.Content, "[["+oldID+"]]", "[["+newID+"]]")
			modified = true
		}

		// Check and update sources
		for i, source := range entity.Metadata.Sources {
			if source == oldID {
				entity.Metadata.Sources[i] = newID
				modified = true
			}
		}

		// Check and update back references
		for i, backRef := range entity.BackRefs {
			if backRef.Source == oldID {
				entity.BackRefs[i].Source = newID
				modified = true
			}
		}

		// Check and update relationships
		for i, rel := range entity.Relationships {
			if rel.Target == oldID {
				entity.Relationships[i].Target = newID
				modified = true
			}
		}

		// Save if modified
		if modified {
			entity.Metadata.Updated = time.Now()
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

	// Save the moved entity with the new ID
	newPath := m.getEntityPath(newID)
	if err := SaveEntityToFile(movedEntity, newPath); err != nil {
		return fmt.Errorf("failed to save moved entity: %w", err)
	}

	// Delete the old entity file
	oldPath := m.getEntityPath(oldID)
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("failed to delete old entity file: %w", err)
	}

	// Update cache
	m.mu.Lock()
	delete(m.cache, oldID)
	m.cache[newID] = &cacheEntry{
		entity:   movedEntity,
		loadedAt: time.Now(),
	}
	m.mu.Unlock()

	fmt.Printf("Successfully moved %s to %s\n", oldID, newID)
	return nil
}
