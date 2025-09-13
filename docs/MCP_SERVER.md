# Silvia MCP Server

The Silvia CLI includes an MCP (Model Context Protocol) server mode that allows AI assistants like Claude to interact with the CLI programmatically. This enables AI assistants to operate the Silvia terminal UI as fully and easily as a human would.

## Features

The MCP server provides tools for:
- Creating and managing CLI sessions
- Sending text input and special keys
- Observing terminal state (with automatic mode detection)
- Waiting for specific output
- Full support for both append mode (line-by-line output) and interactive TUI modes

## Automatic Mode Detection

The server automatically detects when the CLI switches between:
- **Append mode**: Normal line-by-line output
- **Interactive mode**: TUI with cursor movement, screen clearing, etc.

Mode detection uses:
1. **OSC sequences**: Custom terminal sequences emitted by Silvia's interactive components
2. **Pattern detection**: Automatic detection of escape codes indicating interactive UI

## Setup for Claude Desktop

1. Build the Silvia binary:
   ```bash
   make build
   ```

2. Add to Claude Desktop's MCP configuration:
   - On macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - On Windows: `%APPDATA%\Claude\claude_desktop_config.json`

3. Add the Silvia server configuration:
   ```json
   {
     "mcpServers": {
       "silvia": {
         "command": "/path/to/silvia/bin/silvia",
         "args": ["-mcp"]
       }
     }
   }
   ```

4. Restart Claude Desktop

## Available Tools

### cli_create_session
Creates a new CLI session and returns the initial state.

### cli_send_input
Sends text input to the CLI (equivalent to typing and pressing Enter).

Parameters:
- `session_id`: Session ID from cli_create_session
- `input`: Text to send
- `wait_ms`: Optional wait time in milliseconds (default: 500)

### cli_send_key
Sends a special key to the CLI.

Parameters:
- `session_id`: Session ID
- `key`: Key to send (SPACE, ENTER, TAB, ESCAPE, UP, DOWN, LEFT, RIGHT, BACKSPACE, DELETE)
- `wait_ms`: Optional wait time in milliseconds (default: 100)

### cli_observe
Gets the current state without sending input.

Parameters:
- `session_id`: Session ID

### cli_wait_for
Waits for specific text to appear in the output.

Parameters:
- `session_id`: Session ID
- `text`: Text to wait for
- `timeout_ms`: Maximum wait time in milliseconds (default: 5000)

### cli_close_session
Closes a CLI session.

Parameters:
- `session_id`: Session ID to close

## Response Format

Responses vary based on the detected mode:

### Append Mode Response
```json
{
  "session_id": "uuid",
  "mode": "append",
  "append": {
    "lines": ["line1", "line2"],
    "raw": "raw output with escape codes",
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

### Interactive Mode Response
```json
{
  "session_id": "uuid",
  "mode": "interactive",
  "snapshot": {
    "mode": "interactive",
    "screen": ["rendered line 1", "rendered line 2"],
    "cursor": {"row": 5, "col": 10},
    "raw": "raw output with escape codes",
    "context": {"mode": "queue_explorer"},
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

### Mode Transition
When the CLI transitions between modes, a transition field is included:
```json
{
  "transition": {
    "from": "append",
    "to": "interactive",
    "trigger": "/explore queue"
  }
}
```

## Example Usage

Here's how an AI assistant might interact with Silvia:

1. Create a session:
   ```
   Tool: cli_create_session
   ```

2. Search for an entity:
   ```
   Tool: cli_send_input
   Args: {"session_id": "...", "input": "/search douglas wilson"}
   ```

3. Navigate interactive queue explorer:
   ```
   Tool: cli_send_input
   Args: {"session_id": "...", "input": "/explore queue"}
   
   Tool: cli_send_key
   Args: {"session_id": "...", "key": "DOWN"}
   
   Tool: cli_send_key
   Args: {"session_id": "...", "key": "SPACE"}
   ```

4. Exit interactive mode:
   ```
   Tool: cli_send_key
   Args: {"session_id": "...", "key": "ESCAPE"}
   ```

## Testing

Test the MCP server manually:
```bash
# This will fail (requires stdio connection, not terminal)
./bin/silvia -mcp

# This works (with null input)
./bin/silvia -mcp < /dev/null
```

## Development

The MCP server implementation is in `internal/mcp/`:
- `server.go`: Core session management and terminal emulation
- `mcp.go`: MCP server setup and initialization
- `tools.go`: Tool definitions and handlers
- `json.go`: JSON conversion utilities

The terminal utilities and OSC support are in `internal/term/`:
- `osc.go`: OSC sequence generation for mode signaling