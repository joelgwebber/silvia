package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"silvia/internal/cli"
	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/sources"
)

// Queue operation arguments

type GetQueueArgs struct{}

type AddToQueueArgs struct {
	URL         string `json:"url" jsonschema:"required,description=URL to add to the queue"`
	Priority    int    `json:"priority,omitempty" jsonschema:"description=Priority (0=low 1=medium 2=high, default 1)"`
	FromSource  string `json:"from_source,omitempty" jsonschema:"description=Source this URL came from"`
	Description string `json:"description,omitempty" jsonschema:"description=Description of the source"`
}

type RemoveFromQueueArgs struct {
	URL string `json:"url" jsonschema:"required,description=URL to remove from the queue"`
}

type IngestURLArgs struct {
	URL   string `json:"url" jsonschema:"required,description=URL to ingest"`
	Force bool   `json:"force,omitempty" jsonschema:"description=Force re-ingestion even if already processed"`
}

// registerQueueTools registers queue management tools
func registerQueueTools(server *mcp.Server, graphManager *graph.Manager, llmClient *llm.Client) error {
	// Initialize queue
	dataDir := os.Getenv("SILVIA_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	queue := cli.NewSourceQueue()
	queuePath := filepath.Join(dataDir, ".silvia", "queue.json")
	queue.LoadFromFile(queuePath) // Ignore error if file doesn't exist

	sourcesManager := sources.NewManager()
	extractor := sources.NewExtractor(llmClient)
	tracker := cli.NewSourceTracker(dataDir)

	// Get queue
	err := server.RegisterTool(
		"get_queue",
		"Get the current source queue",
		func(args GetQueueArgs) (*mcp.ToolResponse, error) {
			items := queue.GetAll()

			if len(items) == 0 {
				return mcp.NewToolResponse(mcp.NewTextContent("Queue is empty")), nil
			}

			result := fmt.Sprintf("Queue has %d items:\n\n", len(items))
			for i, item := range items {
				result += fmt.Sprintf("%d. %s\n", i+1, item.URL)
				result += fmt.Sprintf("   Priority: %d, Added: %s\n", item.Priority, item.AddedAt.Format("2006-01-02 15:04"))
				if item.FromSource != "" {
					result += fmt.Sprintf("   From: %s\n", item.FromSource)
				}
				if item.Description != "" {
					result += fmt.Sprintf("   Description: %s\n", item.Description)
				}
				result += "\n"
			}

			return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
		},
	)
	if err != nil {
		return err
	}

	// Add to queue
	err = server.RegisterTool(
		"add_to_queue",
		"Add a URL to the source queue",
		func(args AddToQueueArgs) (*mcp.ToolResponse, error) {
			priority := cli.SourcePriority(args.Priority)
			if priority < cli.PriorityLow || priority > cli.PriorityHigh {
				priority = cli.PriorityMedium
			}

			if !queue.Add(args.URL, priority, args.FromSource, args.Description) {
				return mcp.NewToolResponse(mcp.NewTextContent(
					fmt.Sprintf("URL %s is already in the queue", args.URL),
				)), nil
			}

			// Save queue
			if err := queue.SaveToFile(); err != nil {
				return nil, fmt.Errorf("failed to save queue: %w", err)
			}

			return mcp.NewToolResponse(mcp.NewTextContent(
				fmt.Sprintf("Added %s to queue with priority %d", args.URL, priority),
			)), nil
		},
	)
	if err != nil {
		return err
	}

	// Remove from queue
	err = server.RegisterTool(
		"remove_from_queue",
		"Remove a URL from the source queue",
		func(args RemoveFromQueueArgs) (*mcp.ToolResponse, error) {
			if queue.Remove(args.URL) {
				// Save queue
				if err := queue.SaveToFile(); err != nil {
					return nil, fmt.Errorf("failed to save queue: %w", err)
				}
				return mcp.NewToolResponse(mcp.NewTextContent(
					fmt.Sprintf("Removed %s from queue", args.URL),
				)), nil
			}

			return mcp.NewToolResponse(mcp.NewTextContent(
				fmt.Sprintf("URL %s not found in queue", args.URL),
			)), nil
		},
	)
	if err != nil {
		return err
	}

	// Ingest URL directly
	err = server.RegisterTool(
		"ingest_url",
		"Ingest a URL directly (fetch, extract entities, and add to graph)",
		func(args IngestURLArgs) (*mcp.ToolResponse, error) {
			// Check if already processed
			if !args.Force && tracker.IsProcessed(args.URL) {
				return mcp.NewToolResponse(mcp.NewTextContent(
					fmt.Sprintf("URL %s has already been processed. Use force=true to reprocess.", args.URL),
				)), nil
			}

			// Fetch the content
			ctx := context.Background()
			source, err := sourcesManager.Fetch(ctx, args.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch %s: %w", args.URL, err)
			}

			// Save source to disk
			sourcesDir := filepath.Join(dataDir, "sources", "web")
			if err := os.MkdirAll(sourcesDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create sources directory: %w", err)
			}

			// Generate filename from URL
			domain := sources.ExtractDomain(args.URL)
			timestamp := time.Now().Format("20060102-150405")
			filename := fmt.Sprintf("%s-%s.md", strings.ReplaceAll(domain, ".", "-"), timestamp)
			sourcePath := filepath.Join(sourcesDir, filename)

			// Write source content
			content := fmt.Sprintf("# %s\n\nURL: %s\nFetched: %s\n\n---\n\n%s",
				source.Title, source.URL, time.Now().Format(time.RFC3339), source.Content)
			if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to save source: %w", err)
			}

			// Extract entities
			extraction, err := extractor.Extract(ctx, source)
			if err != nil {
				return nil, fmt.Errorf("failed to extract entities: %w", err)
			}

			// Process extracted entities
			var created, updated int
			for _, entity := range extraction.Entities {
				// Generate entity ID
				id := generateEntityID(entity.Name, entity.Type)

				// Check if entity exists
				existing, _ := graphManager.LoadEntity(id)
				if existing != nil {
					// Update existing entity - merge content
					if entity.Content != "" && existing.Content != "" && existing.Content != entity.Content {
						existing.Content = existing.Content + "\n\n---\n\n" + entity.Content
					} else if entity.Content != "" {
						existing.Content = entity.Content
					}

					// Add new wiki links as relationships
					for _, link := range entity.WikiLinks {
						// Check if relationship already exists
						found := false
						for _, existingRel := range existing.Relationships {
							if existingRel.Type == "mentioned_in" && existingRel.Target == link {
								found = true
								break
							}
						}
						if !found {
							existing.Relationships = append(existing.Relationships, graph.Relationship{
								Type:   "mentioned_in",
								Target: link,
							})
						}
					}

					// Update timestamp
					existing.Metadata.Updated = time.Now()

					// Save updated entity
					if err := graphManager.SaveEntity(existing); err != nil {
						return nil, fmt.Errorf("failed to save entity %s: %w", id, err)
					}
					updated++
				} else {
					// Create new entity
					graphEntity := &graph.Entity{
						Metadata: graph.Metadata{
							ID:      id,
							Type:    entity.Type,
							Aliases: []string{entity.Name},
							Created: time.Now(),
							Updated: time.Now(),
						},
						Content: entity.Content,
					}

					// Convert wiki links to relationships
					for _, link := range entity.WikiLinks {
						graphEntity.Relationships = append(graphEntity.Relationships, graph.Relationship{
							Type:   "mentioned_in",
							Target: link,
						})
					}

					// Save new entity
					if err := graphManager.SaveEntity(graphEntity); err != nil {
						return nil, fmt.Errorf("failed to save entity %s: %w", id, err)
					}
					created++
				}
			}

			// Mark as processed
			tracker.MarkProcessed(args.URL, source.Title, sourcePath)

			// Remove from queue if present
			queue.Remove(args.URL)
			queue.SaveToFile()

			result := fmt.Sprintf("Successfully ingested %s\n", args.URL)
			result += fmt.Sprintf("Saved to: %s\n", sourcePath)
			result += fmt.Sprintf("Created %d new entities, updated %d existing entities\n", created, updated)

			if len(extraction.Entities) > 0 {
				result += "\nEntities processed:\n"
				for _, entity := range extraction.Entities {
					result += fmt.Sprintf("- %s (%s): %s\n", entity.Name, entity.Type, generateEntityID(entity.Name, entity.Type))
				}
			}

			return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// generateEntityID creates a consistent ID for an entity
func generateEntityID(name string, entityType graph.EntityType) string {
	// Clean the name for use in ID
	cleaned := strings.ToLower(name)
	cleaned = strings.ReplaceAll(cleaned, " ", "-")
	cleaned = strings.ReplaceAll(cleaned, "'", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	cleaned = strings.ReplaceAll(cleaned, ";", "")
	cleaned = strings.ReplaceAll(cleaned, "?", "")
	cleaned = strings.ReplaceAll(cleaned, "!", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	cleaned = strings.ReplaceAll(cleaned, "[", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")
	cleaned = strings.ReplaceAll(cleaned, "\\", "-")

	// Remove multiple dashes
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}

	// Trim dashes
	cleaned = strings.Trim(cleaned, "-")

	// Add type prefix
	return fmt.Sprintf("%s/%s", entityType, cleaned)
}
