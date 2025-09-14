package operations

import (
	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/sources"
)

// New creates a new Operations instance with all sub-operations
func New(graphManager *graph.Manager, llmClient *llm.Client, sourcesManager *sources.Manager, dataDir string) *Operations {
	ops := &Operations{
		Entity: NewEntityOps(graphManager, llmClient, dataDir),
		Queue:  NewQueueOps(dataDir),
		Source: NewSourceOps(graphManager, llmClient, sourcesManager, dataDir),
		Search: NewSearchOps(graphManager, dataDir),
		LLM:    NewLLMOps(llmClient),
	}

	return ops
}

// NewWithDefaults creates Operations with default configurations
func NewWithDefaults(dataDir string) *Operations {
	// Initialize graph manager
	graphManager := graph.NewManager(dataDir)

	// Initialize sources manager
	sourcesManager := sources.NewManager()

	// LLM client is optional - operations should handle nil gracefully
	var llmClient *llm.Client

	return New(graphManager, llmClient, sourcesManager, dataDir)
}
