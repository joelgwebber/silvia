# Silvia Technical Architecture

## Overview

Silvia uses a clean layered architecture with clear separation of concerns. All business logic is consolidated in an operations layer that can be accessed through multiple interfaces (CLI, MCP, HTTP) without code duplication.

## Architecture Layers

```
┌────────────────────────────────────────────────┐
│              Interface Layer                   │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│   │   CLI    │  │   MCP    │  │   HTTP   │     │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘     │
│        └─────────────┼─────────────┘           │
│                      ▼                         │
├────────────────────────────────────────────────┤
│               Tool Layer                       │
│  ┌──────────────────────────────────────────┐  │
│  │         Unified Tool Registry            │  │
│  │  • Dynamic dispatch                      │  │
│  │  • LLM function calling integration      │  │
│  │  • JSON schema generation                │  │
│  └────────────────┬─────────────────────────┘  │
│                   ▼                            │
├────────────────────────────────────────────────┤
│            Operations Layer                    │
│  ┌──────────────────────────────────────────┐  │
│  │      Typed Business Operations           │  │
│  │                                          │  │
│  │  EntityOps    QueueOps     SourceOps     │  │
│  │  SearchOps    LLMOps                     │  │
│  └────────────────┬─────────────────────────┘  │
│                   ▼                            │
├────────────────────────────────────────────────┤
│             Core Graph Layer                   │
│  ┌──────────────────────────────────────────┐  │
│  │       Basic Data & Storage               │  │
│  │  • Entity types & structures             │  │
│  │  • Markdown I/O with frontmatter         │  │
│  │  • File system operations                │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

## Component Details

### Interface Layer

#### CLI (`/internal/cli/`)
- Interactive command-line interface
- Natural language processing via LLM
- Command registry for structured commands
- Terminal output formatting

#### MCP Server (`/internal/mcp/`)
- Model Context Protocol server for AI assistants
- Exposes tools via stdio transport
- Compatible with Claude Desktop and other MCP clients

#### HTTP Server (`/internal/server/`)
- RESTful API for browser extension
- CORS support for web clients
- Authentication via bearer tokens

### Tool Layer (`/internal/tools/`)

The tool layer provides dynamic dispatch and LLM integration:

- **Registry**: Central registration of all available tools
- **Manager**: Orchestrates tool execution and chaining
- **Tool Wrappers**: Thin adapters around operations
- **Schema Generation**: Automatic JSON schemas for LLM function calling

Key features:
- Type-safe execution with `map[string]any` arguments
- Automatic parameter validation
- Result formatting and error handling
- Tool introspection and help generation

### Operations Layer (`/internal/operations/`)

All business logic is consolidated here:

#### EntityOps
- Create, read, update, delete entities
- Merge duplicate entities
- Rename entities with reference updates
- Refine entity content with LLM

#### QueueOps
- Priority queue management
- Add/remove/process queue items
- Queue persistence and status

#### SourceOps
- Ingest sources (web, Bluesky, etc.)
- Extract entities and relationships
- Archive original content
- Track processed sources

#### SearchOps
- Full-text entity search
- Related entity queries
- Entity type filtering
- Similarity suggestions

#### LLMOps
- LLM function calling
- Entity extraction from text
- Content refinement
- Merge assistance

### Core Graph Layer (`/internal/graph/`)

Foundation data structures and storage:

- **Entity**: Core entity type with metadata
- **Manager**: Basic CRUD operations
- **Markdown**: File I/O with YAML frontmatter
- **Cache**: In-memory entity cache

## Key Design Patterns

### 1. Operations Pattern
Each operation exposes both typed and dynamic interfaces:

```go
// Typed interface for Go code
func (ops *EntityOps) MergeEntities(ctx context.Context, id1, id2 string) (*MergeResult, error)

// Tool wrapper for dynamic dispatch
func (t *MergeEntityTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error)
```

### 2. Tool Registration
Tools self-register with the registry:

```go
func (m *Manager) registerAllTools() {
    m.registry.Register(NewMergeEntitiesTool(m.ops.Entity))
    m.registry.Register(NewSearchEntitiesOpsTool(m.ops.Search))
    // ... more tools
}
```

### 3. LLM Function Calling
Native OpenRouter Tools API integration:

```go
func (l *LLMOps) ExecuteFunctionCall(ctx context.Context, userInput string, toolSchemas []map[string]any) (*FunctionCallResponse, error)
```

## Data Flow Examples

### Entity Creation Flow
1. User issues command in CLI
2. CLI parses command or uses LLM for natural language
3. Tool layer dispatches to CreateEntityTool
4. EntityOps.CreateEntity validates and creates entity
5. Graph.Manager writes markdown file
6. Result propagates back through layers

### Source Ingestion Flow
1. HTTP server receives browser extension request
2. Server calls SourceOps.IngestSource
3. Source fetcher retrieves content
4. LLM extracts entities and relationships
5. Entities created/updated in graph
6. Links added to queue for exploration

### Search Flow
1. MCP client requests entity search
2. MCP server translates to tool call
3. SearchOps performs full-text search
4. Results formatted and returned

## File Organization

```
internal/
├── operations/       # Business logic
│   ├── entity_ops.go
│   ├── queue_ops.go
│   ├── source_ops.go
│   ├── search_ops.go
│   ├── llm_ops.go
│   └── types.go
├── tools/           # Tool layer
│   ├── manager.go
│   ├── registry.go
│   ├── entity_tools.go
│   ├── queue_tools.go
│   └── ...
├── cli/            # CLI interface
├── mcp/            # MCP server
├── server/         # HTTP server
└── graph/          # Core data layer
```

## Extension Points

### Adding New Operations
1. Create new ops file in `/internal/operations/`
2. Add to Operations struct in `types.go`
3. Initialize in `operations.New()`

### Adding New Tools
1. Create tool wrapper in `/internal/tools/`
2. Register in `Manager.registerAllTools()`
3. Tool automatically available to all interfaces

### Adding New Interfaces
1. Create new package for interface
2. Use operations layer for business logic
3. No duplication of functionality needed

## Performance Considerations

- **Caching**: Entities cached in memory on first read
- **Lazy Loading**: Graph loads entities on demand
- **Tool Manager**: Single instance per interface
- **LLM Calls**: Asynchronous with timeout controls
- **File I/O**: Buffered reads/writes for large files

## Security

- **Authentication**: Bearer tokens for HTTP API
- **Input Validation**: All tool inputs validated
- **Path Traversal**: Prevented in file operations
- **LLM Safety**: Structured output with JSON schemas
- **CORS**: Configurable for browser extension

## Testing Strategy

- **Unit Tests**: Operations layer functions
- **Integration Tests**: Tool execution chains
- **Interface Tests**: CLI commands, HTTP endpoints
- **Mock LLM**: Test fixtures for LLM responses

## Future Enhancements

- **Streaming**: Real-time updates for long operations
- **Websockets**: Push notifications for queue processing
- **GraphQL**: Alternative query interface
- **Plugins**: Dynamic tool loading
- **Distributed**: Multi-node graph synchronization
