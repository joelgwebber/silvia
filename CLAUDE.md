# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
- `make build` - Build the binary to bin/silvia
- `make run` or `go run cmd/silvia/main.go` - Run the application directly
- `./bin/silvia` - Run the built binary

### Development
- `make fmt` or `go fmt ./...` - Format all Go code
- `make vet` or `go vet ./...` - Run Go vet for static analysis
- `make test` or `go test ./...` - Run all tests
- `make tidy` or `go mod tidy` - Clean up module dependencies
- `make all` - Run fmt, vet, test, and build in sequence
- `make clean` - Remove built artifacts

### Testing Single Files
- `go test -v ./internal/graph` - Test specific package
- `go test -run TestName ./...` - Run specific test by name

## Architecture

Silvia is a knowledge graph system for analyzing sources and tracking entities/relationships. It uses file-based markdown storage with YAML frontmatter.

### Module Structure
- **Module name**: `silvia` (not github.com/...)
- **Go version**: 1.24+
- **Main dependencies**:
  - `github.com/bluesky-social/indigo` - Bluesky integration
  - `github.com/revrost/go-openrouter` - LLM client
  - `github.com/metoro-io/mcp-golang` - MCP protocol support

### Code Organization
```
cmd/silvia/main.go        # Entry point - interactive CLI
internal/
├── operations/           # Business logic layer (single source of truth)
│   ├── types.go          # Shared operation types
│   ├── entity_ops.go     # Entity CRUD and complex operations
│   ├── queue_ops.go      # Queue management operations
│   ├── source_ops.go     # Source ingestion operations
│   ├── search_ops.go     # Search and relationship queries
│   └── llm_ops.go        # LLM-assisted operations
├── graph/                # Core graph persistence
│   ├── types.go          # Entity/Relationship types
│   ├── entity.go         # Entity I/O operations
│   ├── markdown.go       # Markdown storage with frontmatter
│   └── manager.go        # Graph manager
├── tools/                # Dynamic tool dispatch
│   ├── manager_v2.go     # Tool manager using operations
│   ├── entity_tools.go   # Entity tool wrappers
│   └── queue_tools.go    # Queue tool wrappers
├── cli/                  # Interactive interface
│   ├── cli.go            # Main CLI using operations
│   ├── commands.go       # Command handlers
│   └── queue.go          # Queue management UI
├── mcp/                  # MCP server implementation
│   ├── server_v2.go      # MCP server using operations
│   └── tool_bridge.go    # Bridge operations to MCP tools
├── server/               # HTTP API
│   ├── server_v2.go      # REST API using operations
│   └── server.go         # Legacy extension handler
├── bsky/                 # Bluesky client
│   └── client.go         # Thread fetching
└── llm/                  # LLM integration
    └── client.go         # OpenRouter client
```

### Layered Architecture

The codebase follows a clean layered architecture:

1. **Operations Layer** (`internal/operations/`): Contains all business logic as typed Go functions. This is the single source of truth for all operations.

2. **Tool Layer** (`internal/tools/`): Wraps operations for dynamic dispatch, providing JSON schemas for LLM function calling and MCP protocol.

3. **Interface Layer**: Multiple interfaces all use the same operations:
   - **CLI** (`internal/cli/`): Interactive terminal interface
   - **MCP** (`internal/mcp/`): Model Context Protocol server for AI assistants
   - **HTTP** (`internal/server/`): REST API for browser extensions

4. **Persistence Layer** (`internal/graph/`): Handles file-based storage with markdown and YAML frontmatter.

This architecture ensures:
- No duplicate business logic
- Clean separation of concerns
- Easy testing and maintenance
- Consistent behavior across all interfaces

### Data Storage
```
data/
├── graph/              # Entity markdown files
│   ├── people/
│   ├── organizations/
│   ├── concepts/
│   ├── works/
│   └── events/
├── sources/            # Archived source material
└── .silvia/           # System data (queue.json)
```

### Entity Format
Entities are stored as markdown files with YAML frontmatter containing metadata. Relationships use wiki-style `[[target]]` links anywhere in the content. The system treats all links equally, regardless of where they appear in the document.

### Relationship System
- **Unified extraction**: All wiki-links `[[target]]` are treated as relationships
- **No special sections**: Any section can contain links (except Back-references)
- **Automatic categorization**: Links are categorized by their context:
  - Wiki-links in content → "mentioned_in"
  - Sources in frontmatter → "sourced_from"
- **Back-references**: Automatically maintained by the system
- **Cache invalidation**: Files modified externally are automatically reloaded

### Key Patterns
- All entity IDs include paths (e.g., `people/douglas-wilson`)
- File paths in data/ mirror entity IDs
- All wiki-links are tracked as relationships
- Back-references are auto-maintained and rebuilt with `/rebuild-refs`
- Sources are archived before processing
- External file changes are detected via modification time checks

## Environment Variables

Required for external integrations:
- `BSKY_HANDLE` - Bluesky username
- `BSKY_PASSWORD` - Bluesky app password
- `OPENROUTER_API_KEY` - API key for LLM features

## CLI Commands

The interactive CLI supports both structured commands and natural language:
- `/search <query>` - Search entities
- `/show <entity-id>` - Display entity details
- `/related <entity-id>` - Show connected entities
- `/create <type> <id>` - Create new entity
- `/link <from> <type> <to>` - Add relationship
- `/merge <id1> <id2>` - Merge entity2 into entity1, updating all references
- `/rename <old-id> <new-id>` - Rename entity and update all references
- `/ingest <url>` - Fetch and analyze source
- `/queue` / `/explore queue` - Manage pending sources
- `/rebuild-refs` - Rebuild all back-references in the graph
- `/clear` - Clear screen
- `/help` - Show available commands

