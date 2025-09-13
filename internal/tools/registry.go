package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Registry manages all available tools in the system
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	logger Logger
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:  make(map[string]Tool),
		logger: &NullLogger{}, // Default to no logging
	}
}

// SetLogger sets the logger for the registry
func (r *Registry) SetLogger(logger Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool '%s' already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}

	return tool, nil
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// Execute runs a tool by name with the given arguments
func (r *Registry) Execute(ctx context.Context, toolName string, args map[string]interface{}) (ToolResult, error) {
	// Log the tool call
	r.logger.LogToolCall(toolName, args)
	startTime := time.Now()

	tool, err := r.Get(toolName)
	if err != nil {
		r.logger.LogToolError(toolName, err)
		return ToolResult{Success: false, Error: err.Error()}, err
	}

	// Validate arguments
	if err := tool.ValidateArgs(args); err != nil {
		r.logger.LogToolError(toolName, err)
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(toolName, "invalid arguments", err)
	}

	// Execute the tool
	result, err := tool.Execute(ctx, args)

	// Log the result
	duration := time.Since(startTime)
	r.logger.LogToolResult(toolName, result, duration)

	if err != nil {
		r.logger.LogToolError(toolName, err)
	}

	return result, err
}

// Search finds tools matching a query string
func (r *Registry) Search(query string) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	matches := []Tool{}

	for _, tool := range r.tools {
		// Check if query matches tool name or description
		if strings.Contains(strings.ToLower(tool.Name()), query) ||
			strings.Contains(strings.ToLower(tool.Description()), query) {
			matches = append(matches, tool)
		}
	}

	return matches
}

// GetToolHelp returns formatted help text for a tool
func (r *Registry) GetToolHelp(toolName string) (string, error) {
	tool, err := r.Get(toolName)
	if err != nil {
		return "", err
	}

	var help strings.Builder
	help.WriteString(fmt.Sprintf("Tool: %s\n", tool.Name()))
	help.WriteString(fmt.Sprintf("Description: %s\n", tool.Description()))

	if params := tool.Parameters(); len(params) > 0 {
		help.WriteString("\nParameters:\n")
		for _, param := range params {
			required := ""
			if param.Required {
				required = " (required)"
			}
			help.WriteString(fmt.Sprintf("  - %s: %s%s\n", param.Name, param.Description, required))
			help.WriteString(fmt.Sprintf("    Type: %s\n", param.Type))
			if param.Default != nil {
				help.WriteString(fmt.Sprintf("    Default: %v\n", param.Default))
			}
		}
	}

	return help.String(), nil
}

// GetAllHelp returns formatted help text for all tools
func (r *Registry) GetAllHelp() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var help strings.Builder
	help.WriteString("Available Tools:\n\n")

	for _, tool := range r.tools {
		help.WriteString(fmt.Sprintf("• %s - %s\n", tool.Name(), tool.Description()))
	}

	return help.String()
}
