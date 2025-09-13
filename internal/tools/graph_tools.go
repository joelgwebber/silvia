package tools

import (
	"context"
	"fmt"

	"silvia/internal/graph"
)

// ReadEntityTool reads an entity from the graph
type ReadEntityTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewReadEntityTool creates a new read entity tool
func NewReadEntityTool(ops *graph.GraphOperations) *ReadEntityTool {
	return &ReadEntityTool{
		BaseTool: NewBaseTool(
			"read_entity",
			"Read an entity from the knowledge graph",
			[]Parameter{
				{
					Name:        "id",
					Type:        "string",
					Required:    true,
					Description: "The entity ID to read (e.g., 'people/john-doe')",
				},
			},
		),
		ops: ops,
	}
}

// Execute reads an entity
func (t *ReadEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	id := GetString(args, "id", "")
	if id == "" {
		return ToolResult{Success: false, Error: "entity ID is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	entity, err := t.ops.ReadEntity(id)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to read entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    entity,
		Meta: map[string]any{
			"id":           entity.Metadata.ID,
			"type":         entity.Metadata.Type,
			"title":        entity.Title,
			"num_sources":  len(entity.Metadata.Sources),
			"num_links":    len(entity.Relationships),
			"num_backrefs": len(entity.BackRefs),
		},
	}, nil
}

// UpdateEntityTool updates an existing entity
type UpdateEntityTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewUpdateEntityTool creates a new update entity tool
func NewUpdateEntityTool(ops *graph.GraphOperations) *UpdateEntityTool {
	return &UpdateEntityTool{
		BaseTool: NewBaseTool(
			"update_entity",
			"Update an existing entity's content or metadata",
			[]Parameter{
				{
					Name:        "id",
					Type:        "string",
					Required:    true,
					Description: "The entity ID to update",
				},
				{
					Name:        "content",
					Type:        "string",
					Required:    false,
					Description: "New content for the entity",
				},
				{
					Name:        "aliases",
					Type:        "[]string",
					Required:    false,
					Description: "New aliases for the entity",
				},
				{
					Name:        "tags",
					Type:        "[]string",
					Required:    false,
					Description: "New tags for the entity",
				},
				{
					Name:        "sources",
					Type:        "[]string",
					Required:    false,
					Description: "New sources for the entity",
				},
			},
		),
		ops: ops,
	}
}

// Execute updates an entity
func (t *UpdateEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	id := GetString(args, "id", "")
	if id == "" {
		return ToolResult{Success: false, Error: "entity ID is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	content := GetString(args, "content", "")
	aliases := GetStringSlice(args, "aliases", nil)
	tags := GetStringSlice(args, "tags", nil)
	sources := GetStringSlice(args, "sources", nil)

	// Build metadata update if any metadata fields provided
	var metadata *graph.Metadata
	if aliases != nil || tags != nil || sources != nil {
		metadata = &graph.Metadata{}
		if aliases != nil {
			metadata.Aliases = aliases
		}
		if tags != nil {
			metadata.Tags = tags
		}
		if sources != nil {
			metadata.Sources = sources
		}
	}

	entity, err := t.ops.UpdateEntity(id, content, metadata)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to update entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    entity,
		Meta: map[string]any{
			"id":      entity.Metadata.ID,
			"updated": true,
		},
	}, nil
}

// CreateEntityTool creates a new entity
type CreateEntityTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewCreateEntityTool creates a new create entity tool
func NewCreateEntityTool(ops *graph.GraphOperations) *CreateEntityTool {
	return &CreateEntityTool{
		BaseTool: NewBaseTool(
			"create_entity",
			"Create a new entity in the knowledge graph",
			[]Parameter{
				{
					Name:        "type",
					Type:        "string",
					Required:    true,
					Description: "Entity type (person, organization, concept, event, work)",
				},
				{
					Name:        "id",
					Type:        "string",
					Required:    true,
					Description: "Entity ID (e.g., 'people/john-doe')",
				},
				{
					Name:        "title",
					Type:        "string",
					Required:    true,
					Description: "Entity title/name",
				},
				{
					Name:        "content",
					Type:        "string",
					Required:    false,
					Description: "Entity content/description",
					Default:     "",
				},
			},
		),
		ops: ops,
	}
}

// Execute creates a new entity
func (t *CreateEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	typeStr := GetString(args, "type", "")
	id := GetString(args, "id", "")
	title := GetString(args, "title", "")
	content := GetString(args, "content", "")

	if typeStr == "" || id == "" || title == "" {
		return ToolResult{Success: false, Error: "type, id, and title are required"},
			NewToolError(t.Name(), "missing required parameters", nil)
	}

	// Convert type string to EntityType
	entityType := graph.EntityType(typeStr)
	if !entityType.IsValid() {
		return ToolResult{Success: false, Error: fmt.Sprintf("invalid entity type: %s", typeStr)},
			NewToolError(t.Name(), "invalid entity type", nil)
	}

	entity, err := t.ops.CreateEntity(entityType, id, title, content)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to create entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    entity,
		Meta: map[string]any{
			"id":      entity.Metadata.ID,
			"created": true,
		},
	}, nil
}

// SearchEntitiesTool searches for entities
type SearchEntitiesTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewSearchEntitiesTool creates a new search entities tool
func NewSearchEntitiesTool(ops *graph.GraphOperations) *SearchEntitiesTool {
	return &SearchEntitiesTool{
		BaseTool: NewBaseTool(
			"search_entities",
			"Search for entities in the knowledge graph",
			[]Parameter{
				{
					Name:        "query",
					Type:        "string",
					Required:    true,
					Description: "Search query",
				},
			},
		),
		ops: ops,
	}
}

// Execute searches for entities
func (t *SearchEntitiesTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	query := GetString(args, "query", "")
	if query == "" {
		return ToolResult{Success: false, Error: "search query is required"},
			NewToolError(t.Name(), "missing search query", nil)
	}

	entities, err := t.ops.SearchEntities(query)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "search failed", err)
	}

	// Convert entities to summary format for result
	results := make([]map[string]any, len(entities))
	for i, entity := range entities {
		results[i] = map[string]any{
			"id":      entity.Metadata.ID,
			"type":    entity.Metadata.Type,
			"title":   entity.Title,
			"excerpt": truncateString(entity.Content, 200),
		}
	}

	return ToolResult{
		Success: true,
		Data:    results,
		Meta: map[string]any{
			"count": len(entities),
			"query": query,
		},
	}, nil
}

// CreateLinkTool creates relationships between entities
type CreateLinkTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewCreateLinkTool creates a new create link tool
func NewCreateLinkTool(ops *graph.GraphOperations) *CreateLinkTool {
	return &CreateLinkTool{
		BaseTool: NewBaseTool(
			"create_link",
			"Create a relationship between two entities",
			[]Parameter{
				{
					Name:        "source",
					Type:        "string",
					Required:    true,
					Description: "Source entity ID",
				},
				{
					Name:        "type",
					Type:        "string",
					Required:    true,
					Description: "Relationship type",
				},
				{
					Name:        "target",
					Type:        "string",
					Required:    true,
					Description: "Target entity ID",
				},
				{
					Name:        "note",
					Type:        "string",
					Required:    false,
					Description: "Optional note about the relationship",
					Default:     "",
				},
			},
		),
		ops: ops,
	}
}

// Execute creates a link between entities
func (t *CreateLinkTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	source := GetString(args, "source", "")
	relType := GetString(args, "type", "")
	target := GetString(args, "target", "")
	note := GetString(args, "note", "")

	if source == "" || relType == "" || target == "" {
		return ToolResult{Success: false, Error: "source, type, and target are required"},
			NewToolError(t.Name(), "missing required parameters", nil)
	}

	err := t.ops.CreateLink(source, relType, target, note)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to create link", err)
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"source": source,
			"type":   relType,
			"target": target,
			"note":   note,
		},
		Meta: map[string]any{
			"created": true,
		},
	}, nil
}

// GetRelatedEntitiesTool gets entities related to a given entity
type GetRelatedEntitiesTool struct {
	*BaseTool
	ops *graph.GraphOperations
}

// NewGetRelatedEntitiesTool creates a new get related entities tool
func NewGetRelatedEntitiesTool(ops *graph.GraphOperations) *GetRelatedEntitiesTool {
	return &GetRelatedEntitiesTool{
		BaseTool: NewBaseTool(
			"get_related",
			"Get entities related to a specific entity",
			[]Parameter{
				{
					Name:        "id",
					Type:        "string",
					Required:    true,
					Description: "The entity ID to get related entities for",
				},
			},
		),
		ops: ops,
	}
}

// Execute gets related entities
func (t *GetRelatedEntitiesTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	id := GetString(args, "id", "")
	if id == "" {
		return ToolResult{Success: false, Error: "entity ID is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	result, err := t.ops.GetRelatedEntities(id)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to get related entities", err)
	}

	// Format the result for output
	formattedResult := map[string]any{
		"entity":       result.Entity,
		"outgoing":     formatEntitiesByType(result.OutgoingByType),
		"incoming":     formatEntitiesByType(result.IncomingByType),
		"broken_links": result.BrokenLinks,
		"all":          formatEntities(result.All),
	}

	return ToolResult{
		Success: true,
		Data:    formattedResult,
		Meta: map[string]any{
			"entity_id":      id,
			"outgoing_count": countEntitiesInMap(result.OutgoingByType),
			"incoming_count": countEntitiesInMap(result.IncomingByType),
			"total_count":    len(result.All),
			"broken_count":   len(result.BrokenLinks),
		},
	}, nil
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatEntitiesByType(entitiesByType map[string][]*graph.Entity) map[string][]map[string]any {
	result := make(map[string][]map[string]any)
	for relType, entities := range entitiesByType {
		relEntities := make([]map[string]any, len(entities))
		for i, entity := range entities {
			relEntities[i] = map[string]any{
				"id":    entity.Metadata.ID,
				"type":  entity.Metadata.Type,
				"title": entity.Title,
			}
		}
		result[relType] = relEntities
	}
	return result
}

func countEntitiesInMap(entitiesByType map[string][]*graph.Entity) int {
	count := 0
	for _, entities := range entitiesByType {
		count += len(entities)
	}
	return count
}

func formatEntities(entities []*graph.Entity) []map[string]any {
	result := make([]map[string]any, len(entities))
	for i, entity := range entities {
		result[i] = map[string]any{
			"id":    entity.Metadata.ID,
			"type":  entity.Metadata.Type,
			"title": entity.Title,
		}
	}
	return result
}
