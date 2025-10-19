package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/sources"
)

// SourceOps handles source ingestion and processing
type SourceOps struct {
	graph     *graph.Manager
	llm       *llm.Client
	sources   *sources.Manager
	extractor *sources.Extractor
	dataDir   string
}

// NewSourceOps creates a new source operations handler
func NewSourceOps(graphManager *graph.Manager, llmClient *llm.Client, sourcesManager *sources.Manager, dataDir string) *SourceOps {
	return &SourceOps{
		graph:     graphManager,
		llm:       llmClient,
		sources:   sourcesManager,
		extractor: sources.NewExtractor(llmClient),
		dataDir:   dataDir,
	}
}

// IngestSource ingests a source URL, extracting entities and relationships
func (s *SourceOps) IngestSource(ctx context.Context, url string, force bool) (*IngestResult, error) {
	startTime := time.Now()

	// Check if already processed
	if !force && s.isSourceProcessed(url) {
		return nil, NewOperationError("ingest source", url, fmt.Errorf("source already processed"))
	}

	// Fetch the source
	source, err := s.sources.Fetch(ctx, url)
	if err != nil {
		return nil, NewOperationError("ingest source", url, fmt.Errorf("failed to fetch: %w", err))
	}

	// Archive the source
	archivedPath, err := s.archiveSource(source)
	if err != nil {
		// Non-fatal error, continue processing
		fmt.Printf("Warning: failed to archive source: %v\n", err)
	}

	// Extract entities and relationships
	extractResult, err := s.extractor.Extract(ctx, source)
	if err != nil {
		return nil, NewOperationError("ingest source", url, fmt.Errorf("extraction failed: %w", err))
	}

	// Process extraction results using shared logic
	extractedEntities, extractedLinks := s.processExtractionResult(extractResult, url)

	// Mark source as processed
	s.markSourceProcessed(url)

	return &IngestResult{
		SourceURL:         url,
		ArchivedPath:      archivedPath,
		ExtractedEntities: extractedEntities,
		ExtractedLinks:    extractedLinks,
		ProcessingTime:    time.Since(startTime),
	}, nil
}

// ExtractFromHTML extracts entities from HTML content (used by browser extension)
func (s *SourceOps) ExtractFromHTML(ctx context.Context, url, html, title string, metadata map[string]string) (*IngestResult, error) {
	// Convert HTML to markdown
	webFetcher := sources.NewWebFetcher()
	markdown := webFetcher.ConvertHTMLToMarkdown(html)

	// Create a Source object
	source := &sources.Source{
		URL:        url,
		Title:      title,
		Content:    markdown,
		RawContent: html,
		Metadata:   metadata,
	}

	// Add metadata
	if source.Metadata == nil {
		source.Metadata = make(map[string]string)
	}
	source.Metadata["fetched_at"] = time.Now().Format(time.RFC3339)
	source.Metadata["capture_method"] = "extension"

	// Archive the source
	archivedPath, err := s.archiveSource(source)
	if err != nil {
		fmt.Printf("Warning: failed to archive source: %v\n", err)
	}

	// Extract entities
	extractResult, err := s.extractor.Extract(ctx, source)
	if err != nil {
		return nil, NewOperationError("extract from HTML", url, err)
	}

	// Process extraction results using shared logic
	extractedEntities, extractedLinks := s.processExtractionResult(extractResult, url)

	// Mark source as processed
	s.markSourceProcessed(url)

	return &IngestResult{
		SourceURL:         url,
		ArchivedPath:      archivedPath,
		ExtractedEntities: extractedEntities,
		ExtractedLinks:    extractedLinks,
		ProcessingTime:    time.Since(time.Now()),
	}, nil
}

// processExtractionResult is the shared logic for processing extraction results
// Used by both IngestSource and ExtractFromHTML to ensure consistent behavior
func (s *SourceOps) processExtractionResult(extractResult *sources.ExtractionResult, sourceURL string) ([]ExtractedEntity, []ExtractedLink) {
	// Process extracted entities
	extractedEntities := []ExtractedEntity{}
	for _, extracted := range extractResult.Entities {
		// Generate entity ID
		entityID := s.generateEntityID(string(extracted.Type), extracted.Name)

		// Check if entity exists
		isNew := !s.graph.EntityExists(entityID)
		wasUpdated := false

		if isNew {
			// Create new entity
			entity := &graph.Entity{
				Title:   extracted.Name,
				Content: extracted.Description,
				Metadata: graph.Metadata{
					ID:      entityID,
					Type:    graph.EntityType(extracted.Type),
					Sources: []string{sourceURL},
					Created: time.Now(),
					Updated: time.Now(),
				},
			}

			if err := s.graph.SaveEntity(entity); err != nil {
				fmt.Printf("Warning: failed to save entity %s: %v\n", entityID, err)
				continue
			}
		} else {
			// Update existing entity with new source
			entity, err := s.graph.LoadEntity(entityID)
			if err != nil {
				fmt.Printf("Warning: failed to load entity %s: %v\n", entityID, err)
				continue
			}

			// Add source if not already present
			hasSource := slices.Contains(entity.Metadata.Sources, sourceURL)
			if !hasSource {
				entity.Metadata.Sources = append(entity.Metadata.Sources, sourceURL)
				entity.Metadata.Updated = time.Now()
				if err := s.graph.SaveEntity(entity); err != nil {
					fmt.Printf("Warning: failed to update entity %s: %v\n", entityID, err)
					continue
				}
				wasUpdated = true
			}
		}

		extractedEntities = append(extractedEntities, ExtractedEntity{
			ID:          entityID,
			Type:        string(extracted.Type),
			Name:        extracted.Name,
			Description: extracted.Description,
			Content:     extracted.Description,
			IsNew:       isNew,
			WasUpdated:  wasUpdated,
		})
	}

	// Process extracted links
	extractedLinks := []ExtractedLink{}
	for _, link := range extractResult.Links {
		extractedLinks = append(extractedLinks, ExtractedLink{
			URL:         link.URL,
			Title:       link.Title,
			Description: link.Description,
			Category:    link.Category,
			Relevance:   link.Relevance,
		})
	}

	return extractedEntities, extractedLinks
}

// archiveSource saves a source to the archive
func (s *SourceOps) archiveSource(source *sources.Source) (string, error) {
	// Generate archive path
	timestamp := time.Now().Format("20060102-150405")
	domain := sources.ExtractDomain(source.URL)
	filename := fmt.Sprintf("%s-%s.md", strings.ReplaceAll(domain, ".", "-"), timestamp)
	archivePath := filepath.Join(s.dataDir, "sources", "web", filename)

	// Ensure directory exists
	dir := filepath.Dir(archivePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Prepare content with metadata
	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("url: %s\n", source.URL))
	content.WriteString(fmt.Sprintf("title: %s\n", source.Title))
	content.WriteString(fmt.Sprintf("fetched_at: %s\n", time.Now().Format(time.RFC3339)))
	if source.Metadata != nil {
		for key, value := range source.Metadata {
			content.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}
	content.WriteString("---\n\n")
	content.WriteString(source.Content)

	// Write to file
	if err := os.WriteFile(archivePath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write archive file: %w", err)
	}

	return archivePath, nil
}

// generateEntityID generates a consistent ID for an entity
func (s *SourceOps) generateEntityID(entityType, name string) string {
	// Convert type to singular form
	typePrefix := entityType
	switch entityType {
	case "person":
		typePrefix = "people"
	case "organization":
		typePrefix = "organizations"
	case "concept":
		typePrefix = "concepts"
	case "work":
		typePrefix = "works"
	case "event":
		typePrefix = "events"
	}

	// Convert name to ID format (lowercase, replace spaces with hyphens)
	nameID := strings.ToLower(name)
	nameID = strings.ReplaceAll(nameID, " ", "-")
	nameID = strings.ReplaceAll(nameID, "'", "")
	nameID = strings.ReplaceAll(nameID, ".", "")
	nameID = strings.ReplaceAll(nameID, ",", "")

	return fmt.Sprintf("%s/%s", typePrefix, nameID)
}

// Source tracking (to avoid re-processing)

type sourceTracker struct {
	ProcessedURLs map[string]time.Time `json:"processed_urls"`
}

func (s *SourceOps) getTrackerPath() string {
	return filepath.Join(s.dataDir, ".silvia", "processed_sources.json")
}

func (s *SourceOps) loadTracker() (*sourceTracker, error) {
	trackerPath := s.getTrackerPath()
	data, err := os.ReadFile(trackerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &sourceTracker{ProcessedURLs: make(map[string]time.Time)}, nil
		}
		return nil, err
	}

	var tracker sourceTracker
	if err := json.Unmarshal(data, &tracker); err != nil {
		return nil, err
	}
	if tracker.ProcessedURLs == nil {
		tracker.ProcessedURLs = make(map[string]time.Time)
	}
	return &tracker, nil
}

func (s *SourceOps) saveTracker(tracker *sourceTracker) error {
	trackerPath := s.getTrackerPath()
	dir := filepath.Dir(trackerPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tracker, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(trackerPath, data, 0644)
}

func (s *SourceOps) isSourceProcessed(url string) bool {
	tracker, err := s.loadTracker()
	if err != nil {
		return false
	}
	_, exists := tracker.ProcessedURLs[url]
	return exists
}

func (s *SourceOps) markSourceProcessed(url string) {
	tracker, _ := s.loadTracker()
	if tracker == nil {
		tracker = &sourceTracker{ProcessedURLs: make(map[string]time.Time)}
	}
	tracker.ProcessedURLs[url] = time.Now()
	s.saveTracker(tracker)
}
