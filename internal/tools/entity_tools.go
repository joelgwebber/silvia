package tools

import (
	"context"

	"silvia/internal/operations"
)

// MergeEntitiesTool merges two entities
type MergeEntitiesTool struct {
	*BaseTool
	ops *operations.EntityOps
}

// NewMergeEntitiesTool creates a new merge entities tool
func NewMergeEntitiesTool(ops *operations.EntityOps) *MergeEntitiesTool {
	return &MergeEntitiesTool{
		BaseTool: NewBaseTool(
			"merge_entities",
			"Merge entity2 into entity1, updating all references",
			[]Parameter{
				{
					Name:        "entity1_id",
					Type:        "string",
					Required:    true,
					Description: "The ID of the entity to merge into (will be kept)",
				},
				{
					Name:        "entity2_id",
					Type:        "string",
					Required:    true,
					Description: "The ID of the entity to merge from (will be deleted)",
				},
			},
		),
		ops: ops,
	}
}

// Execute merges two entities
func (t *MergeEntitiesTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entity1ID := GetString(args, "entity1_id", "")
	entity2ID := GetString(args, "entity2_id", "")

	if entity1ID == "" || entity2ID == "" {
		return ToolResult{Success: false, Error: "both entity IDs are required"},
			NewToolError(t.Name(), "missing entity IDs", nil)
	}

	result, err := t.ops.MergeEntities(ctx, entity1ID, entity2ID)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to merge entities", err)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"merged_into":   entity1ID,
			"deleted":       entity2ID,
			"updated_files": len(result.UpdatedFiles),
		},
	}, nil
}

// RenameEntityTool renames an entity and updates all references
type RenameEntityTool struct {
	*BaseTool
	ops *operations.EntityOps
}

// NewRenameEntityTool creates a new rename entity tool
func NewRenameEntityTool(ops *operations.EntityOps) *RenameEntityTool {
	return &RenameEntityTool{
		BaseTool: NewBaseTool(
			"rename_entity",
			"Rename an entity and update all references",
			[]Parameter{
				{
					Name:        "old_id",
					Type:        "string",
					Required:    true,
					Description: "The current ID of the entity",
				},
				{
					Name:        "new_id",
					Type:        "string",
					Required:    true,
					Description: "The new ID for the entity",
				},
			},
		),
		ops: ops,
	}
}

// Execute renames an entity
func (t *RenameEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	oldID := GetString(args, "old_id", "")
	newID := GetString(args, "new_id", "")

	if oldID == "" || newID == "" {
		return ToolResult{Success: false, Error: "both old and new IDs are required"},
			NewToolError(t.Name(), "missing IDs", nil)
	}

	result, err := t.ops.RenameEntity(oldID, newID)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to rename entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"old_id":        oldID,
			"new_id":        newID,
			"updated_files": len(result.UpdatedFiles),
		},
	}, nil
}

// RefineEntityTool uses LLM to refine an entity's content
type RefineEntityTool struct {
	*BaseTool
	ops *operations.EntityOps
}

// NewRefineEntityTool creates a new refine entity tool
func NewRefineEntityTool(ops *operations.EntityOps) *RefineEntityTool {
	return &RefineEntityTool{
		BaseTool: NewBaseTool(
			"refine_entity",
			"Use LLM to refine an entity's content based on its sources",
			[]Parameter{
				{
					Name:        "entity_id",
					Type:        "string",
					Required:    true,
					Description: "The ID of the entity to refine",
				},
				{
					Name:        "guidance",
					Type:        "string",
					Required:    false,
					Description: "Optional guidance for the refinement",
				},
			},
		),
		ops: ops,
	}
}

// Execute refines an entity
func (t *RefineEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityID := GetString(args, "entity_id", "")
	if entityID == "" {
		return ToolResult{Success: false, Error: "entity ID is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	guidance := GetString(args, "guidance", "")

	entity, err := t.ops.RefineEntity(ctx, entityID, guidance)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to refine entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    entity,
		Meta: map[string]any{
			"id":          entity.Metadata.ID,
			"refined":     true,
			"num_sources": len(entity.Metadata.Sources),
		},
	}, nil
}

// DeleteEntityTool deletes an entity
type DeleteEntityTool struct {
	*BaseTool
	ops *operations.EntityOps
}

// NewDeleteEntityTool creates a new delete entity tool
func NewDeleteEntityTool(ops *operations.EntityOps) *DeleteEntityTool {
	return &DeleteEntityTool{
		BaseTool: NewBaseTool(
			"delete_entity",
			"Delete an entity (only if no other entities reference it)",
			[]Parameter{
				{
					Name:        "entity_id",
					Type:        "string",
					Required:    true,
					Description: "The ID of the entity to delete",
				},
			},
		),
		ops: ops,
	}
}

// Execute deletes an entity
func (t *DeleteEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityID := GetString(args, "entity_id", "")
	if entityID == "" {
		return ToolResult{Success: false, Error: "entity ID is required"},
			NewToolError(t.Name(), "missing entity ID", nil)
	}

	err := t.ops.DeleteEntity(entityID)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to delete entity", err)
	}

	return ToolResult{
		Success: true,
		Data:    map[string]any{"deleted": entityID},
		Meta:    map[string]any{"id": entityID},
	}, nil
}

// CreateEntityOpsTool creates a new entity using operations
type CreateEntityOpsTool struct {
	*BaseTool
	ops *operations.EntityOps
}

// NewCreateEntityOpsTool creates a new create entity tool using operations
func NewCreateEntityOpsTool(ops *operations.EntityOps) *CreateEntityOpsTool {
	return &CreateEntityOpsTool{
		BaseTool: NewBaseTool(
			"create_entity_ops",
			"Create a new entity using operations layer",
			[]Parameter{
				{
					Name:        "type",
					Type:        "string",
					Required:    true,
					Description: "Entity type (person, organization, concept, work, event)",
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
					Description: "Entity title",
				},
				{
					Name:        "content",
					Type:        "string",
					Required:    false,
					Description: "Entity content/description",
				},
			},
		),
		ops: ops,
	}
}

// Execute creates a new entity
func (t *CreateEntityOpsTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	entityType := GetString(args, "type", "")
	id := GetString(args, "id", "")
	title := GetString(args, "title", "")
	content := GetString(args, "content", "")

	if entityType == "" || id == "" || title == "" {
		return ToolResult{Success: false, Error: "type, id, and title are required"},
			NewToolError(t.Name(), "missing required fields", nil)
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
			"type":    entity.Metadata.Type,
			"created": true,
		},
	}, nil
}
