package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"silvia/internal/operations"
)

// RegisterOperationsTools registers all operations-based tools with the MCP server
func RegisterOperationsTools(server *mcp.Server, ops *operations.Operations) error {
	// Register entity operations
	if err := registerEntityOperations(server, ops.Entity); err != nil {
		return fmt.Errorf("failed to register entity operations: %w", err)
	}

	// Register queue operations
	if err := registerQueueOperations(server, ops.Queue); err != nil {
		return fmt.Errorf("failed to register queue operations: %w", err)
	}

	// Register search operations
	if err := registerSearchOperations(server, ops.Search); err != nil {
		return fmt.Errorf("failed to register search operations: %w", err)
	}

	// Register source operations
	if err := registerSourceOperations(server, ops.Source); err != nil {
		return fmt.Errorf("failed to register source operations: %w", err)
	}

	return nil
}

func registerEntityOperations(server *mcp.Server, entityOps *operations.EntityOps) error {
	// Merge entities
	err := server.RegisterTool(
		"merge_entities",
		"Merge entity2 into entity1, updating all references",
		func(args struct {
			Entity1ID string `json:"entity1_id" jsonschema:"required,description=ID of entity to merge into"`
			Entity2ID string `json:"entity2_id" jsonschema:"required,description=ID of entity to merge from"`
		}) (*mcp.ToolResponse, error) {
			ctx := context.Background()
			result, err := entityOps.MergeEntities(ctx, args.Entity1ID, args.Entity2ID)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Successfully merged %s into %s. Updated %d references.",
				result.DeletedEntityID, args.Entity1ID, len(result.UpdatedFiles))

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	// Rename entity
	err = server.RegisterTool(
		"rename_entity",
		"Rename an entity and update all references",
		func(args struct {
			OldID string `json:"old_id" jsonschema:"required,description=Current entity ID"`
			NewID string `json:"new_id" jsonschema:"required,description=New entity ID"`
		}) (*mcp.ToolResponse, error) {
			result, err := entityOps.RenameEntity(args.OldID, args.NewID)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Successfully renamed %s to %s. Updated %d references.",
				result.OldID, result.NewID, len(result.UpdatedFiles))

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	// Create entity
	err = server.RegisterTool(
		"create_entity",
		"Create a new entity",
		func(args struct {
			Type    string `json:"type" jsonschema:"required,description=Entity type (person/organization/concept/work/event)"`
			ID      string `json:"id" jsonschema:"required,description=Entity ID"`
			Title   string `json:"title" jsonschema:"required,description=Entity title"`
			Content string `json:"content" jsonschema:"description=Entity content"`
		}) (*mcp.ToolResponse, error) {
			entity, err := entityOps.CreateEntity(args.Type, args.ID, args.Title, args.Content)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Created entity %s (%s)", entity.Metadata.ID, entity.Title)

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func registerQueueOperations(server *mcp.Server, queueOps *operations.QueueOps) error {
	// Get queue
	err := server.RegisterTool(
		"get_queue",
		"Get the current source queue status",
		func(args struct{}) (*mcp.ToolResponse, error) {
			status, err := queueOps.GetQueue()
			if err != nil {
				return nil, err
			}

			// Format as JSON for better structure
			data, _ := json.MarshalIndent(status, "", "  ")

			return mcp.NewToolResponse(mcp.NewTextContent(string(data))), nil
		},
	)
	if err != nil {
		return err
	}

	// Add to queue
	err = server.RegisterTool(
		"add_to_queue",
		"Add a URL to the processing queue",
		func(args struct {
			URL         string `json:"url" jsonschema:"required,description=URL to add"`
			Priority    int    `json:"priority" jsonschema:"description=Priority (0=low 1=medium 2=high)"`
			FromSource  string `json:"from_source" jsonschema:"description=Source this came from"`
			Description string `json:"description" jsonschema:"description=Why this URL is being added"`
		}) (*mcp.ToolResponse, error) {
			err := queueOps.AddToQueue(args.URL, args.Priority, args.FromSource, args.Description)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Added %s to queue with priority %d", args.URL, args.Priority)

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	// Remove from queue
	err = server.RegisterTool(
		"remove_from_queue",
		"Remove a URL from the processing queue",
		func(args struct {
			URL string `json:"url" jsonschema:"required,description=URL to remove"`
		}) (*mcp.ToolResponse, error) {
			err := queueOps.RemoveFromQueue(args.URL)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Removed %s from queue", args.URL)

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func registerSearchOperations(server *mcp.Server, searchOps *operations.SearchOps) error {
	// Search entities
	err := server.RegisterTool(
		"search_entities",
		"Search for entities in the knowledge graph",
		func(args struct {
			Query string `json:"query" jsonschema:"required,description=Search query"`
		}) (*mcp.ToolResponse, error) {
			result, err := searchOps.SearchEntities(args.Query)
			if err != nil {
				return nil, err
			}

			// Format results
			var response string
			if result.Total == 0 {
				response = "No entities found"
			} else {
				response = fmt.Sprintf("Found %d entities:\n", result.Total)
				for i, match := range result.Results {
					if i >= 10 { // Limit to 10 results
						response += fmt.Sprintf("\n... and %d more", result.Total-10)
						break
					}
					response += fmt.Sprintf("\n%d. %s (%s) - %s",
						i+1, match.Entity.Title, match.Entity.Metadata.ID, match.Snippet)
				}
			}

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	// Get related entities
	err = server.RegisterTool(
		"get_related_entities",
		"Get all entities related to a specific entity",
		func(args struct {
			EntityID string `json:"entity_id" jsonschema:"required,description=Entity ID"`
		}) (*mcp.ToolResponse, error) {
			result, err := searchOps.GetRelatedEntities(args.EntityID)
			if err != nil {
				return nil, err
			}

			// Format results
			var response string
			response = fmt.Sprintf("Related entities for %s:\n", result.Entity.Title)

			// Outgoing relationships
			if len(result.OutgoingByType) > 0 {
				response += "\nOutgoing relationships:\n"
				for relType, entities := range result.OutgoingByType {
					response += fmt.Sprintf("  %s: %d entities\n", relType, len(entities))
				}
			}

			// Incoming relationships
			if len(result.IncomingByType) > 0 {
				response += "\nIncoming relationships:\n"
				for relType, entities := range result.IncomingByType {
					response += fmt.Sprintf("  %s: %d entities\n", relType, len(entities))
				}
			}

			if len(result.BrokenLinks) > 0 {
				response += fmt.Sprintf("\nBroken links: %d\n", len(result.BrokenLinks))
			}

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func registerSourceOperations(server *mcp.Server, sourceOps *operations.SourceOps) error {
	// Ingest source
	err := server.RegisterTool(
		"ingest_url",
		"Ingest a URL and extract entities",
		func(args struct {
			URL   string `json:"url" jsonschema:"required,description=URL to ingest"`
			Force bool   `json:"force" jsonschema:"description=Force re-ingestion"`
		}) (*mcp.ToolResponse, error) {
			ctx := context.Background()
			result, err := sourceOps.IngestSource(ctx, args.URL, args.Force)
			if err != nil {
				return nil, err
			}

			response := fmt.Sprintf("Ingested %s\nExtracted %d entities and %d links\nArchived to: %s\nProcessing time: %v",
				result.SourceURL,
				len(result.ExtractedEntities),
				len(result.ExtractedLinks),
				result.ArchivedPath,
				result.ProcessingTime)

			return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}
