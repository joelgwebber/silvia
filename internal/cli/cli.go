package cli

import (
	"context"
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
	"silvia/internal/sources"
)

// CLI provides the interactive command-line interface
type CLI struct {
	graph     *graph.Manager
	llm       *llm.Client
	queue     *SourceQueue
	readline  *readline.Instance
	sources   *sources.Manager
	extractor *sources.Extractor
	dataDir   string
}

// NewCLI creates a new CLI instance
func NewCLI(graphManager *graph.Manager, llmClient *llm.Client) *CLI {
	return &CLI{
		graph:     graphManager,
		llm:       llmClient,
		queue:     NewSourceQueue(),
		sources:   sources.NewManager(),
		extractor: sources.NewExtractor(llmClient),
		dataDir:   "data", // Default data directory
	}
}

// LoadQueue loads the queue from a file
func (c *CLI) LoadQueue(filePath string) error {
	return c.queue.LoadFromFile(filePath)
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

// buildAutoCompleter creates the autocompletion configuration
func (c *CLI) buildAutoCompleter() *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("/help"),
		readline.PcItem("/ingest"),
		readline.PcItem("/show",
			readline.PcItemDynamic(c.listEntityIDs()),
		),
		readline.PcItem("/search"),
		readline.PcItem("/queue"),
		readline.PcItem("/explore",
			readline.PcItem("queue"),
		),
		readline.PcItem("/related",
			readline.PcItemDynamic(c.listEntityIDs()),
		),
		readline.PcItem("/create",
			readline.PcItem("person"),
			readline.PcItem("organization"),
			readline.PcItem("concept"),
			readline.PcItem("work"),
			readline.PcItem("event"),
		),
		readline.PcItem("/link",
			readline.PcItemDynamic(c.listEntityIDs()),
		),
		readline.PcItem("/clear"),
		readline.PcItem("/exit"),
		readline.PcItem("/quit"),
		readline.PcItem("/q"),
	)
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

// processCommand handles slash commands
func (c *CLI) processCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "/help", "/h", "/?":
		c.showHelp()
	case "/ingest":
		if len(args) < 1 {
			return fmt.Errorf("usage: /ingest <url>")
		}
		return c.ingestSource(ctx, args[0])
	case "/show", "/view":
		if len(args) < 1 {
			return fmt.Errorf("usage: /show <entity-id>")
		}
		return c.showEntity(strings.Join(args, " "))
	case "/search", "/find":
		if len(args) < 1 {
			return fmt.Errorf("usage: /search <query>")
		}
		return c.searchEntities(strings.Join(args, " "))
	case "/queue":
		return c.showQueue()
	case "/explore":
		if len(args) > 0 && args[0] == "queue" {
			return c.exploreQueue(ctx)
		}
		return fmt.Errorf("usage: /explore queue")
	case "/related", "/connections":
		if len(args) < 1 {
			return fmt.Errorf("usage: /related <entity-id>")
		}
		return c.showRelated(strings.Join(args, " "))
	case "/create":
		if len(args) < 2 {
			return fmt.Errorf("usage: /create <type> <id>")
		}
		return c.createEntity(args[0], strings.Join(args[1:], " "))
	case "/link", "/connect":
		if len(args) < 3 {
			return fmt.Errorf("usage: /link <source-id> <rel-type> <target-id>")
		}
		return c.createLink(args[0], args[1], args[2])
	case "/clear":
		fmt.Print("\033[H\033[2J") // Clear screen
	default:
		return fmt.Errorf("unknown command: %s (type /help for commands)", command)
	}

	return nil
}

// showHelp displays available commands
func (c *CLI) showHelp() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  /help                      - Show this help message")
	fmt.Println("  /ingest <url>              - Ingest a new source")
	fmt.Println("  /show <entity-id>          - Display an entity (tab for autocomplete)")
	fmt.Println("  /search <query>            - Search for entities")
	fmt.Println("  /queue                     - Show pending sources")
	fmt.Println("  /explore queue             - Process sources in queue")
	fmt.Println("  /related <entity-id>       - Show related entities (tab for autocomplete)")
	fmt.Println("  /create <type> <id>        - Create new entity")
	fmt.Println("  /link <from> <type> <to>   - Create relationship")
	fmt.Println("  /clear                     - Clear screen")
	fmt.Println("  /exit, /quit, /q           - Exit the program")
	fmt.Println("\nTips:")
	fmt.Println("  • Use Tab for command and entity ID autocompletion")
	fmt.Println("  • Use ↑/↓ arrows to navigate command history")
	fmt.Println("  • Type without / for natural language queries")
	fmt.Println()
}

// showEntity displays an entity's details
func (c *CLI) showEntity(entityID string) error {
	entity, err := c.graph.LoadEntity(entityID)
	if err != nil {
		// Try searching if exact ID doesn't match
		matches, searchErr := c.graph.SearchEntities(entityID)
		if searchErr != nil || len(matches) == 0 {
			return fmt.Errorf("entity not found: %v", err)
		}

		if len(matches) == 1 {
			entity = matches[0]
		} else {
			fmt.Println("Multiple matches found:")
			for i, match := range matches {
				fmt.Printf("%d. %s (%s)\n", i+1, match.Title, match.Metadata.ID)
			}
			return nil
		}
	}

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
	matches, err := c.graph.SearchEntities(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(matches) == 0 {
		fmt.Println("No entities found.")
		return nil
	}

	fmt.Printf("\nFound %d entities:\n", len(matches))
	for _, entity := range matches {
		fmt.Printf("  %s %s (%s)\n",
			getEntityIcon(entity.Metadata.Type),
			entity.Title,
			entity.Metadata.ID)
		if entity.Content != "" {
			// Show first line of content
			lines := strings.Split(entity.Content, "\n")
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
	related, err := c.graph.GetRelatedEntities(entityID)
	if err != nil {
		return fmt.Errorf("failed to get related entities: %w", err)
	}

	if len(related) == 0 {
		fmt.Println("No related entities found.")
		return nil
	}

	fmt.Printf("\n%d related entities:\n", len(related))
	for _, entity := range related {
		fmt.Printf("  %s %s (%s)\n",
			getEntityIcon(entity.Metadata.Type),
			entity.Title,
			entity.Metadata.ID)
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

// handleNaturalQuery processes natural language queries using the LLM
func (c *CLI) handleNaturalQuery(ctx context.Context, query string) error {
	fmt.Println("🔍 Analyzing your query...")

	// First, search for relevant entities
	// This is a simplified version - in production, we'd use the LLM to extract entity names
	words := strings.Fields(query)
	var relevantEntities []*graph.Entity

	for _, word := range words {
		if len(word) > 3 { // Skip short words
			matches, err := c.graph.SearchEntities(word)
			if err == nil {
				relevantEntities = append(relevantEntities, matches...)
			}
		}
	}

	// Build context for LLM
	var context strings.Builder
	context.WriteString("You are a knowledge graph assistant. ")
	context.WriteString("The user asked: \"" + query + "\"\n\n")

	if len(relevantEntities) > 0 {
		context.WriteString("Relevant entities from the knowledge graph:\n")
		for _, entity := range relevantEntities {
			context.WriteString(fmt.Sprintf("- %s (%s): %s\n",
				entity.Title, entity.Metadata.Type, entity.Content))
		}
	}

	context.WriteString("\nProvide a helpful response based on the available information.")

	// Get LLM response
	response, err := c.llm.Complete(ctx, context.String(), "")
	if err != nil {
		return fmt.Errorf("LLM query failed: %w", err)
	}

	fmt.Printf("\n%s\n\n", response)
	return nil
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
	
	return nil
}

// isSourceProcessed checks if a URL has already been processed
func (c *CLI) isSourceProcessed(url string) bool {
	// Check if source file exists
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
				// Could read file and check URL in metadata for exact match
				// For now, domain match is sufficient
				return true
			}
		}
	}
	
	return false
}
