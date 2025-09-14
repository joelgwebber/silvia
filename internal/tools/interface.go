package tools

import (
	"context"
	"fmt"
)

// Tool represents a single executable operation in the system
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of what this tool does
	Description() string

	// Parameters returns the parameter definitions for this tool
	Parameters() []Parameter

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args map[string]any) (ToolResult, error)

	// ValidateArgs checks if the provided arguments are valid
	ValidateArgs(args map[string]any) error
}

// Parameter describes a single parameter for a tool
type Parameter struct {
	Name        string // Parameter name
	Type        string // Type (string, int, bool, etc.)
	Required    bool   // Whether this parameter is required
	Description string // Human-readable description
	Default     any    // Default value if not provided
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	Success bool           // Whether the operation succeeded
	Data    any            // The actual result data
	Error   string         // Error message if failed
	Meta    map[string]any // Additional metadata
}

// ToolError represents an error that occurred during tool execution
type ToolError struct {
	Tool    string // Name of the tool that failed
	Message string // Error message
	Cause   error  // Underlying error if any
}

func (e *ToolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s tool error: %s (caused by: %v)", e.Tool, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s tool error: %s", e.Tool, e.Message)
}

// NewToolError creates a new tool error
func NewToolError(tool, message string, cause error) error {
	return &ToolError{
		Tool:    tool,
		Message: message,
		Cause:   cause,
	}
}

// BaseTool provides common functionality for all tools
type BaseTool struct {
	name        string
	description string
	parameters  []Parameter
}

// NewBaseTool creates a new base tool
func NewBaseTool(name, description string, parameters []Parameter) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
	}
}

// Name returns the tool name
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns the tool description
func (t *BaseTool) Description() string {
	return t.description
}

// Parameters returns the tool parameters
func (t *BaseTool) Parameters() []Parameter {
	return t.parameters
}

// ValidateArgs provides basic argument validation
func (t *BaseTool) ValidateArgs(args map[string]any) error {
	// Check required parameters
	for _, param := range t.parameters {
		if param.Required {
			if _, ok := args[param.Name]; !ok {
				return fmt.Errorf("required parameter '%s' not provided", param.Name)
			}
		}
	}

	// Check for unknown parameters
	validParams := make(map[string]bool)
	for _, param := range t.parameters {
		validParams[param.Name] = true
	}

	for key := range args {
		if !validParams[key] {
			return fmt.Errorf("unknown parameter '%s'", key)
		}
	}

	return nil
}

// GetString extracts a string parameter from args
func GetString(args map[string]any, key string, defaultValue string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetBool extracts a boolean parameter from args
func GetBool(args map[string]any, key string, defaultValue bool) bool {
	if val, ok := args[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// GetInt extracts an integer parameter from args
func GetInt(args map[string]any, key string, defaultValue int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case float32:
			return int(v)
		}
	}
	return defaultValue
}

// GetStringSlice extracts a string slice parameter from args
func GetStringSlice(args map[string]any, key string, defaultValue []string) []string {
	if val, ok := args[key]; ok {
		if slice, ok := val.([]string); ok {
			return slice
		}
		// Try to convert []any to []string
		if iSlice, ok := val.([]any); ok {
			result := make([]string, 0, len(iSlice))
			for _, item := range iSlice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}

// GetMap extracts a map parameter from args
func GetMap(args map[string]any, key string, defaultValue map[string]any) map[string]any {
	if val, ok := args[key]; ok {
		if m, ok := val.(map[string]any); ok {
			return m
		}
	}
	return defaultValue
}
