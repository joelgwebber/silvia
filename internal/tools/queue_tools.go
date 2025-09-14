package tools

import (
	"context"

	"silvia/internal/operations"
)

// GetQueueTool gets the current queue status
type GetQueueTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewGetQueueTool creates a new get queue tool
func NewGetQueueTool(ops *operations.QueueOps) *GetQueueTool {
	return &GetQueueTool{
		BaseTool: NewBaseTool(
			"get_queue",
			"Get the current status of the source queue",
			[]Parameter{},
		),
		ops: ops,
	}
}

// Execute gets the queue status
func (t *GetQueueTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	status, err := t.ops.GetQueue()
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to get queue", err)
	}

	return ToolResult{
		Success: true,
		Data:    status,
		Meta: map[string]any{
			"total_count":     status.TotalCount,
			"high_priority":   status.ByPriority[2],
			"medium_priority": status.ByPriority[1],
			"low_priority":    status.ByPriority[0],
		},
	}, nil
}

// AddToQueueTool adds a source to the queue
type AddToQueueTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewAddToQueueTool creates a new add to queue tool
func NewAddToQueueTool(ops *operations.QueueOps) *AddToQueueTool {
	return &AddToQueueTool{
		BaseTool: NewBaseTool(
			"add_to_queue",
			"Add a source URL to the processing queue",
			[]Parameter{
				{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "The URL to add to the queue",
				},
				{
					Name:        "priority",
					Type:        "int",
					Required:    false,
					Description: "Priority level (0=low, 1=medium, 2=high)",
				},
				{
					Name:        "from_source",
					Type:        "string",
					Required:    false,
					Description: "The source this URL came from",
				},
				{
					Name:        "description",
					Type:        "string",
					Required:    false,
					Description: "Description of why this URL is being added",
				},
			},
		),
		ops: ops,
	}
}

// Execute adds to the queue
func (t *AddToQueueTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return ToolResult{Success: false, Error: "URL is required"},
			NewToolError(t.Name(), "missing URL", nil)
	}

	priority := GetInt(args, "priority", 1) // Default to medium priority
	fromSource := GetString(args, "from_source", "")
	description := GetString(args, "description", "")

	err := t.ops.AddToQueue(url, priority, fromSource, description)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to add to queue", err)
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"url":         url,
			"priority":    priority,
			"from_source": fromSource,
			"description": description,
		},
		Meta: map[string]any{
			"added": true,
		},
	}, nil
}

// RemoveFromQueueTool removes a source from the queue
type RemoveFromQueueTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewRemoveFromQueueTool creates a new remove from queue tool
func NewRemoveFromQueueTool(ops *operations.QueueOps) *RemoveFromQueueTool {
	return &RemoveFromQueueTool{
		BaseTool: NewBaseTool(
			"remove_from_queue",
			"Remove a source URL from the processing queue",
			[]Parameter{
				{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "The URL to remove from the queue",
				},
			},
		),
		ops: ops,
	}
}

// Execute removes from the queue
func (t *RemoveFromQueueTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return ToolResult{Success: false, Error: "URL is required"},
			NewToolError(t.Name(), "missing URL", nil)
	}

	err := t.ops.RemoveFromQueue(url)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to remove from queue", err)
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"url":     url,
			"removed": true,
		},
		Meta: map[string]any{
			"removed": true,
		},
	}, nil
}

// ProcessNextQueueItemTool processes the next item in the queue
type ProcessNextQueueItemTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewProcessNextQueueItemTool creates a new process next queue item tool
func NewProcessNextQueueItemTool(ops *operations.QueueOps) *ProcessNextQueueItemTool {
	return &ProcessNextQueueItemTool{
		BaseTool: NewBaseTool(
			"process_next_queue_item",
			"Get and remove the highest priority item from the queue",
			[]Parameter{},
		),
		ops: ops,
	}
}

// Execute processes the next queue item
func (t *ProcessNextQueueItemTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	item, err := t.ops.ProcessNextItem()
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to process next item", err)
	}

	if item == nil {
		return ToolResult{
			Success: true,
			Data:    nil,
			Meta: map[string]any{
				"queue_empty": true,
			},
		}, nil
	}

	return ToolResult{
		Success: true,
		Data:    item,
		Meta: map[string]any{
			"url":      item.URL,
			"priority": item.Priority,
		},
	}, nil
}

// UpdateQueuePriorityTool updates the priority of an item in the queue
type UpdateQueuePriorityTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewUpdateQueuePriorityTool creates a new update queue priority tool
func NewUpdateQueuePriorityTool(ops *operations.QueueOps) *UpdateQueuePriorityTool {
	return &UpdateQueuePriorityTool{
		BaseTool: NewBaseTool(
			"update_queue_priority",
			"Update the priority of an item in the queue",
			[]Parameter{
				{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "The URL to update",
				},
				{
					Name:        "priority",
					Type:        "int",
					Required:    true,
					Description: "New priority level (0=low, 1=medium, 2=high)",
				},
			},
		),
		ops: ops,
	}
}

// Execute updates queue priority
func (t *UpdateQueuePriorityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return ToolResult{Success: false, Error: "URL is required"},
			NewToolError(t.Name(), "missing URL", nil)
	}

	priority := GetInt(args, "priority", -1)
	if priority < 0 || priority > 2 {
		return ToolResult{Success: false, Error: "priority must be 0, 1, or 2"},
			NewToolError(t.Name(), "invalid priority", nil)
	}

	err := t.ops.UpdatePriority(url, priority)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to update priority", err)
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"url":          url,
			"new_priority": priority,
		},
		Meta: map[string]any{
			"updated": true,
		},
	}, nil
}

// ClearQueueTool clears all items from the queue
type ClearQueueTool struct {
	*BaseTool
	ops *operations.QueueOps
}

// NewClearQueueTool creates a new clear queue tool
func NewClearQueueTool(ops *operations.QueueOps) *ClearQueueTool {
	return &ClearQueueTool{
		BaseTool: NewBaseTool(
			"clear_queue",
			"Remove all items from the queue",
			[]Parameter{},
		),
		ops: ops,
	}
}

// Execute clears the queue
func (t *ClearQueueTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	err := t.ops.ClearQueue()
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to clear queue", err)
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"cleared": true,
		},
		Meta: map[string]any{
			"cleared": true,
		},
	}, nil
}
