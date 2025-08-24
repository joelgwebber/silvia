# Silvia

A knowledge graph system for analyzing and connecting information from various sources.

## Quick Start

```bash
# Build and run the project
make build
./bin/silvia

# Or run directly
go run cmd/silvia/main.go
```

## Features

- 📊 Build knowledge graphs from multiple sources
- 🔍 Interactive CLI for exploration
- 📝 Markdown-based storage with frontmatter
- 🔗 Automatic relationship tracking
- 🤖 LLM-powered entity extraction (with OpenRouter)
- 📥 Source ingestion queue management

## Configuration

Set environment variables for external services:

```bash
export BSKY_HANDLE="your.handle"
export BSKY_PASSWORD="your-app-password"
export OPENROUTER_API_KEY="your-api-key"
```

## Basic Usage

```bash
# Search for entities
> search wilson

# Show entity details  
> show people/douglas-wilson

# View relationships
> related people/peter-thiel

# Create new entity
> create person people/new-person

# Add relationship
> link people/source-person founded organizations/new-org

# Process source queue
> explore queue
```

## Project Structure

```
silvia/
├── cmd/silvia/       # Main entry point
├── internal/
│   ├── bsky/         # Bluesky client
│   ├── llm/          # OpenRouter LLM client
│   ├── graph/        # Core graph data structures
│   ├── cli/          # Interactive CLI
│   └── sources/      # Source ingestion (WIP)
└── data/             # Knowledge graph storage
    ├── graph/        # Entity markdown files
    └── sources/      # Archived source material
```

## Design

See [DESIGN.md](DESIGN.md) for detailed architecture and design decisions.

## Development

```bash
# Format code
make fmt

# Run tests
make test

# Clean build artifacts
make clean

# Full build
make all
```

## Status

Core functionality is implemented. Source ingestion and LLM extraction are in progress.

## License

MIT
