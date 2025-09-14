package tools

import (
	"context"
	"fmt"
	"time"

	"silvia/internal/operations"
)

// Manager manages tools using the operations layer
type Manager struct {
	registry *Registry
	ops      *operations.Operations
	logger   Logger
}

// ToolCall represents a single tool invocation in a chain
type ToolCall struct {
	Tool string         // Tool name
	Args map[string]any // Arguments for the tool
}

// NewManager creates a new tool manager with operations-based tools
func NewManager(ops *operations.Operations) *Manager {
	// Create manager with default logger
	m := &Manager{
		registry: NewRegistry(),
		ops:      ops,
		logger:   NewDefaultLogger(false), // Default to non-verbose logging
	}

	// Set the logger on the registry
	m.registry.SetLogger(m.logger)

	// Register all standard tools
	m.registerAllTools()

	return m
}

// SetLogger sets the logger for the manager and its registry
func (m *Manager) SetLogger(logger Logger) {
	m.logger = logger
	m.registry.SetLogger(logger)
}

// EnableVerboseLogging enables verbose logging to stdout
func (m *Manager) EnableVerboseLogging() {
	logger := NewDefaultLogger(true)
	m.SetLogger(logger)
}

// EnableFileLogging enables logging to a file
func (m *Manager) EnableFileLogging(filename string, verbose bool) error {
	logger, err := NewFileLogger(filename, verbose)
	if err != nil {
		return err
	}
	m.SetLogger(logger)
	return nil
}

// registerAllTools registers all tools from the operations layer
func (m *Manager) registerAllTools() {
	// Entity tools
	m.registry.Register(NewMergeEntitiesTool(m.ops.Entity))
	m.registry.Register(NewRenameEntityTool(m.ops.Entity))
	m.registry.Register(NewRefineEntityTool(m.ops.Entity))
	m.registry.Register(NewDeleteEntityTool(m.ops.Entity))
	m.registry.Register(NewCreateEntityOpsTool(m.ops.Entity))

	// Queue tools
	m.registry.Register(NewGetQueueTool(m.ops.Queue))
	m.registry.Register(NewAddToQueueTool(m.ops.Queue))
	m.registry.Register(NewRemoveFromQueueTool(m.ops.Queue))
	m.registry.Register(NewProcessNextQueueItemTool(m.ops.Queue))
	m.registry.Register(NewUpdateQueuePriorityTool(m.ops.Queue))
	m.registry.Register(NewClearQueueTool(m.ops.Queue))

	// Source tools
	m.registry.Register(NewIngestSourceTool(m.ops.Source))
	m.registry.Register(NewExtractFromHTMLTool(m.ops.Source))

	// Search tools
	m.registry.Register(NewSearchEntitiesOpsTool(m.ops.Search))
	m.registry.Register(NewGetRelatedEntitiesOpsTool(m.ops.Search))
	m.registry.Register(NewGetEntitiesByTypeTool(m.ops.Search))
	m.registry.Register(NewSuggestRelatedTool(m.ops.Search))

	// All tools now use the operations layer - no more GraphOperations
}

// Registry returns the tool registry
func (m *Manager) Registry() *Registry {
	return m.registry
}

// Execute runs a tool by name with the given arguments
func (m *Manager) Execute(ctx context.Context, toolName string, args map[string]any) (ToolResult, error) {
	return m.registry.Execute(ctx, toolName, args)
}

// ExecuteChain runs multiple tools in sequence, passing results between them
func (m *Manager) ExecuteChain(ctx context.Context, chain []ToolCall) ([]ToolResult, error) {
	m.logger.LogChainStart(len(chain))
	startTime := time.Now()

	results := make([]ToolResult, 0, len(chain))
	success := true

	for i, call := range chain {
		// Allow referencing previous results in arguments
		processedArgs := m.processArguments(call.Args, results)

		result, err := m.Execute(ctx, call.Tool, processedArgs)
		if err != nil {
			success = false
			m.logger.LogChainComplete(time.Since(startTime), false)
			return results, fmt.Errorf("failed at step %d (%s): %w", i+1, call.Tool, err)
		}

		results = append(results, result)

		// Stop chain if a tool fails
		if !result.Success {
			success = false
			m.logger.LogChainComplete(time.Since(startTime), false)
			return results, fmt.Errorf("tool %s failed: %s", call.Tool, result.Error)
		}
	}

	m.logger.LogChainComplete(time.Since(startTime), success)
	return results, nil
}

// processArguments processes arguments, replacing references to previous results
func (m *Manager) processArguments(args map[string]any, previousResults []ToolResult) map[string]any {
	processed := make(map[string]any)

	for key, value := range args {
		// Check if value is a reference to a previous result
		if strVal, ok := value.(string); ok {
			// Simple reference format: "$1.data.id" references the first result's data.id field
			// This is a simplified implementation - could be made more sophisticated
			if len(strVal) > 0 && strVal[0] == '$' {
				// Parse and resolve reference (simplified for now)
				processed[key] = value // Keep original for now
			} else {
				processed[key] = value
			}
		} else {
			processed[key] = value
		}
	}

	return processed
}

// GetToolHelp returns help text for a specific tool
func (m *Manager) GetToolHelp(toolName string) (string, error) {
	return m.registry.GetToolHelp(toolName)
}

// GetAllTools returns a list of all available tools
func (m *Manager) GetAllTools() []Tool {
	return m.registry.List()
}

// FindTools searches for tools matching a query
func (m *Manager) FindTools(query string) []Tool {
	return m.registry.Search(query)
}

// GetToolSchemas returns JSON schemas for all tools (for LLM function calling)
func (m *Manager) GetToolSchemas() []map[string]interface{} {
	tools := m.registry.List()
	schemas := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		schema := m.toolToSchema(tool)
		schemas = append(schemas, schema)
	}

	return schemas
}

// toolToSchema converts a tool to a JSON schema for LLM function calling
func (m *Manager) toolToSchema(tool Tool) map[string]interface{} {
	// Build parameter schema
	properties := make(map[string]interface{})
	required := []string{}

	for _, param := range tool.Parameters() {
		paramSchema := map[string]interface{}{
			"type":        convertTypeToJSONSchema(param.Type),
			"description": param.Description,
		}

		if param.Default != nil {
			paramSchema["default"] = param.Default
		}

		properties[param.Name] = paramSchema

		if param.Required {
			required = append(required, param.Name)
		}
	}

	return map[string]interface{}{
		"name":        tool.Name(),
		"description": tool.Description(),
		"parameters": map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	}
}

// convertTypeToJSONSchema converts Go types to JSON schema types
func convertTypeToJSONSchema(goType string) string {
	switch goType {
	case "string":
		return "string"
	case "int", "int32", "int64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "[]string":
		return "array"
	case "map":
		return "object"
	default:
		return "string" // Default to string for unknown types
	}
}
