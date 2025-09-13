package tools

import (
	"context"
	"fmt"
	"time"

	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/sources"
)

// Manager manages tools and their dependencies
type Manager struct {
	registry *Registry
	graphOps *graph.GraphOperations
	llm      *llm.Client
	sources  *sources.Manager
	logger   Logger
}

// NewManager creates a new tool manager with all standard tools registered
func NewManager(graphManager *graph.Manager, llmClient *llm.Client, sourcesManager *sources.Manager) *Manager {
	// Create operations layer
	graphOps := graph.NewGraphOperations(graphManager, llmClient)

	// Create manager with default logger
	m := &Manager{
		registry: NewRegistry(),
		graphOps: graphOps,
		llm:      llmClient,
		sources:  sourcesManager,
		logger:   NewDefaultLogger(false), // Default to non-verbose logging
	}

	// Set the logger on the registry
	m.registry.SetLogger(m.logger)

	// Register all standard tools
	m.registerStandardTools()

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

// registerStandardTools registers all built-in tools
func (m *Manager) registerStandardTools() {
	// Graph tools
	m.registry.Register(NewReadEntityTool(m.graphOps))
	m.registry.Register(NewUpdateEntityTool(m.graphOps))
	m.registry.Register(NewCreateEntityTool(m.graphOps))
	m.registry.Register(NewSearchEntitiesTool(m.graphOps))
	m.registry.Register(NewCreateLinkTool(m.graphOps))
	m.registry.Register(NewGetRelatedEntitiesTool(m.graphOps))

	// Additional tools can be registered here as they're created:
	// - MergeTool
	// - RenameTool
	// - RefineTool
	// - IngestTool
	// - QueueTool
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

// ToolCall represents a single tool invocation in a chain
type ToolCall struct {
	Tool string         // Tool name
	Args map[string]any // Arguments for the tool
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
