# Silvia Browser Extension

This Chrome/Firefox extension allows you to capture web content directly into your Silvia knowledge graph.

## Features

- **One-click capture**: Capture entire web pages with full HTML preservation
- **Smart link extraction**: Captures all links with surrounding context
- **Selection mode**: Capture only selected text when needed
- **Authentication support**: Works with paywalled content you're logged into
- **Platform detection**: Special handling for Bluesky, Twitter/X, and other platforms
- **Connection status**: Real-time connection monitoring to your local Silvia instance

## Installation

### Chrome (Development Mode)

1. Start Silvia with the extension server:
   ```bash
   ./bin/silvia
   # Server starts on port 8765 by default
   ```

2. Open Chrome and navigate to `chrome://extensions/`

3. Enable "Developer mode" (toggle in top right)

4. Click "Load unpacked"

5. Select the `extension` directory from your Silvia installation

6. The extension icon should appear in your toolbar

### Firefox (Development Mode)

1. Start Silvia as above

2. Open Firefox and navigate to `about:debugging`

3. Click "This Firefox"

4. Click "Load Temporary Add-on"

5. Select the `manifest.json` file from the `extension` directory

## Usage

1. **Ensure Silvia is running** with the extension server enabled (default)

2. **Click the extension icon** in your browser toolbar

3. **Verify connection** - you should see "Connected to Silvia"

4. **Navigate to content** you want to capture

5. **Click "Capture to Silvia"** to ingest the page

### Capture Options

- **Capture links with context**: Extracts all links with surrounding text for better context
- **Capture selection only**: Only captures selected text (useful for long articles)

### Connection Settings

- **Server URL**: Default is `http://localhost:8765`
- **Auth Token**: Optional, set with `--token` flag when starting Silvia

## How It Works

1. **Content Extraction**: The extension extracts:
   - Full HTML content
   - Structured metadata (author, date, publication)
   - All links with context
   - Selected text (if in selection mode)

2. **Smart Routing**: Silvia automatically detects content type:
   - Bluesky posts → Uses Bluesky API when possible
   - Regular web pages → Processes HTML with link preservation
   - Future: PDF handling, other platforms

3. **Queue Integration**: Extracted links are automatically added to Silvia's processing queue

## Security Notes

- Extension only connects to localhost by default
- No data is sent to external servers
- Optional token authentication for additional security
- All processing happens locally on your machine

## Troubleshooting

### "Not connected" error
- Ensure Silvia is running: `./bin/silvia`
- Check the port matches (default 8765)
- Try disabling with `--no-server` flag and re-enabling

### "Already processing" message
- Silvia is busy with another ingestion
- Wait a moment and try again

### No content captured
- Some sites may block content extraction
- Try using selection mode for specific text
- Check browser console for errors

## Development

To modify the extension:

1. Edit files in the `extension/` directory
2. Click "Reload" in Chrome extensions page
3. Test your changes

Icons can be customized by replacing:
- `icon-16.png` (16x16 toolbar icon)
- `icon-48.png` (48x48 medium icon)  
- `icon-128.png` (128x128 large icon)