# Silvia - Knowledge Graph System Design

## Overview

Silvia is a knowledge graph system for analyzing sources (Bluesky posts, web articles, PDFs) to extract and track entities (people, organizations, concepts) and their relationships. The system uses a file-based approach with markdown storage and provides an interactive CLI for exploration.

## Core Concepts

### Entities
Nodes in the graph representing distinct objects of interest:
- **Person**: Individuals with influence or relevance
- **Organization**: Companies, foundations, churches, groups  
- **Concept**: Ideas, movements, doctrines
- **Work**: Books, articles, papers
- **Event**: Conferences, meetings, occurrences

### Relationships
Directed edges between entities with:
- Type (founded, authored, recommended, attended, etc.)
- Temporal attributes (when the relationship occurred)
- Source citations (where this information was discovered)
- Notes for additional context

### Sources
Original content preserved for reference:
- Archived in markdown format
- Metadata including URL, retrieval date, author
- Associated media (images, PDFs)
- Link to entities extracted from them

## Architecture

### Storage Layer
File-based storage using markdown with YAML frontmatter:
```
data/
├── graph/           # Entity storage
│   ├── people/
│   ├── organizations/
│   ├── concepts/
│   ├── works/
│   └── events/
├── sources/         # Archived source material
│   ├── bsky/
│   ├── web/
│   └── pdfs/
└── .silvia/         # System data
    └── queue.json   # Exploration queue
```

### Entity Format
```markdown
---
id: people/douglas-wilson
type: person
aliases: ["Doug Wilson"]
created: 2024-01-15T10:30:00Z
updated: 2024-01-20T14:22:00Z
sources:
  - sources/bsky/thread-123
tags: ["christian-nationalism"]
---

# Douglas Wilson

Conservative pastor involved with National Conservatism.

## Relationships

### Founded
- [[organizations/churches/crec]] - CREC (1998)

## Back-references
<!-- Auto-maintained by the system -->
- [[concepts/christian-nationalism]] - Key proponent
```

### Key Design Decisions

1. **File-based Storage**: Each entity is a markdown file with frontmatter. This provides:
   - Human readability and editability
   - Version control compatibility
   - Simple backup and migration
   - No database dependencies

2. **Flexible Hierarchy**: Entity IDs include paths (e.g., `people/us-politics/douglas-wilson`) but the hierarchy is part of the identifier, not a rigid structure. This allows organic growth without code changes.

3. **Wiki-style Links**: Using `[[target]]` syntax for entity references enables:
   - Easy navigation
   - Automatic back-reference tracking
   - Familiar syntax for users

4. **Bidirectional References**: The system maintains both outgoing relationships and incoming back-references automatically, ensuring graph consistency.

5. **Priority Queue**: Source exploration uses a priority queue to manage the discovery process, allowing users to systematically work through linked sources.

## User Interface

### Interactive CLI
A chat-like interface similar to claude-code that supports both commands and natural language:

```
> show people/peter-thiel
[Displays entity details]

> who is connected to both Wilson and Thiel?
[Natural language query processed by LLM]

> ingest https://bsky.app/profile/...
[Fetches and analyzes source]
```

### Core Commands
- `ingest <url>` - Add and analyze a new source
- `show <entity-id>` - Display entity details
- `search <query>` - Search entities
- `related <entity-id>` - Show connected entities
- `queue` - Manage pending sources
- `create <type> <id>` - Create new entity
- `link <from> <type> <to>` - Add relationship

## Processing Pipeline

### Source Ingestion Flow
1. **Fetch** - Retrieve content from URL
2. **Archive** - Store original in markdown format
3. **Extract** - Use LLM to identify entities and relationships
4. **Resolve** - Match against existing entities or create new
5. **Update** - Modify graph with new information
6. **Queue** - Add linked sources for exploration

### Entity Resolution
- Fuzzy matching on names and aliases
- LLM-assisted disambiguation
- Manual merge capability for duplicates

### Incremental Updates
When new sources provide information about existing entities:
- Add new relationships
- Update source citations
- Maintain change history
- Refresh back-references

## Implementation Status

### ✅ Completed
- Core graph data structures
- Markdown I/O with frontmatter
- Graph manager for CRUD operations
- Interactive CLI with chat interface
- Priority queue for source management
- Back-reference maintenance
- Basic search and navigation

### 🚧 In Progress
- Bluesky thread fetching
- Web page scraping
- LLM entity extraction
- PDF text extraction

### 📋 Planned
- Advanced entity resolution
- Temporal queries
- Graph visualization export
- Pattern detection
- Bulk operations

## Usage Examples

### Creating Entities
```bash
> create person people/us-politics/pete-hegseth
Title: Pete Hegseth
Description: Secretary of Defense nominee, television host
```

### Exploring Relationships
```bash
> related people/douglas-wilson
3 related entities:
  📚 Southern Slavery As It Was (works/books/southern-slavery-as-it-was)
  🏢 CREC (organizations/churches/crec)
  📅 NatCon 2023 (events/natcon-2023)
```

### Processing Sources
```bash
> ingest https://bsky.app/profile/jennycohn.bsky.social/post/3lwyrvrmpck2a
📥 Fetching Bluesky thread...
Found 15 posts with 8 external links

Extracted Entities:
• Douglas Wilson (person) - exists, updating
• Pete Hegseth (person) - NEW
• Christian Nationalism (concept) - exists, updating

Found 5 linked sources. Add to queue? (select)
1. [HIGH] theocracywatch.org/overview
2. [MED] amazon.com/book-link
> 1
✅ Added 1 source to queue
```

## Configuration

### Environment Variables
- `BSKY_HANDLE` - Bluesky username
- `BSKY_PASSWORD` - Bluesky app password  
- `OPENROUTER_API_KEY` - API key for LLM features

### Data Directory
Default: `./data`
Override with `-data` flag

## Future Enhancements

1. **Source Processing**
   - Automatic summarization
   - OCR for images
   - Video transcript extraction

2. **Graph Analysis**
   - Influence clustering
   - Path finding between entities
   - Community detection

3. **Export Formats**
   - GraphML/GEXF for visualization tools
   - JSON-LD for semantic web
   - Static site generation

4. **Collaboration**
   - Multi-user support
   - Conflict resolution
   - Change attribution

## Principles

1. **Incremental Building**: The graph grows gradually through source analysis
2. **Human in the Loop**: Users guide exploration and validate extractions
3. **Transparency**: All sources and citations are preserved
4. **Flexibility**: Entities and relationships can be manually edited
5. **Interoperability**: Standard formats enable tool integration

