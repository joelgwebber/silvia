package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager handles graph operations and maintains consistency
type Manager struct {
	baseDir string
	mu      sync.RWMutex
	cache   map[string]*Entity // Simple in-memory cache
}

// NewManager creates a new graph manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
		cache:   make(map[string]*Entity),
	}
}

// LoadEntity loads an entity by ID
func (m *Manager) LoadEntity(id string) (*Entity, error) {
	m.mu.RLock()
	if cached, ok := m.cache[id]; ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	filePath := m.getEntityPath(id)
	entity, err := LoadEntityFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity %s: %w", id, err)
	}

	m.mu.Lock()
	m.cache[id] = entity
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
	m.cache[entity.Metadata.ID] = entity
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

// GetRelatedEntities returns all entities directly related to the given entity
func (m *Manager) GetRelatedEntities(entityID string) ([]*Entity, error) {
	entity, err := m.LoadEntity(entityID)
	if err != nil {
		return nil, err
	}

	relatedIDs := make(map[string]bool)

	// Collect outgoing relationships
	for _, rel := range entity.Relationships {
		relatedIDs[rel.Target] = true
	}

	// Collect incoming relationships
	for _, backRef := range entity.BackRefs {
		relatedIDs[backRef.Source] = true
	}

	// Load related entities
	var related []*Entity
	for id := range relatedIDs {
		if relEntity, err := m.LoadEntity(id); err == nil {
			related = append(related, relEntity)
		}
	}

	return related, nil
}

// updateBackReferences updates back-references in entities that this entity points to
func (m *Manager) updateBackReferences(entity *Entity) error {
	// First, remove old back-references from entities that are no longer linked
	if err := m.removeOldBackReferences(entity); err != nil {
		return err
	}

	// Then, add/update back-references for current relationships
	for _, rel := range entity.Relationships {
		if !m.EntityExists(rel.Target) {
			// Target entity doesn't exist yet, skip
			continue
		}

		targetEntity, err := m.LoadEntity(rel.Target)
		if err != nil {
			return fmt.Errorf("failed to load target entity %s: %w", rel.Target, err)
		}

		// Add back-reference
		targetEntity.AddBackReference(entity.Metadata.ID, rel.Type, rel.Note)

		// Save target entity
		filePath := m.getEntityPath(targetEntity.Metadata.ID)
		if err := SaveEntityToFile(targetEntity, filePath); err != nil {
			return fmt.Errorf("failed to save target entity %s: %w", rel.Target, err)
		}

		// Update cache
		m.mu.Lock()
		m.cache[targetEntity.Metadata.ID] = targetEntity
		m.mu.Unlock()
	}

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
		filepath.Join(m.baseDir, "graph", "relationships"),
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
	m.cache = make(map[string]*Entity)
	m.mu.Unlock()
}

