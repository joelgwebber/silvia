# MCP Server Integration

## Overview

Silvia includes a Model Context Protocol (MCP) server that exposes its knowledge graph functionality to AI assistants like Claude Desktop. The MCP server provides programmatic access to all Silvia operations through a standardized protocol.

## Setup

### For Claude Desktop

1. Install Silvia and ensure it's in your PATH
2. Add to Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "silvia": {
      "command": "silvia",
      "args": ["-mcp"],
      "env": {
        "OPENROUTER_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

3. Restart Claude Desktop to load the MCP server

## Available Tools

The MCP server exposes all Silvia operations as tools:

### Entity Operations
- `read_entity` - Read entity details
- `create_entity` - Create new entities
- `update_entity` - Modify existing entities
- `merge_entities` - Merge duplicate entities
- `rename_entity` - Rename with reference updates
- `delete_entity` - Remove entities
- `refine_entity` - LLM-assisted content improvement

### Search Operations
- `search_entities` - Full-text search
- `get_related_entities` - Find connected entities
- `get_entities_by_type` - List by type (person, org, etc.)
- `suggest_related` - Find similar entities

### Source Operations
- `ingest_source` - Process URLs and extract entities
- `extract_from_html` - Extract from HTML content

### Queue Operations
- `get_queue` - View pending sources
- `add_to_queue` - Add sources for processing
- `remove_from_queue` - Remove sources
- `process_next_queue_item` - Process highest priority
- `update_queue_priority` - Change priorities
- `clear_queue` - Remove all items

## Usage Examples

In Claude Desktop or any MCP-compatible client:

```
"Search for entities related to Douglas Wilson"
→ Uses search_entities tool

"Create a new person entity for John Smith"
→ Uses create_entity tool

"What sources are in the queue?"
→ Uses get_queue tool

"Merge the duplicate Peter Thiel entities"
→ Uses merge_entities tool
```

## Implementation

The MCP server:
- Runs via stdio transport (stdin/stdout)
- Uses the same operations layer as CLI and HTTP interfaces
- Provides automatic tool discovery and schema generation
- Handles all error cases gracefully
- Logs to stderr to avoid protocol interference

## Development

To test the MCP server directly:

```bash
# Run in MCP mode (requires stdio redirect)
silvia -mcp < /dev/null

# Test with an MCP client
npm install -g @modelcontextprotocol/inspector
mcp-inspector silvia -mcp
```

For debugging, logs are written to stderr and can be captured:

```bash
silvia -mcp 2> mcp.log
```

## Technical Details

See [architecture.md](./architecture.md) for details on how the MCP server integrates with Silvia's layered architecture.