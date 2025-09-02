package cli

import (
	"context"
	"fmt"
	"strings"
)

// CommandHandler is a function that handles a command
type CommandHandler func(ctx context.Context, c *CLI, args []string) error

// Command represents a CLI command with all its metadata
type Command struct {
	Name        string         // Primary command name (e.g., "/help")
	Aliases     []string       // Alternative names (e.g., ["/h", "/?"])
	Description string         // Help text description
	Usage       string         // Usage pattern (e.g., "<entity-id>")
	Handler     CommandHandler // Function to execute the command
	SubCommands []string       // For auto-completion of sub-commands (e.g., entity types)
	Dynamic     bool           // Whether this command uses dynamic completion (entity IDs)
}

// CommandRegistry holds all command definitions
type CommandRegistry struct {
	commands map[string]*Command
	ordered  []*Command // Maintain order for help display
}

// NewCommandRegistry creates and initializes the command registry
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		commands: make(map[string]*Command),
		ordered:  []*Command{},
	}
	r.registerAllCommands()
	return r
}

// registerAllCommands defines all commands in one place
func (r *CommandRegistry) registerAllCommands() {
	// Define all commands with their metadata
	commands := []*Command{
		{
			Name:        "/help",
			Aliases:     []string{"/h", "/?"},
			Description: "Show this help message",
			Handler:     handleHelp,
		},
		{
			Name:        "/ingest",
			Aliases:     []string{},
			Description: "Ingest a source (--force to re-process)",
			Usage:       "<url> [--force]",
			Handler:     handleIngest,
		},
		{
			Name:        "/show",
			Aliases:     []string{"/view"},
			Description: "Display an entity (tab for autocomplete)",
			Usage:       "<entity-id>",
			Handler:     handleShow,
			Dynamic:     true,
		},
		{
			Name:        "/search",
			Aliases:     []string{"/find"},
			Description: "Search for entities",
			Usage:       "<query>",
			Handler:     handleSearch,
		},
		{
			Name:        "/queue",
			Aliases:     []string{"/q"},
			Description: "Manage pending sources",
			Handler:     handleQueue,
		},
		{
			Name:        "/explore",
			Aliases:     []string{},
			Description: "Explore queue interactively",
			Usage:       "queue",
			Handler:     handleExplore,
			SubCommands: []string{"queue"},
		},
		{
			Name:        "/related",
			Aliases:     []string{"/connections"},
			Description: "Show related entities (tab for autocomplete)",
			Usage:       "<entity-id>",
			Handler:     handleRelated,
			Dynamic:     true,
		},
		{
			Name:        "/create",
			Aliases:     []string{},
			Description: "Create new entity",
			Usage:       "<type> <id>",
			Handler:     handleCreate,
			SubCommands: []string{"person", "organization", "concept", "work", "event"},
		},
		{
			Name:        "/link",
			Aliases:     []string{"/connect"},
			Description: "Create relationship",
			Usage:       "<from> <type> <to>",
			Handler:     handleLink,
			Dynamic:     true,
		},
		{
			Name:        "/merge",
			Aliases:     []string{},
			Description: "Merge entity2 into entity1",
			Usage:       "<id1> <id2>",
			Handler:     handleMerge,
			Dynamic:     true,
		},
		{
			Name:        "/rename",
			Aliases:     []string{},
			Description: "Rename entity and update references",
			Usage:       "<old-id> <new-id>",
			Handler:     handleRename,
			Dynamic:     true,
		},
		{
			Name:        "/move",
			Aliases:     []string{},
			Description: "Move entity (allows type change)",
			Usage:       "<old-id> <new-id>",
			Handler:     handleMove,
			Dynamic:     true,
		},
		{
			Name:        "/rebuild-refs",
			Aliases:     []string{},
			Description: "Rebuild all back-references",
			Handler:     handleRebuildRefs,
		},
		{
			Name:        "/refine",
			Aliases:     []string{},
			Description: "Refine entity using sources and LLM",
			Usage:       "<entity-id> [guidance]",
			Handler:     handleRefine,
			Dynamic:     true,
		},
		{
			Name:        "/clear",
			Aliases:     []string{},
			Description: "Clear screen",
			Handler:     handleClear,
		},
		{
			Name:        "/exit",
			Aliases:     []string{"/quit", "/q"},
			Description: "Exit the program",
			Handler:     nil, // Special case, handled in main loop
		},
	}

	// Register all commands
	for _, cmd := range commands {
		r.register(cmd)
	}
}

// register adds a command to the registry
func (r *CommandRegistry) register(cmd *Command) {
	// Register primary name
	r.commands[cmd.Name] = cmd
	r.ordered = append(r.ordered, cmd)

	// Register aliases
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
	}
}

// Get retrieves a command by name (including aliases)
func (r *CommandRegistry) Get(name string) (*Command, bool) {
	cmd, ok := r.commands[strings.ToLower(name)]
	return cmd, ok
}

// GetAll returns all commands in order (excluding aliases)
func (r *CommandRegistry) GetAll() []*Command {
	return r.ordered
}

// Command handler functions
// These wrap the existing methods to match the CommandHandler signature

func handleHelp(ctx context.Context, c *CLI, args []string) error {
	c.showHelpFromRegistry()
	return nil
}

func handleIngest(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /ingest <url> [--force]")
	}
	force := false
	url := args[0]
	if len(args) > 1 && args[1] == "--force" {
		force = true
	}
	return c.ingestSourceWithForce(ctx, url, force)
}

func handleShow(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /show <entity-id>")
	}
	return c.showEntity(strings.Join(args, " "))
}

func handleSearch(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /search <query>")
	}
	return c.searchEntities(strings.Join(args, " "))
}

func handleQueue(ctx context.Context, c *CLI, args []string) error {
	return c.InteractiveQueueExplorer(ctx)
}

func handleExplore(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 || args[0] != "queue" {
		return fmt.Errorf("usage: /explore queue")
	}
	return c.InteractiveQueueExplorer(ctx)
}

func handleRelated(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /related <entity-id>")
	}
	return c.showRelated(strings.Join(args, " "))
}

func handleCreate(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /create <type> <id>")
	}
	return c.createEntity(args[0], strings.Join(args[1:], " "))
}

func handleLink(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: /link <source-id> <rel-type> <target-id>")
	}
	return c.createLink(args[0], args[1], args[2])
}

func handleMerge(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /merge <entity1-id> <entity2-id>")
	}
	return c.mergeEntities(ctx, args[0], args[1])
}

func handleRename(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /rename <old-id> <new-id>")
	}
	return c.renameEntity(args[0], args[1])
}

func handleMove(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /move <old-id> <new-id>")
	}
	return c.moveEntity(args[0], args[1])
}

func handleRebuildRefs(ctx context.Context, c *CLI, args []string) error {
	fmt.Println("Rebuilding all back-references in the graph...")
	if err := c.graph.RebuildAllBackReferences(); err != nil {
		return fmt.Errorf("failed to rebuild references: %w", err)
	}
	fmt.Println(SuccessStyle.Render("✓ Back-references rebuilt successfully"))
	return nil
}

func handleRefine(ctx context.Context, c *CLI, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /refine <entity-id> [guidance]")
	}
	entityID := args[0]
	guidance := ""
	if len(args) > 1 {
		guidance = strings.Join(args[1:], " ")
	}
	return c.refineEntity(ctx, entityID, guidance)
}

func handleClear(ctx context.Context, c *CLI, args []string) error {
	fmt.Print("\033[H\033[2J") // Clear screen
	return nil
}
