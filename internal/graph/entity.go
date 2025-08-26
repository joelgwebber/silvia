package graph

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// NewEntity creates a new entity with the given ID and type
func NewEntity(id string, entityType EntityType) *Entity {
	now := time.Now()
	return &Entity{
		Metadata: Metadata{
			ID:      id,
			Type:    entityType,
			Created: now,
			Updated: now,
		},
		Title:         extractTitle(id),
		Relationships: []Relationship{},
		BackRefs:      []BackReference{},
	}
}

// extractTitle derives a human-readable title from an entity ID
func extractTitle(id string) string {
	// Get the last segment of the path
	base := filepath.Base(id)
	// Remove file extension if present
	base = strings.TrimSuffix(base, ".md")
	// Replace hyphens with spaces and title case
	parts := strings.Split(base, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// AddRelationship adds a new relationship to the entity
func (e *Entity) AddRelationship(relType string, target string, date *time.Time, note string) {
	rel := Relationship{
		Type:   relType,
		Target: target,
		Date:   date,
		Note:   note,
	}
	e.Relationships = append(e.Relationships, rel)
	e.Metadata.Updated = time.Now()
}

// AddBackReference adds a back reference from another entity
func (e *Entity) AddBackReference(source string, relType string, note string) {
	// Check if this back reference already exists
	for i, ref := range e.BackRefs {
		if ref.Source == source {
			// Update existing reference if type changed
			if ref.Type != relType {
				e.BackRefs[i].Type = relType
				e.BackRefs[i].Note = note
				e.Metadata.Updated = time.Now()
			}
			return
		}
	}

	// Add new back reference
	backRef := BackReference{
		Source: source,
		Type:   relType,
		Note:   note,
	}
	e.BackRefs = append(e.BackRefs, backRef)
	e.Metadata.Updated = time.Now()
}

// AddSource adds a source reference to the entity
func (e *Entity) AddSource(source string) {
	// Check if source already exists
	if slices.Contains(e.Metadata.Sources, source) {
		return
	}
	e.Metadata.Sources = append(e.Metadata.Sources, source)
	e.Metadata.Updated = time.Now()
}

// AddAlias adds an alternative name for the entity
func (e *Entity) AddAlias(alias string) {
	// Check if alias already exists
	for _, a := range e.Metadata.Aliases {
		if strings.EqualFold(a, alias) {
			return
		}
	}
	e.Metadata.Aliases = append(e.Metadata.Aliases, alias)
	e.Metadata.Updated = time.Now()
}

// GetFilePath returns the file path for storing this entity
func (e *Entity) GetFilePath(baseDir string) string {
	return filepath.Join(baseDir, "graph", e.Metadata.ID+".md")
}

// Validate checks if the entity has required fields
func (e *Entity) Validate() error {
	if e.Metadata.ID == "" {
		return fmt.Errorf("entity ID is required")
	}
	if e.Metadata.Type == "" {
		return fmt.Errorf("entity type is required")
	}
	if e.Title == "" {
		return fmt.Errorf("entity title is required")
	}
	return nil
}

// OutgoingLink represents any link from this entity to another
type OutgoingLink struct {
	Target string // Entity ID or URL
	Type   string // Link type: "wiki-link", "source", or relationship type
	Note   string // Optional note or description
}

// GetAllOutgoingLinks extracts all outgoing references from the entity
func (e *Entity) GetAllOutgoingLinks() []OutgoingLink {
	links := []OutgoingLink{}
	seen := make(map[string]bool)
	
	// Extract wiki-links from content
	wikiLinks := ExtractWikiLinks(e.Content)
	for _, target := range wikiLinks {
		key := "wiki:" + target
		if !seen[key] {
			links = append(links, OutgoingLink{
				Target: target,
				Type:   "mentioned_in",
			})
			seen[key] = true
		}
	}
	
	// Extract entity references from sources (not URLs)
	for _, source := range e.Metadata.Sources {
		if strings.Contains(source, "/") && !strings.Contains(source, "://") {
			key := "source:" + source
			if !seen[key] {
				links = append(links, OutgoingLink{
					Target: source,
					Type:   "sourced_from",
				})
				seen[key] = true
			}
		}
	}
	
	return links
}

