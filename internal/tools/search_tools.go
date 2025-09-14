package tools

import (
	"context"

	"silvia/internal/operations"
)

// SearchEntitiesOpsTool searches for entities using operations
type SearchEntitiesOpsTool struct {
	*BaseTool
	ops *operations.SearchOps
}

// NewSearchEntitiesOpsTool creates a new search entities tool using operations
func NewSearchEntitiesOpsTool(ops *operations.SearchOps) *SearchEntitiesOpsTool {
	return &SearchEntitiesOpsTool{
		BaseTool: NewBaseTool(
			"search_entities",
			"Search for entities matching a query",
			[]Parameter{
				{
					Name:        "query",
					Type:        "string",
					Required:    true,
					Description: "The search query",
				},
			},
		),
		ops: ops,
	}
}

// Execute searches for entities
func (t *SearchEntitiesOpsTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	query := GetString(args, "query", "")
	if query == "" {
		return ToolResult{Success: false, Error: "query is required"},
			NewToolError(t.Name(), "missing query", nil)
	}

	result, err := t.ops.SearchEntities(query)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "search failed", err)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"query":        query,
			"result_count": result.Total,
		},
	}, nil
}

// GetRelatedEntitiesOpsTool gets entities related to a specific entity using operations
type GetRelatedEntitiesOpsTool struct {
	*BaseTool
	ops *operations.SearchOps
}

// NewGetRelatedEntitiesOpsTool creates a new get related entities tool using operations
func NewGetRelatedEntitiesOpsTool(ops *operations.SearchOps) *GetRelatedEntitiesOpsTool {
	return &GetRelatedEntitiesOpsTool{
		BaseTool: NewBaseTool(
			"get_related_entities",
			"Get all entities related to a specific entity",
			[]Parameter{
				{
					Name:        "entity_id",
					Type:        "string",
					Required:    true,
					Description: "The entity ID to get relationships for",
				},
			},
		),
		ops: ops,
	}
}

// Execute gets related entities
func (t *GetRelatedEntitiesOpsTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityID := GetString(args, "entity_id", "")
	if entityID == "" {
		return ToolResult{Success: false, Error: "entity_id is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	result, err := t.ops.GetRelatedEntities(entityID)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to get related entities", err)
	}

	// Count relationships
	outgoingCount := 0
	for _, entities := range result.OutgoingByType {
		outgoingCount += len(entities)
	}
	incomingCount := 0
	for _, entities := range result.IncomingByType {
		incomingCount += len(entities)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"entity_id":      entityID,
			"outgoing_count": outgoingCount,
			"incoming_count": incomingCount,
			"broken_links":   len(result.BrokenLinks),
			"total_related":  len(result.All),
		},
	}, nil
}

// GetEntitiesByTypeTool gets all entities of a specific type
type GetEntitiesByTypeTool struct {
	*BaseTool
	ops *operations.SearchOps
}

// NewGetEntitiesByTypeTool creates a new get entities by type tool
func NewGetEntitiesByTypeTool(ops *operations.SearchOps) *GetEntitiesByTypeTool {
	return &GetEntitiesByTypeTool{
		BaseTool: NewBaseTool(
			"get_entities_by_type",
			"Get all entities of a specific type",
			[]Parameter{
				{
					Name:        "entity_type",
					Type:        "string",
					Required:    true,
					Description: "The entity type (person, organization, concept, work, event)",
				},
			},
		),
		ops: ops,
	}
}

// Execute gets entities by type
func (t *GetEntitiesByTypeTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityType := GetString(args, "entity_type", "")
	if entityType == "" {
		return ToolResult{Success: false, Error: "entity_type is required"},
			NewToolError(t.Name(), "missing entity type", nil)
	}

	entities, err := t.ops.GetEntitiesByType(entityType)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to get entities by type", err)
	}

	return ToolResult{
		Success: true,
		Data:    entities,
		Meta: map[string]any{
			"entity_type": entityType,
			"count":       len(entities),
		},
	}, nil
}

// SuggestRelatedTool suggests entities that might be related
type SuggestRelatedTool struct {
	*BaseTool
	ops *operations.SearchOps
}

// NewSuggestRelatedTool creates a new suggest related tool
func NewSuggestRelatedTool(ops *operations.SearchOps) *SuggestRelatedTool {
	return &SuggestRelatedTool{
		BaseTool: NewBaseTool(
			"suggest_related",
			"Suggest entities that might be related based on content similarity",
			[]Parameter{
				{
					Name:        "entity_id",
					Type:        "string",
					Required:    true,
					Description: "The entity ID to find suggestions for",
				},
				{
					Name:        "limit",
					Type:        "int",
					Required:    false,
					Description: "Maximum number of suggestions (default 10)",
				},
			},
		),
		ops: ops,
	}
}

// Execute suggests related entities
func (t *SuggestRelatedTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityID := GetString(args, "entity_id", "")
	if entityID == "" {
		return ToolResult{Success: false, Error: "entity_id is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	limit := GetInt(args, "limit", 10)
	if limit <= 0 {
		limit = 10
	}

	entities, err := t.ops.SuggestRelated(entityID, limit)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to suggest related entities", err)
	}

	return ToolResult{
		Success: true,
		Data:    entities,
		Meta: map[string]any{
			"entity_id":         entityID,
			"suggestions_count": len(entities),
			"limit":             limit,
		},
	}, nil
}
