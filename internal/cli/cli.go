package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/operations"
	"silvia/internal/sources"
	"silvia/internal/term"
	"silvia/internal/tools"
)

// CLI provides the interactive command-line interface
type CLI struct {
	graph      *graph.Manager
	llm        *llm.Client
	queue      *SourceQueue
	readline   *readline.Instance
	sources    *sources.Manager
	extractor  *sources.Extractor
	tracker    *SourceTracker
	registry   *CommandRegistry
	tools      *tools.Manager         // Tool manager for operations
	ops        *operations.Operations // Unified operations layer
	termWriter *term.OSCWriter
	dataDir    string
	debug      bool
}

// NewCLI creates a new CLI instance
func NewCLI(graphManager *graph.Manager, llmClient *llm.Client) *CLI {
	dataDir := "data" // Default data directory
	sourcesManager := sources.NewManager()

	// Create operations layer
	ops := operations.New(graphManager, llmClient, sourcesManager, dataDir)

	// Create tool manager
	var toolsMgr *tools.Manager
	if ops != nil {
		toolsMgr = tools.NewManager(ops)
	}

	return &CLI{
		graph:      graphManager,
		llm:        llmClient,
		queue:      NewSourceQueue(),
		sources:    sourcesManager,
		extractor:  sources.NewExtractor(llmClient),
		tracker:    NewSourceTracker(dataDir),
		registry:   NewCommandRegistry(),
		tools:      toolsMgr,
		ops:        ops,
		termWriter: term.NewOSCWriter(os.Stdout),
		dataDir:    dataDir,
	}
}

// NewCLIWithOperations creates a new CLI instance with explicit operations
func NewCLIWithOperations(ops *operations.Operations, graphManager *graph.Manager, llmClient *llm.Client) *CLI {
	dataDir := "data" // Default data directory
	sourcesManager := sources.NewManager()

	// Create tool manager
	var toolsMgr *tools.Manager
	if ops != nil {
		toolsMgr = tools.NewManager(ops)
	}

	return &CLI{
		graph:      graphManager,
		llm:        llmClient,
		queue:      NewSourceQueue(),
		sources:    sourcesManager,
		extractor:  sources.NewExtractor(llmClient),
		tracker:    NewSourceTracker(dataDir),
		registry:   NewCommandRegistry(),
		tools:      toolsMgr,
		ops:        ops,
		termWriter: term.NewOSCWriter(os.Stdout),
		dataDir:    dataDir,
	}
}

// GetOperations returns the operations layer
func (c *CLI) GetOperations() *operations.Operations {
	return c.ops
}

// LoadQueue loads the queue from a file
func (c *CLI) LoadQueue(filePath string) error {
	return c.queue.LoadFromFile(filePath)
}

// SetDebug enables or disables debug mode
func (c *CLI) SetDebug(debug bool) {
	c.debug = debug
	if c.extractor != nil {
		c.extractor.SetDebug(debug)
	}
	// Enable verbose tool logging in debug mode
	if c.tools != nil && debug {
		c.tools.EnableVerboseLogging()
	}
}

// Run starts the interactive CLI session
func (c *CLI) Run(ctx context.Context) error {
	// Initialize readline with autocompletion
	config := &readline.Config{
		Prompt:            "> ",
		HistoryFile:       filepath.Join(os.TempDir(), ".silvia_history"),
		AutoComplete:      c.buildAutoCompleter(),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	c.readline = rl
	defer rl.Close()

	fmt.Println("Welcome to silvia - Knowledge Graph Explorer")
	fmt.Println("Type /help for commands, or just chat naturally.")
	fmt.Println()

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for exit commands
		if line == "/exit" || line == "/quit" || line == "/q" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Process the command
		if err := c.processInput(ctx, line); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

// buildAutoCompleter creates the autocompletion configuration from the registry
func (c *CLI) buildAutoCompleter() *readline.PrefixCompleter {
	var items []readline.PrefixCompleterInterface
	processed := make(map[string]bool)

	for _, cmd := range c.registry.GetAll() {
		// Skip if we've already processed this command (aliases handled separately)
		if processed[cmd.Name] {
			continue
		}
		processed[cmd.Name] = true

		// Create completion item
		var item readline.PrefixCompleterInterface

		if cmd.Dynamic {
			// Commands with dynamic entity ID completion
			item = readline.PcItem(cmd.Name, readline.PcItemDynamic(c.listEntityIDs()))
		} else if len(cmd.SubCommands) > 0 {
			// Commands with sub-commands
			var subItems []readline.PrefixCompleterInterface
			for _, sub := range cmd.SubCommands {
				subItems = append(subItems, readline.PcItem(sub))
			}
			item = readline.PcItem(cmd.Name, subItems...)
		} else {
			// Simple commands
			item = readline.PcItem(cmd.Name)
		}
		items = append(items, item)

		// Add aliases as separate items
		for _, alias := range cmd.Aliases {
			if cmd.Dynamic {
				items = append(items, readline.PcItem(alias, readline.PcItemDynamic(c.listEntityIDs())))
			} else if len(cmd.SubCommands) > 0 {
				var subItems []readline.PrefixCompleterInterface
				for _, sub := range cmd.SubCommands {
					subItems = append(subItems, readline.PcItem(sub))
				}
				items = append(items, readline.PcItem(alias, subItems...))
			} else {
				items = append(items, readline.PcItem(alias))
			}
		}
	}

	return readline.NewPrefixCompleter(items...)
}

// listEntityIDs returns a function that lists all entity IDs for autocompletion
func (c *CLI) listEntityIDs() func(string) []string {
	return func(line string) []string {
		entities, err := c.graph.ListAllEntities()
		if err != nil {
			return []string{}
		}

		var ids []string
		for _, entity := range entities {
			ids = append(ids, entity.Metadata.ID)
		}
		return ids
	}
}

// processInput handles user input
func (c *CLI) processInput(ctx context.Context, input string) error {
	// Check if it's a slash command
	if strings.HasPrefix(input, "/") {
		return c.processCommand(ctx, input)
	}

	// Otherwise treat as natural language query
	return c.handleNaturalQuery(ctx, input)
}

// processCommand handles slash commands using the registry
func (c *CLI) processCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	commandName := strings.ToLower(parts[0])
	args := parts[1:]

	// Special case for /explore queue
	if commandName == "/explore" && len(args) > 0 && args[0] == "queue" {
		return c.InteractiveQueueExplorer(ctx)
	}

	// Look up command in registry
	cmd, ok := c.registry.Get(commandName)
	if !ok {
		return fmt.Errorf("unknown command: %s (type /help for commands)", commandName)
	}

	// Execute command handler if it exists
	if cmd.Handler != nil {
		return cmd.Handler(ctx, c, args)
	}

	return nil
}

// showHelpFromRegistry displays help text generated from the command registry
func (c *CLI) showHelpFromRegistry() {
	fmt.Println("\nAvailable Commands:")

	// Calculate max width for formatting
	maxWidth := 0
	for _, cmd := range c.registry.GetAll() {
		usageStr := cmd.Name
		if cmd.Usage != "" {
			usageStr += " " + cmd.Usage
		}
		if len(usageStr) > maxWidth {
			maxWidth = len(usageStr)
		}
	}

	// Display commands
	for _, cmd := range c.registry.GetAll() {
		usageStr := cmd.Name
		if cmd.Usage != "" {
			usageStr += " " + cmd.Usage
		}

		// Add aliases to description if present
		desc := cmd.Description
		if len(cmd.Aliases) > 0 {
			desc = fmt.Sprintf("%s (aliases: %s)", desc, strings.Join(cmd.Aliases, ", "))
		}

		// Format with padding
		padding := maxWidth - len(usageStr) + 3
		fmt.Printf("  %s%s- %s\n", usageStr, strings.Repeat(" ", padding), desc)
	}

	fmt.Println("\nTips:")
	fmt.Println("  • Use Tab for command and entity ID autocompletion")
	fmt.Println("  • Use ↑/↓ arrows to navigate command history")
	fmt.Println("  • Type without / for natural language queries")
	fmt.Println()
}

// showEntity displays an entity's details
func (c *CLI) showEntity(entityID string) error {
	// Try to read entity using tools
	result, err := c.tools.Execute(context.Background(), "read_entity", map[string]any{
		"id": entityID,
	})

	if err != nil || !result.Success {
		// Try searching if exact ID doesn't match
		searchResult, searchErr := c.tools.Execute(context.Background(), "search_entities", map[string]any{
			"query": entityID,
		})

		if searchErr != nil || !searchResult.Success {
			return fmt.Errorf("entity not found: %v", err)
		}

		// Handle SearchResult type
		var matches []operations.SearchMatch
		if sr, ok := searchResult.Data.(*operations.SearchResult); ok {
			matches = sr.Results
		} else {
			return fmt.Errorf("unexpected search result type")
		}

		if len(matches) == 0 {
			return fmt.Errorf("entity not found: %v", err)
		}

		if len(matches) == 1 {
			// Load the single match
			entityID = matches[0].Entity.Metadata.ID
			result, err = c.tools.Execute(context.Background(), "read_entity", map[string]any{
				"id": entityID,
			})
			if err != nil || !result.Success {
				return fmt.Errorf("failed to load entity: %v", err)
			}
		} else {
			fmt.Println("Multiple matches found:")
			for i, match := range matches {
				fmt.Printf("%d. %s (%s)\n", i+1, match.Entity.Title, match.Entity.Metadata.ID)
			}
			return nil
		}
	}

	entity := result.Data.(*graph.Entity)

	// Display entity
	fmt.Printf("\n%s %s\n", getEntityIcon(entity.Metadata.Type), entity.Title)
	fmt.Printf("ID: %s\n", entity.Metadata.ID)
	fmt.Printf("Type: %s\n", entity.Metadata.Type)

	if len(entity.Metadata.Aliases) > 0 {
		fmt.Printf("Aliases: %s\n", strings.Join(entity.Metadata.Aliases, ", "))
	}

	if entity.Content != "" {
		fmt.Printf("\n%s\n", entity.Content)
	}

	// Show relationships
	if len(entity.Relationships) > 0 {
		fmt.Println("\nRelationships:")
		for _, rel := range entity.Relationships {
			fmt.Printf("  → %s: %s", rel.Type, rel.Target)
			if rel.Note != "" {
				fmt.Printf(" (%s)", rel.Note)
			}
			fmt.Println()
		}
	}

	// Show back-references
	if len(entity.BackRefs) > 0 {
		fmt.Println("\nReferenced by:")
		for _, ref := range entity.BackRefs {
			fmt.Printf("  ← %s (%s)", ref.Source, ref.Type)
			if ref.Note != "" {
				fmt.Printf(" - %s", ref.Note)
			}
			fmt.Println()
		}
	}

	fmt.Println()
	return nil
}

// searchEntities searches for entities matching a query
func (c *CLI) searchEntities(query string) error {
	result, err := c.tools.Execute(context.Background(), "search_entities", map[string]any{
		"query": query,
	})

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("search failed: %s", result.Error)
	}

	matches := result.Data.([]map[string]any)

	if len(matches) == 0 {
		fmt.Println("No entities found.")
		return nil
	}

	fmt.Printf("\nFound %d entities:\n", len(matches))
	for _, match := range matches {
		// Handle type conversion - could be string or EntityType
		var entityType graph.EntityType
		switch t := match["type"].(type) {
		case string:
			entityType = graph.EntityType(t)
		case graph.EntityType:
			entityType = t
		default:
			entityType = graph.EntityPerson // default
		}
		fmt.Printf("  %s %s (%s)\n",
			getEntityIcon(entityType),
			match["title"],
			match["id"])
		if excerpt, ok := match["excerpt"].(string); ok && excerpt != "" {
			// Show excerpt (already truncated by tool)
			lines := strings.Split(excerpt, "\n")
			if len(lines) > 0 && lines[0] != "" {
				preview := lines[0]
				if len(preview) > 60 {
					preview = preview[:60] + "..."
				}
				fmt.Printf("    %s\n", preview)
			}
		}
	}
	fmt.Println()

	return nil
}

// showRelated shows entities related to the given entity
func (c *CLI) showRelated(entityID string) error {
	result, err := c.graph.GetRelatedEntities(entityID)
	if err != nil {
		return fmt.Errorf("failed to get related entities: %w", err)
	}

	if len(result.All) == 0 && len(result.BrokenLinks) == 0 {
		fmt.Println("No related entities found.")
		return nil
	}

	// Display results
	fmt.Printf("\n📊 Related entities for: %s %s\n",
		getEntityIcon(result.Entity.Metadata.Type), result.Entity.Title)
	fmt.Println(strings.Repeat("─", 60))

	// Show outgoing relationships by type
	if len(result.OutgoingByType) > 0 {
		fmt.Println(SubheaderStyle.Render("→ Outgoing:"))
		for relType, entities := range result.OutgoingByType {
			// Format the relationship type for display
			displayType := strings.ReplaceAll(relType, "_", " ")
			displayType = strings.Title(displayType)

			fmt.Printf("  %s:\n", InfoStyle.Render(displayType))
			for _, e := range entities {
				fmt.Printf("    %s %s %s\n",
					getEntityIcon(e.Metadata.Type),
					HighlightStyle.Render(e.Title),
					DimStyle.Render("("+e.Metadata.ID+")"))
			}
		}
		fmt.Println()
	}

	// Show incoming relationships by type
	if len(result.IncomingByType) > 0 {
		fmt.Println(SubheaderStyle.Render("← Incoming:"))
		for relType, entities := range result.IncomingByType {
			// Format the relationship type for display
			displayType := strings.ReplaceAll(relType, "_", " ")
			displayType = strings.Title(displayType)

			fmt.Printf("  %s:\n", InfoStyle.Render(displayType))
			for _, e := range entities {
				fmt.Printf("    %s %s %s\n",
					getEntityIcon(e.Metadata.Type),
					HighlightStyle.Render(e.Title),
					DimStyle.Render("("+e.Metadata.ID+")"))
			}
		}
		fmt.Println()
	}

	// Show broken links
	if len(result.BrokenLinks) > 0 {
		fmt.Println(WarningStyle.Render("⚠️  Broken links (entities not found):"))
		for _, link := range result.BrokenLinks {
			fmt.Printf("  %s %s\n",
				ErrorStyle.Render("✗"),
				DimStyle.Render(link))
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d related entities", len(result.All))
	if len(result.BrokenLinks) > 0 {
		fmt.Printf(" (%s)", WarningStyle.Render(fmt.Sprintf("%d broken links", len(result.BrokenLinks))))
	}
	fmt.Println()
	return nil
}

// createEntity creates a new entity interactively
func (c *CLI) createEntity(entityType, id string) error {
	// Validate entity type
	var eType graph.EntityType
	switch strings.ToLower(entityType) {
	case "person":
		eType = graph.EntityPerson
	case "organization", "org":
		eType = graph.EntityOrganization
	case "concept":
		eType = graph.EntityConcept
	case "work":
		eType = graph.EntityWork
	case "event":
		eType = graph.EntityEvent
	default:
		return fmt.Errorf("invalid entity type: %s", entityType)
	}

	// Create entity
	entity := graph.NewEntity(id, eType)

	// Get title
	c.readline.SetPrompt("Title: ")
	title, err := c.readline.Readline()
	if err != nil {
		return fmt.Errorf("cancelled")
	}
	entity.Title = strings.TrimSpace(title)

	// Get description
	fmt.Println("Description (press Enter twice to finish):")
	c.readline.SetPrompt("")
	var descLines []string
	emptyCount := 0
	for {
		line, err := c.readline.Readline()
		if err != nil {
			break
		}
		if line == "" {
			emptyCount++
			if emptyCount >= 1 {
				break
			}
		} else {
			emptyCount = 0
		}
		descLines = append(descLines, line)
	}
	entity.Content = strings.Join(descLines, "\n")

	// Restore prompt
	c.readline.SetPrompt("> ")

	// Save entity
	if err := c.graph.SaveEntity(entity); err != nil {
		return fmt.Errorf("failed to save entity: %w", err)
	}

	fmt.Printf("✅ Created entity: %s\n", entity.Metadata.ID)
	return nil
}

// createLink creates a relationship between two entities
func (c *CLI) createLink(sourceID, relType, targetID string) error {
	// Load source entity
	source, err := c.graph.LoadEntity(sourceID)
	if err != nil {
		return fmt.Errorf("source entity not found: %w", err)
	}

	// Check if target exists
	if !c.graph.EntityExists(targetID) {
		return fmt.Errorf("target entity not found: %s", targetID)
	}

	// Add relationship
	source.AddRelationship(relType, targetID, nil, "")

	// Save entity (this will also update back-references)
	if err := c.graph.SaveEntity(source); err != nil {
		return fmt.Errorf("failed to save relationship: %w", err)
	}

	fmt.Printf("✅ Created link: %s → %s → %s\n", sourceID, relType, targetID)
	return nil
}

// mergeEntities merges two entities into one
func (c *CLI) mergeEntities(ctx context.Context, entity1ID, entity2ID string) error {
	// Validate both entities exist
	entity1, err := c.ops.Entity.ReadEntity(entity1ID)
	if err != nil {
		return fmt.Errorf("first entity not found: %w", err)
	}

	entity2, err := c.ops.Entity.ReadEntity(entity2ID)
	if err != nil {
		return fmt.Errorf("second entity not found: %w", err)
	}

	// Confirm with user
	fmt.Printf("\n⚠️  This will merge:\n")
	fmt.Printf("  %s (%s)\n", entity2.Title, entity2ID)
	fmt.Printf("  INTO\n")
	fmt.Printf("  %s (%s)\n", entity1.Title, entity1ID)
	fmt.Printf("\nThe second entity will be deleted and all references updated.\n")
	fmt.Print("Proceed? (y/N):\n")

	confirmation, err := c.readline.Readline()
	if err != nil || strings.ToLower(strings.TrimSpace(confirmation)) != "y" {
		fmt.Println("Merge cancelled.")
		return nil
	}

	// Perform the merge using operations
	result, err := c.ops.Entity.MergeEntities(ctx, entity1ID, entity2ID)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	fmt.Printf("\n✅ Successfully merged %s into %s\n", result.DeletedEntityID, entity1ID)
	if len(result.UpdatedFiles) > 0 {
		fmt.Printf("Updated %d references.\n", len(result.UpdatedFiles))
	}
	return nil
}

// renameEntity renames an entity and updates all references
func (c *CLI) renameEntity(oldID, newID string) error {
	// Validate old entity exists
	oldEntity, err := c.ops.Entity.ReadEntity(oldID)
	if err != nil {
		return fmt.Errorf("entity not found: %s", oldID)
	}

	// Show what will happen
	fmt.Printf("\n⚠️  This will rename:\n")
	fmt.Printf("  %s (%s)\n", oldEntity.Title, oldID)
	fmt.Printf("  TO\n")
	fmt.Printf("  %s\n", newID)
	fmt.Printf("\nAll references will be updated throughout the graph.\n")
	fmt.Print("Proceed? (y/N):\n")

	confirmation, err := c.readline.Readline()
	if err != nil || strings.ToLower(strings.TrimSpace(confirmation)) != "y" {
		fmt.Println("Rename cancelled.")
		return nil
	}

	// Perform the rename using operations
	result, err := c.ops.Entity.RenameEntity(oldID, newID)
	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	fmt.Printf("\n✅ Successfully renamed %s to %s\n", result.OldID, result.NewID)
	if len(result.UpdatedFiles) > 0 {
		fmt.Printf("Updated %d references.\n", len(result.UpdatedFiles))
	}
	return nil
}

func (c *CLI) moveEntity(oldID, newID string) error {
	// Validate old entity exists
	oldEntity, err := c.graph.LoadEntity(oldID)
	if err != nil {
		return fmt.Errorf("entity not found: %s", oldID)
	}
	// Check if new ID already exists
	if c.graph.EntityExists(newID) {
		return fmt.Errorf("entity already exists: %s", newID)
	}

	// Extract types to show what's changing
	oldParts := strings.SplitN(oldID, "/", 2)
	newParts := strings.SplitN(newID, "/", 2)
	if len(oldParts) != 2 || len(newParts) != 2 {
		return fmt.Errorf("invalid ID format - must be 'type/name'")
	}

	// Show what will happen
	fmt.Printf("\n⚠️  This will move:\n")
	fmt.Printf("  %s (%s)\n", oldEntity.Title, oldID)
	fmt.Printf("  FROM: %s\n", oldParts[0])
	fmt.Printf("  TO:   %s (%s)\n", newParts[0], newID)
	if oldParts[0] != newParts[0] {
		fmt.Printf("\n📝 Type will change from '%s' to '%s'\n", oldParts[0], newParts[0])
	}
	fmt.Printf("\nAll references will be updated throughout the graph.\n")
	fmt.Print("Proceed? (y/N):\n")
	confirmation, err := c.readline.Readline()
	if err != nil || strings.ToLower(strings.TrimSpace(confirmation)) != "y" {
		fmt.Println("Move cancelled.")
		return nil
	}
	// Perform the move
	if err := c.graph.MoveEntity(oldID, newID); err != nil {
		return fmt.Errorf("move failed: %w", err)
	}
	fmt.Printf("\n✅ Successfully moved %s to %s\n", oldID, newID)
	return nil
}

// handleNaturalQuery processes natural language queries using tools and LLM
func (c *CLI) handleNaturalQuery(ctx context.Context, query string) error {
	fmt.Println("🔍 Analyzing your query...")

	// Use the LLM to determine which tools to use
	toolCalls, err := c.extractToolCallsForQuery(ctx, query)
	if err != nil {
		// Fallback to simple search if LLM fails
		return c.fallbackSearch(query)
	}

	// Execute the tools
	var allResults []map[string]any
	for _, call := range toolCalls {
		result, err := c.tools.Execute(ctx, call.Tool, call.Args)
		if err != nil {
			fmt.Printf("Warning: Tool %s failed: %v\n", call.Tool, err)
			continue
		}
		if result.Success && result.Data != nil {
			// Store result with tool context
			allResults = append(allResults, map[string]any{
				"tool": call.Tool,
				"args": call.Args,
				"data": result.Data,
			})
		}
	}

	// Build context from tool results ONLY
	var context strings.Builder
	context.WriteString("You are a knowledge graph assistant that ONLY uses information from the local knowledge graph.\n")
	context.WriteString("IMPORTANT: Base your response ONLY on the tool results below. Do not add information from general knowledge.\n")
	context.WriteString("If the requested information is not in the tool results, say so clearly.\n\n")
	context.WriteString("The user asked: \"" + query + "\"\n\n")

	if len(allResults) > 0 {
		context.WriteString("Tool results from the knowledge graph:\n")
		for _, result := range allResults {
			context.WriteString(fmt.Sprintf("\nTool: %s\n", result["tool"]))
			// Format the data based on tool type
			c.formatToolResultForContext(&context, result["tool"].(string), result["data"])
		}
	} else {
		context.WriteString("No relevant information found in the knowledge graph.\n")
	}

	context.WriteString("\nBased ONLY on these tool results, provide a helpful response to the user.\n")
	context.WriteString("IMPORTANT: The information above IS available in the knowledge graph - summarize it for the user.\n")
	context.WriteString("Rules:\n")
	context.WriteString("1. Only mention information that appears in the tool results above\n")
	context.WriteString("2. If a search returned results, that means the information WAS found\n")
	context.WriteString("3. Be concise but informative - summarize what you found\n")
	context.WriteString("4. Do not add facts from your general knowledge - only report what's in the results\n")
	context.WriteString("5. Do not say information is limited if you found entities - describe what you found\n")

	// Get LLM response
	if c.debug {
		fmt.Printf("DEBUG: Sending context to LLM (%d chars)\n", len(context.String()))
	}

	response, err := c.llm.Complete(ctx, context.String(), "")
	if err != nil {
		return fmt.Errorf("LLM query failed: %w", err)
	}

	if c.debug {
		fmt.Printf("DEBUG: Received response (%d chars)\n", len(response))
	}

	fmt.Printf("\n%s\n\n", response)
	return nil
}

// extractToolCallsForQuery determines which tools to use for a natural language query
func (c *CLI) extractToolCallsForQuery(ctx context.Context, query string) ([]tools.ToolCall, error) {
	// Use the new LLM operations for structured function calling if available
	if c.ops != nil && c.ops.LLM != nil && c.tools != nil {
		// Get tool schemas from the cached tool manager
		toolSchemas := c.tools.GetToolSchemas()

		response, err := c.ops.LLM.ExecuteFunctionCall(ctx, query, toolSchemas)
		if err == nil && len(response.ToolCalls) > 0 {
			// Convert LLM tool calls to our tool format
			var toolCalls []tools.ToolCall
			for _, tc := range response.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal(tc.Function.Arguments, &args); err != nil {
					if c.debug {
						fmt.Printf("Failed to unmarshal tool arguments: %v\n", err)
					}
					continue
				}
				toolCalls = append(toolCalls, tools.ToolCall{
					Tool: tc.Function.Name,
					Args: args,
				})
			}
			if c.debug {
				fmt.Printf("LLM function calling returned %d tool calls\n", len(toolCalls))
			}
			return toolCalls, nil
		}
		if c.debug && err != nil {
			fmt.Printf("LLM function calling failed: %v, falling back to manual parsing\n", err)
		}
		// Fall through to legacy method if LLM ops not available or failed
	}

	// Legacy method: Build prompt for tool extraction
	var prompt strings.Builder
	prompt.WriteString("You are a knowledge graph assistant that MUST use tools to access information.\n")
	prompt.WriteString("Analyze the user's query and determine which tools to use.\n\n")

	prompt.WriteString("Available tools:\n")
	prompt.WriteString("- search_entities: Search for entities by keyword\n")
	prompt.WriteString("- read_entity: Read a specific entity by ID\n")
	prompt.WriteString("- get_related: Get entities related to a specific entity\n\n")

	prompt.WriteString("User query: ")
	prompt.WriteString(query)
	prompt.WriteString("\n\n")

	prompt.WriteString("IMPORTANT: You MUST use tools to retrieve ANY information about entities.\n")
	prompt.WriteString("Even if you think you know the answer, you MUST use tools.\n\n")

	prompt.WriteString("Respond with tool calls in this format (one per line):\n")
	prompt.WriteString("TOOL: tool_name ARG: argument_name=value\n")
	prompt.WriteString("Example:\n")
	prompt.WriteString("TOOL: search_entities ARG: query=elon musk\n\n")
	prompt.WriteString("Your response:")

	// Get LLM response
	response, err := c.llm.Complete(ctx, prompt.String(), "")
	if err != nil {
		return nil, err
	}

	// Parse tool calls from response
	return c.parseToolCalls(response), nil
}

// parseToolCalls extracts tool calls from LLM response
func (c *CLI) parseToolCalls(response string) []tools.ToolCall {
	var calls []tools.ToolCall

	lines := strings.SplitSeq(response, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TOOL:") {
			parts := strings.Split(line, "ARG:")
			if len(parts) >= 2 {
				toolName := strings.TrimSpace(strings.TrimPrefix(parts[0], "TOOL:"))
				argPart := strings.TrimSpace(parts[1])

				// Parse arguments
				args := make(map[string]any)
				argPairs := strings.SplitSeq(argPart, ",")
				for pair := range argPairs {
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) == 2 {
						args[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}

				if toolName != "" && len(args) > 0 {
					calls = append(calls, tools.ToolCall{
						Tool: toolName,
						Args: args,
					})
				}
			}
		}
	}

	// If no explicit tools found, try to infer from query
	if len(calls) == 0 {
		// Default to searching for key terms
		calls = append(calls, tools.ToolCall{
			Tool: "search_entities",
			Args: map[string]any{
				"query": response,
			},
		})
	}

	return calls
}

// fallbackSearch performs a simple search when LLM tool extraction fails
func (c *CLI) fallbackSearch(query string) error {
	result, err := c.tools.Execute(context.Background(), "search_entities", map[string]any{
		"query": query,
	})

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if !result.Success {
		fmt.Println("No entities found.")
		return nil
	}

	// Display search results
	if matches, ok := result.Data.([]map[string]any); ok {
		if len(matches) == 0 {
			fmt.Println("No entities found.")
		} else {
			fmt.Printf("\nFound %d entities:\n", len(matches))
			for _, match := range matches {
				fmt.Printf("  %s %s (%s)\n",
					getEntityIcon(graph.EntityType(match["type"].(string))),
					match["title"],
					match["id"])
				if excerpt, ok := match["excerpt"].(string); ok && excerpt != "" {
					fmt.Printf("    %s\n", excerpt)
				}
			}
		}
	}

	return nil
}

// formatToolResultForContext formats tool results for LLM context
func (c *CLI) formatToolResultForContext(sb *strings.Builder, toolName string, data any) {
	switch toolName {
	case "search_entities":
		if matches, ok := data.([]map[string]any); ok {
			sb.WriteString("Search results:\n")
			for _, match := range matches {
				fmt.Fprintf(sb, "- %s (ID: %s, Type: %s)\n", match["title"], match["id"], match["type"])
				if excerpt, ok := match["excerpt"].(string); ok && excerpt != "" {
					fmt.Fprintf(sb, "  Content: %s\n", excerpt)
				}
			}
		}

	case "read_entity":
		if entity, ok := data.(*graph.Entity); ok {
			fmt.Fprintf(sb, "Entity: %s (ID: %s, Type: %s)\n", entity.Title, entity.Metadata.ID, entity.Metadata.Type)
			if entity.Content != "" {
				fmt.Fprintf(sb, "Content: %s\n", entity.Content)
			}
			if len(entity.Metadata.Sources) > 0 {
				fmt.Fprintf(sb, "Sources: %s\n", strings.Join(entity.Metadata.Sources, ", "))
			}
		}

	case "get_related":
		if result, ok := data.(map[string]any); ok {
			if entity, ok := result["entity"].(*graph.Entity); ok {
				fmt.Fprintf(sb, "Related to: %s\n", entity.Title)
			}
			if outgoing, ok := result["outgoing"].(map[string][]map[string]any); ok {
				for relType, entities := range outgoing {
					fmt.Fprintf(sb, "  %s:\n", relType)
					for _, e := range entities {
						fmt.Fprintf(sb, "    - %s (ID: %s)\n", e["title"], e["id"])
					}
				}
			}
		}

	default:
		// Generic formatting
		fmt.Fprintf(sb, "Result: %v\n", data)
	}
}

// getEntityIcon returns an icon for the entity type
func getEntityIcon(entityType graph.EntityType) string {
	switch entityType {
	case graph.EntityPerson:
		return "👤"
	case graph.EntityOrganization:
		return "🏢"
	case graph.EntityConcept:
		return "💭"
	case graph.EntityWork:
		return "📚"
	case graph.EntityEvent:
		return "📅"
	default:
		return "📄"
	}
}

// generateEntityID creates a standardized ID for an entity
func (c *CLI) generateEntityID(name string, entityType graph.EntityType) string {
	// Convert to lowercase and replace spaces with hyphens
	id := strings.ToLower(name)
	id = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")

	// Add type prefix
	var prefix string
	switch entityType {
	case graph.EntityPerson:
		prefix = "people/"
	case graph.EntityOrganization:
		prefix = "organizations/"
	case graph.EntityConcept:
		prefix = "concepts/"
	case graph.EntityWork:
		prefix = "works/"
	case graph.EntityEvent:
		prefix = "events/"
	default:
		prefix = "entities/"
	}

	return prefix + id
}

// saveSource saves the fetched source content to disk
func (c *CLI) saveSource(source *sources.Source) error {
	// Create filename from URL
	domain := sources.ExtractDomain(source.URL)
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", domain, timestamp)

	// Determine subdirectory based on domain
	var subdir string
	if strings.Contains(domain, "bsky") {
		subdir = "bsky"
	} else if strings.Contains(domain, ".pdf") {
		subdir = "pdfs"
	} else {
		subdir = "web"
	}

	// Create full path
	filePath := filepath.Join(c.dataDir, "sources", subdir, filename)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create markdown with metadata
	content := fmt.Sprintf(`---
url: %s
title: %s
fetched_at: %s
domain: %s
---

# %s

Source: %s

%s
`,
		source.URL,
		source.Title,
		time.Now().Format(time.RFC3339),
		domain,
		source.Title,
		source.URL,
		source.Content,
	)

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}

	// Mark as processed in tracker
	if c.tracker != nil {
		c.tracker.MarkProcessed(source.URL, source.Title, filePath)
		// Save tracker state
		if err := c.tracker.Save(); err != nil {
			// Log but don't fail
			if c.debug {
				fmt.Printf("[DEBUG] Failed to save tracker: %v\n", err)
			}
		}
	}

	return nil
}

// isSourceProcessed checks if a URL has already been processed
func (c *CLI) isSourceProcessed(url string) bool {
	// Use the tracker for exact URL matching
	if c.tracker != nil {
		return c.tracker.IsProcessed(url)
	}

	// Fallback to file-based check if tracker not available
	domain := sources.ExtractDomain(url)
	sourcesDir := filepath.Join(c.dataDir, "sources")

	// Check all subdirectories
	subdirs := []string{"web", "bsky", "pdfs"}
	for _, subdir := range subdirs {
		dir := filepath.Join(sourcesDir, subdir)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		// Check if any file contains this URL
		for _, file := range files {
			if strings.Contains(file.Name(), domain) {
				return true
			}
		}
	}

	return false
}
