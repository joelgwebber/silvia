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

### Code Organization
```
cmd/silvia/main.go        # Entry point - interactive CLI
internal/
├── graph/                # Core graph logic
│   ├── types.go          # Entity/Relationship types
│   ├── entity.go         # Entity operations
│   ├── markdown.go       # Markdown I/O with frontmatter
│   ├── manager.go        # Graph CRUD operations
├── cli/                  # Interactive interface
│   ├── cli.go            # Main CLI loop and command parsing
│   ├── queue.go          # Priority queue for sources
│   └── queue_commands.go # Queue-related commands
├── bsky/                 # Bluesky client
│   └── client.go         # Thread fetching
└── llm/                  # LLM integration
    └── client.go         # OpenRouter client for entity extraction
```

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
- `/ingest <url>` - Fetch and analyze source
- `/queue` / `/explore queue` - Manage pending sources
- `/rebuild-refs` - Rebuild all back-references in the graph
- `/clear` - Clear screen
- `/help` - Show available commands

