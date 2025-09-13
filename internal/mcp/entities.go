package mcp

import (
	"fmt"
	"strings"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"silvia/internal/graph"
	"silvia/internal/llm"
)

type SearchArgs struct {
	Query string `json:"query" jsonschema:"required,description=Search query for entities"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results (default 10)"`
}

type ReadEntityArgs struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=Entity ID (e.g. people/douglas-wilson)"`
}

type CreateEntityArgs struct {
	EntityType string `json:"entity_type" jsonschema:"required,enum=people;organizations;concepts;works;events,description=Type of entity"`
	EntityID   string `json:"entity_id" jsonschema:"required,description=Entity ID (e.g. douglas-wilson for people/douglas-wilson)"`
	Name       string `json:"name" jsonschema:"required,description=Display name for the entity"`
	Content    string `json:"content,omitempty" jsonschema:"description=Initial content for the entity"`
}

type UpdateEntityArgs struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=Entity ID to update"`
	Content  string `json:"content" jsonschema:"required,description=New content for the entity"`
}

type GetRelatedArgs struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=Entity ID to get relationships for"`
}

type ListEntitiesArgs struct {
	EntityType string `json:"entity_type,omitempty" jsonschema:"enum=people;organizations;concepts;works;events,description=Filter by entity type"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results (default 50)"`
}

// registerEntityTools registers all direct graph access tools
func registerEntityTools(server *mcp.Server, graphManager *graph.Manager, llmClient *llm.Client) error {
	// Search entities
	err := server.RegisterTool(
		"search_entities",
		"Search for entities in the knowledge graph",

		func(args SearchArgs) (*mcp.ToolResponse, error) {
			limit := args.Limit
			if limit == 0 {
				limit = 10
			}

			results, err := graphManager.SearchEntities(args.Query)
			if err != nil {
				return nil, fmt.Errorf("search failed: %w", err)
			}

			// Format results
			var formatted []string

			// Limit results
			if len(results) > limit {
				results = results[:limit]
			}

			for _, entity := range results {
				name := entity.Metadata.ID // Use ID as name since Name field doesn't exist
				if len(entity.Metadata.Aliases) > 0 {
					name = entity.Metadata.Aliases[0] // Use first alias if available
				}
				formatted = append(formatted, fmt.Sprintf("%s (%s): %s",
					entity.Metadata.ID,
					entity.Metadata.Type,
					name))
			}

			content := strings.Join(formatted, "\n")
			if content == "" {
				content = "No results found"
			}

			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	// Read entity
	err = server.RegisterTool(
		"read_entity",
		"Read a specific entity from the knowledge graph",

		func(args ReadEntityArgs) (*mcp.ToolResponse, error) {
			entity, err := graphManager.LoadEntity(args.EntityID)
			if err != nil {
				return nil, fmt.Errorf("failed to read entity: %w", err)
			}

			// Format entity as readable text
			name := entity.Metadata.ID
			if len(entity.Metadata.Aliases) > 0 {
				name = entity.Metadata.Aliases[0]
			}
			content := fmt.Sprintf("# %s\n\nType: %s\nID: %s\n\n%s",
				name,
				entity.Metadata.Type,
				entity.Metadata.ID,
				entity.Content,
			)

			// Add relationships if any
			if len(entity.Relationships) > 0 {
				content += "\n\n## Relationships\n"
				for _, rel := range entity.Relationships {
					content += fmt.Sprintf("- %s: [[%s]]\n", rel.Type, rel.Target)
				}
			}

			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	// Create entity
	err = server.RegisterTool(
		"create_entity",
		"Create a new entity in the knowledge graph",

		func(args CreateEntityArgs) (*mcp.ToolResponse, error) {
			// Construct full entity ID with type prefix
			fullID := fmt.Sprintf("%s/%s", args.EntityType, args.EntityID)

			// Create entity
			entity := &graph.Entity{
				Metadata: graph.Metadata{
					ID:      fullID,
					Type:    graph.EntityType(args.EntityType),
					Aliases: []string{args.Name}, // Store name as alias
					Created: time.Now(),
					Updated: time.Now(),
				},
				Content: args.Content,
			}

			// Save entity
			if err := graphManager.SaveEntity(entity); err != nil {
				return nil, fmt.Errorf("failed to create entity: %w", err)
			}

			content := fmt.Sprintf("Created entity: %s (%s)", fullID, args.Name)
			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	// Update entity
	err = server.RegisterTool(
		"update_entity",
		"Update an existing entity's content",

		func(args UpdateEntityArgs) (*mcp.ToolResponse, error) {
			// Get existing entity
			entity, err := graphManager.LoadEntity(args.EntityID)
			if err != nil {
				return nil, fmt.Errorf("failed to get entity: %w", err)
			}

			// Update content
			entity.Content = args.Content

			// Save updated entity
			if err := graphManager.SaveEntity(entity); err != nil {
				return nil, fmt.Errorf("failed to save entity: %w", err)
			}

			content := fmt.Sprintf("Updated entity: %s", args.EntityID)
			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	// Get related entities
	err = server.RegisterTool(
		"get_related_entities",
		"Get entities related to a specific entity",

		func(args GetRelatedArgs) (*mcp.ToolResponse, error) {
			entity, err := graphManager.LoadEntity(args.EntityID)
			if err != nil {
				return nil, fmt.Errorf("failed to get entity: %w", err)
			}

			// Get related entities
			related, err := graphManager.GetRelatedEntities(args.EntityID)
			if err != nil {
				return nil, fmt.Errorf("failed to get related entities: %w", err)
			}

			// Format results
			var formatted []string
			name := entity.Metadata.ID
			if len(entity.Metadata.Aliases) > 0 {
				name = entity.Metadata.Aliases[0]
			}
			formatted = append(formatted, fmt.Sprintf("Related to: %s (%s)\n", name, args.EntityID))

			// Format outgoing relationships by type
			if len(related.OutgoingByType) > 0 {
				formatted = append(formatted, "\nOutgoing relationships:")
				for relType, entities := range related.OutgoingByType {
					for _, e := range entities {
						ename := e.Metadata.ID
						if len(e.Metadata.Aliases) > 0 {
							ename = e.Metadata.Aliases[0]
						}
						formatted = append(formatted, fmt.Sprintf("- %s -> %s (%s)", relType, ename, e.Metadata.ID))
					}
				}
			}

			// Format incoming relationships by type
			if len(related.IncomingByType) > 0 {
				formatted = append(formatted, "\nIncoming relationships:")
				for relType, entities := range related.IncomingByType {
					for _, e := range entities {
						ename := e.Metadata.ID
						if len(e.Metadata.Aliases) > 0 {
							ename = e.Metadata.Aliases[0]
						}
						formatted = append(formatted, fmt.Sprintf("- %s <- %s (%s)", relType, ename, e.Metadata.ID))
					}
				}
			}

			// Show broken links if any
			if len(related.BrokenLinks) > 0 {
				formatted = append(formatted, "\nBroken links:")
				for _, link := range related.BrokenLinks {
					formatted = append(formatted, fmt.Sprintf("- [broken] %s", link))
				}
			}

			if len(related.OutgoingByType) == 0 && len(related.IncomingByType) == 0 {
				formatted = append(formatted, "No related entities found")
			}

			content := strings.Join(formatted, "\n")
			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	// List entities
	err = server.RegisterTool(
		"list_entities",
		"List all entities or filter by type",

		func(args ListEntitiesArgs) (*mcp.ToolResponse, error) {
			limit := args.Limit
			if limit == 0 {
				limit = 50
			}

			var entities []*graph.Entity
			var err error

			if args.EntityType != "" {
				// List entities of specific type
				entityType := graph.EntityType(args.EntityType)
				entities, err = graphManager.FindEntitiesByType(entityType)
			} else {
				// List all entities
				entities, err = graphManager.ListAllEntities()
			}

			if err != nil {
				return nil, fmt.Errorf("failed to list entities: %w", err)
			}

			// Limit results
			if len(entities) > limit {
				entities = entities[:limit]
			}

			// Format results
			var formatted []string
			for _, entity := range entities {
				name := entity.Metadata.ID
				if len(entity.Metadata.Aliases) > 0 {
					name = entity.Metadata.Aliases[0]
				}
				formatted = append(formatted, fmt.Sprintf("%s (%s): %s",
					entity.Metadata.ID,
					entity.Metadata.Type,
					name))
			}

			content := strings.Join(formatted, "\n")
			if content == "" {
				content = "No entities found"
			}

			return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}
