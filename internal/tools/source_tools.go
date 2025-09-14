package tools

import (
	"context"

	"silvia/internal/operations"
)

// IngestSourceTool ingests a source URL
type IngestSourceTool struct {
	*BaseTool
	ops *operations.SourceOps
}

// NewIngestSourceTool creates a new ingest source tool
func NewIngestSourceTool(ops *operations.SourceOps) *IngestSourceTool {
	return &IngestSourceTool{
		BaseTool: NewBaseTool(
			"ingest_source",
			"Ingest a source URL, extracting entities and relationships",
			[]Parameter{
				{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "The URL to ingest",
				},
				{
					Name:        "force",
					Type:        "bool",
					Required:    false,
					Description: "Force re-ingestion even if already processed",
				},
			},
		),
		ops: ops,
	}
}

// Execute ingests a source
func (t *IngestSourceTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := GetString(args, "url", "")
	if url == "" {
		return ToolResult{Success: false, Error: "URL is required"},
			NewToolError(t.Name(), "missing URL", nil)
	}

	force := GetBool(args, "force", false)

	result, err := t.ops.IngestSource(ctx, url, force)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to ingest source", err)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"url":                url,
			"entities_extracted": len(result.ExtractedEntities),
			"links_extracted":    len(result.ExtractedLinks),
			"processing_time_ms": result.ProcessingTime.Milliseconds(),
		},
	}, nil
}

// ExtractFromHTMLTool extracts entities from HTML content
type ExtractFromHTMLTool struct {
	*BaseTool
	ops *operations.SourceOps
}

// NewExtractFromHTMLTool creates a new extract from HTML tool
func NewExtractFromHTMLTool(ops *operations.SourceOps) *ExtractFromHTMLTool {
	return &ExtractFromHTMLTool{
		BaseTool: NewBaseTool(
			"extract_from_html",
			"Extract entities from HTML content (used by browser extension)",
			[]Parameter{
				{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "The source URL",
				},
				{
					Name:        "html",
					Type:        "string",
					Required:    true,
					Description: "The HTML content",
				},
				{
					Name:        "title",
					Type:        "string",
					Required:    true,
					Description: "The page title",
				},
				{
					Name:        "metadata",
					Type:        "map",
					Required:    false,
					Description: "Additional metadata about the source",
				},
			},
		),
		ops: ops,
	}
}

// Execute extracts from HTML
func (t *ExtractFromHTMLTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	url := GetString(args, "url", "")
	html := GetString(args, "html", "")
	title := GetString(args, "title", "")

	if url == "" || html == "" || title == "" {
		return ToolResult{Success: false, Error: "url, html, and title are required"},
			NewToolError(t.Name(), "missing required fields", nil)
	}

	metadata := GetMap(args, "metadata", nil)
	metadataStr := make(map[string]string)
	if metadata != nil {
		for k, v := range metadata {
			if s, ok := v.(string); ok {
				metadataStr[k] = s
			}
		}
	}

	result, err := t.ops.ExtractFromHTML(ctx, url, html, title, metadataStr)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()},
			NewToolError(t.Name(), "failed to extract from HTML", err)
	}

	return ToolResult{
		Success: true,
		Data:    result,
		Meta: map[string]any{
			"url":                url,
			"entities_extracted": len(result.ExtractedEntities),
			"processing_time_ms": result.ProcessingTime.Milliseconds(),
		},
	}, nil
}
