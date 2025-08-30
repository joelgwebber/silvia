package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"silvia/internal/graph"
	"silvia/internal/sources"
)

// ExtensionLinkInfo represents a link from the extension
type ExtensionLinkInfo struct {
	URL     string `json:"url"`
	Text    string `json:"text"`
	Context string `json:"context"`
}

// IngestFromExtension processes content received from the browser extension
func (c *CLI) IngestFromExtension(ctx context.Context, url string, html string, title string, links []ExtensionLinkInfo, metadata map[string]string, force bool) error {
	// Check if we've already processed this URL (unless force is true)
	if !force && c.isSourceProcessed(url) {
		fmt.Printf("⚠️  Source already processed: %s\n", url)
		return fmt.Errorf("source already ingested: %s", url)
	}
	
	if force && c.isSourceProcessed(url) {
		fmt.Printf("🔄 Force update: Re-processing %s\n", url)
		// Remove from tracker to allow re-processing
		if c.tracker != nil {
			c.tracker.RemoveProcessed(url)
		}
	}
	
	// Convert extension links to source links
	var sourceLinks []string
	linkMap := make(map[string]string) // URL -> context mapping
	for _, link := range links {
		sourceLinks = append(sourceLinks, link.URL)
		if link.Context != "" {
			linkMap[link.URL] = link.Context
		}
	}
	
	if c.debug {
		fmt.Printf("[DEBUG] Received %d links from extension\n", len(links))
		if len(links) > 0 && len(links) <= 5 {
			for i, link := range links {
				fmt.Printf("[DEBUG] Link %d: %s\n", i+1, link.URL)
			}
		} else if len(links) > 5 {
			fmt.Printf("[DEBUG] First 5 links:\n")
			for i := 0; i < 5; i++ {
				fmt.Printf("[DEBUG] Link %d: %s\n", i+1, links[i].URL)
			}
		}
	}
	
	// Create a Source object from extension data
	source := &sources.Source{
		URL:        url,
		Title:      title,
		Content:    "", // Will be populated below
		RawContent: html,
		Links:      sourceLinks,
		Metadata:   metadata,
	}
	
	// Add extension-specific metadata
	if source.Metadata == nil {
		source.Metadata = make(map[string]string)
	}
	source.Metadata["fetched_at"] = time.Now().Format(time.RFC3339)
	source.Metadata["capture_method"] = "extension"
	source.Metadata["domain"] = sources.ExtractDomain(url)
	
	// Convert HTML to markdown if we have HTML content
	if html != "" {
		// Use the existing HTML to markdown converter
		webFetcher := sources.NewWebFetcher()
		markdown := webFetcher.ConvertHTMLToMarkdown(html)
		source.Content = markdown
		
		if c.debug {
			// Count markdown links to verify conversion
			linkCount := strings.Count(markdown, "](")
			fmt.Printf("[DEBUG] HTML conversion: Found %d markdown links in converted content\n", linkCount)
			
			// Show first few links for verification
			if linkCount > 0 {
				lines := strings.Split(markdown, "\n")
				shown := 0
				for _, line := range lines {
					if strings.Contains(line, "](") && shown < 3 {
						fmt.Printf("[DEBUG] Sample link in markdown: %s\n", strings.TrimSpace(line))
						shown++
					}
				}
			}
		}
		
		// Try to extract title from HTML if not provided
		if title == "" {
			source.Title = webFetcher.ExtractTitleFromHTML(html)
		}
	} else {
		// Fallback to text content if no HTML
		source.Content = metadata["text"]
	}
	
	// Check if this looks like a Bluesky post
	if strings.Contains(url, "bsky.app") || strings.Contains(url, "bsky.social") {
		// Route to Bluesky fetcher for special handling
		bskyFetcher := sources.NewBskyFetcher(nil)
		if bskyFetcher.CanHandle(url) {
			// Try to fetch via API for richer data
			if apiSource, err := bskyFetcher.Fetch(ctx, url); err == nil {
				// Merge extension data with API data
				source = apiSource
				// Preserve extension-captured links if API didn't get them
				if len(apiSource.Links) == 0 && len(sourceLinks) > 0 {
					source.Links = sourceLinks
				}
			}
		}
	}
	
	// Save the source content
	if err := c.saveSource(source); err != nil {
		// Log but don't fail the ingestion
		fmt.Printf("Warning: Failed to save source content: %v\n", err)
	}
	
	// Extract entities and relationships using LLM
	fmt.Println(InfoStyle.Render("🔍 Extracting entities from extension capture..."))
	extraction, err := c.extractor.Extract(ctx, source)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	
	if c.debug {
		fmt.Printf("[DEBUG] Extraction complete. Found %d entities, %d relationships, %d links\n", 
			len(extraction.Entities), len(extraction.Relationships), len(extraction.Links))
	}
	
	// Process the extraction (same as regular ingestion)
	return c.processExtraction(ctx, source, extraction, linkMap)
}

// processExtraction handles the entity and relationship creation from an extraction
func (c *CLI) processExtraction(ctx context.Context, source *sources.Source, extraction *sources.ExtractionResult, linkContextMap map[string]string) error {
	// Create source summary entity if we have one
	var sourceSummaryID string
	if extraction.SourceSummary != nil {
		sourceSummaryID = c.createSourceSummary(source, extraction.SourceSummary, source.URL)
	}
	
	// Process extracted entities
	if len(extraction.Entities) > 0 {
		fmt.Printf("Found %d entities\n", len(extraction.Entities))
		for _, entity := range extraction.Entities {
			// Create or update entity in graph
			id := c.generateEntityID(entity.Name, entity.Type)
			
			// Check if entity exists
			if !c.graph.EntityExists(id) {
				// Create new entity
				graphEntity := graph.NewEntity(id, entity.Type)
				graphEntity.Title = entity.Name
				graphEntity.Content = entity.Content
				graphEntity.Metadata.Aliases = entity.Aliases
				
				// Reference source summary if available, otherwise raw URL
				if sourceSummaryID != "" {
					graphEntity.AddSource(sourceSummaryID)
				} else {
					graphEntity.AddSource(source.URL)
				}
				
				if err := c.graph.SaveEntity(graphEntity); err != nil {
					fmt.Printf("Warning: Failed to save %s: %v\n", entity.Name, err)
				} else {
					fmt.Printf("  ✓ Created: %s (%s)\n", entity.Name, id)
				}
			} else {
				// Update existing entity with new source
				existing, err := c.graph.LoadEntity(id)
				if err == nil {
					if sourceSummaryID != "" {
						existing.AddSource(sourceSummaryID)
					} else {
						existing.AddSource(source.URL)
					}
					if err := c.graph.SaveEntity(existing); err != nil {
						fmt.Printf("Warning: Failed to update %s: %v\n", entity.Name, err)
					} else {
						fmt.Printf("  ✓ Updated: %s\n", entity.Name)
					}
				}
			}
		}
	}
	
	// Process relationships
	if len(extraction.Relationships) > 0 {
		fmt.Printf("Found %d relationships\n", len(extraction.Relationships))
		for _, rel := range extraction.Relationships {
			fmt.Printf("  • %s → %s → %s\n", rel.Source, rel.Type, rel.Target)
		}
	}
	
	// Add relevant links to queue with context
	if len(extraction.Links) > 0 {
		added := 0
		for _, link := range extraction.Links {
			// Skip if already in queue or processed
			if c.queue.Contains(link.URL) || c.isSourceProcessed(link.URL) {
				continue
			}
			
			// Skip low relevance links
			if link.Relevance == "low" {
				continue
			}
			
			// Build description with context if available
			desc := ""
			if link.Relevance == "high" {
				desc = "⭐ "
			}
			
			// Add context from extension if available
			if context, ok := linkContextMap[link.URL]; ok && context != "" {
				desc += fmt.Sprintf("[%s] %s", link.Category, context)
			} else if link.Title != "" {
				desc += link.Title
				if link.Description != "" {
					desc += " - " + link.Description
				}
			} else if link.Description != "" {
				desc += link.Description
			} else {
				desc += fmt.Sprintf("[%s] from %s", link.Category, source.Title)
			}
			
			// Add with appropriate priority
			priority := PriorityMedium
			if link.Relevance == "high" {
				priority = PriorityHigh
			}
			
			if c.queue.Add(link.URL, priority, source.URL, desc) {
				added++
			}
		}
		
		if added > 0 {
			fmt.Printf("Added %d links to queue\n", added)
			c.queue.SaveToFile()
		}
	}
	
	return nil
}