package cli

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"silvia/internal/graph"
	"silvia/internal/sources"
)

// ingestSource processes a source without force
func (c *CLI) ingestSource(ctx context.Context, url string) error {
	return c.ingestSourceWithForce(ctx, url, false)
}

// ingestSourceWithForce processes a source with optional force flag
func (c *CLI) ingestSourceWithForce(ctx context.Context, url string, force bool) error {
	// Check if already processed (unless force is true)
	if !force && c.isSourceProcessed(url) {
		fmt.Println(WarningStyle.Render("⚠️  Source already processed: ") + URLStyle.Render(url))
		fmt.Println(DimStyle.Render("Use /ingest <url> --force to re-process"))
		// Remove from queue if present
		c.queue.Remove(url)
		c.queue.SaveToFile()
		return nil
	}

	if force && c.isSourceProcessed(url) {
		fmt.Println(InfoStyle.Render("🔄 Force update: Re-processing ") + URLStyle.Render(url))
		// Remove from tracker to allow re-processing
		if c.tracker != nil {
			c.tracker.RemoveProcessed(url)
		}
	}

	fmt.Println(InfoStyle.Render("📥 Ingesting source: ") + URLStyle.Render(url))

	// Fetch the source
	source, err := c.sources.Fetch(ctx, url)
	if err != nil {
		// Check if it's an authentication error
		var fetchErr *sources.FetchError
		if errors.As(err, &fetchErr) && fetchErr.NeedsAuth {
			// Handle authentication required
			fmt.Println(WarningStyle.Render("⚠️  Authentication required"))
			fmt.Println()
			fmt.Println("Opening URL in browser...")

			// Open in browser
			if openErr := sources.OpenInBrowser(url); openErr != nil {
				fmt.Println(FormatError(fmt.Sprintf("Failed to open browser: %v", openErr)))
				fmt.Printf("\nPlease open manually: %s\n", URLStyle.Render(url))
			} else {
				fmt.Println(SuccessStyle.Render("✓ Browser opened"))
			}

			fmt.Println()
			fmt.Println(InfoStyle.Render("After the page loads:"))
			fmt.Println("  1. Log in if needed")
			fmt.Println("  2. Wait for article to fully load")
			fmt.Println("  3. Select all text (Cmd+A / Ctrl+A)")
			fmt.Println("  4. Copy to clipboard (Cmd+C / Ctrl+C)")
			fmt.Println("  5. Press Enter here to continue")
			fmt.Println()
			fmt.Print(PromptStyle.Render("Press Enter when ready: "))

			// Wait for user input
			c.readline.Readline()

			// Try to fetch from clipboard
			fmt.Println(InfoStyle.Render("📋 Reading from clipboard..."))

			webFetcher := sources.NewWebFetcher()
			source, err = webFetcher.FetchFromClipboard(url)
			if err != nil {
				return fmt.Errorf("failed to read clipboard: %w", err)
			}

			fmt.Println(FormatSuccess("Captured: " + source.Title))
		} else {
			return fmt.Errorf("failed to fetch source: %w", err)
		}
	} else {
		fmt.Println(FormatSuccess("Fetched: " + source.Title))
	}

	// Save the source content
	if err := c.saveSource(source); err != nil {
		fmt.Println(FormatWarning(fmt.Sprintf("Failed to save source: %v", err)))
	}

	// Extract entities and relationships
	fmt.Println(InfoStyle.Render("🔍 Extracting entities..."))
	extraction, err := c.extractor.Extract(ctx, source)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Create source summary entity if we have one
	var sourceSummaryID string
	if extraction.SourceSummary != nil {
		sourceSummaryID = c.createSourceSummary(source, extraction.SourceSummary, url)
	}

	// Process extracted entities
	if len(extraction.Entities) > 0 {
		fmt.Println(SubheaderStyle.Render(fmt.Sprintf("Found %d entities:", len(extraction.Entities))))
		for _, entity := range extraction.Entities {
			// Create or update entity in graph
			id := c.generateEntityID(entity.Name, entity.Type)

			// Check if entity exists
			if !c.graph.EntityExists(id) {
				// Create new entity
				graphEntity := graph.NewEntity(id, entity.Type)
				graphEntity.Title = entity.Name
				graphEntity.Content = entity.Content // Use rich content
				graphEntity.Metadata.Aliases = entity.Aliases
				// Reference source summary if available, otherwise raw URL
				if sourceSummaryID != "" {
					graphEntity.AddSource(sourceSummaryID) // No wiki-link format in YAML
				} else {
					graphEntity.AddSource(url)
				}

				if err := c.graph.SaveEntity(graphEntity); err != nil {
					fmt.Println(FormatWarning(fmt.Sprintf("Failed to save %s: %v", entity.Name, err)))
				} else {
					fmt.Printf("  %s %s %s %s\n",
						SuccessStyle.Render("✓ Created:"),
						getEntityIcon(entity.Type),
						HighlightStyle.Render(entity.Name),
						DimStyle.Render("("+id+")"))
				}
			} else {
				// Update existing entity with new source
				existing, err := c.graph.LoadEntity(id)
				if err == nil {
					// Reference source summary if available, otherwise raw URL
					if sourceSummaryID != "" {
						existing.AddSource(sourceSummaryID) // No wiki-link format in YAML
					} else {
						existing.AddSource(url)
					}
					if err := c.graph.SaveEntity(existing); err != nil {
						fmt.Println(FormatWarning(fmt.Sprintf("Failed to update %s: %v", entity.Name, err)))
					} else {
						fmt.Printf("  %s %s %s\n",
							SuccessStyle.Render("✓ Updated:"),
							getEntityIcon(entity.Type),
							HighlightStyle.Render(entity.Name))
					}
				}
			}
		}
	}

	// Process relationships
	if len(extraction.Relationships) > 0 {
		fmt.Println(SubheaderStyle.Render(fmt.Sprintf("Found %d relationships:", len(extraction.Relationships))))
		for _, rel := range extraction.Relationships {
			fmt.Printf("  %s %s %s %s %s %s\n",
				SuccessStyle.Render("•"),
				HighlightStyle.Render(rel.Source),
				DimStyle.Render("→"),
				InfoStyle.Render(rel.Type),
				DimStyle.Render("→"),
				HighlightStyle.Render(rel.Target))

			// Create the relationship in the graph
			// Note: This will be handled by entity content updates with wiki-links
			// The graph automatically extracts relationships from [[target]] links
			// So no explicit relationship creation is needed here
		}
	}

	// Automatically add relevant links to queue
	if len(extraction.Links) > 0 {
		added := 0
		highAdded := 0

		for _, link := range extraction.Links {
			// Skip if already in queue or processed
			if c.queue.Contains(link.URL) || c.isSourceProcessed(link.URL) {
				continue
			}

			// Skip low relevance links
			if link.Relevance == "low" {
				continue
			}

			// Build rich description
			desc := ""
			if link.Relevance == "high" {
				desc = "⭐ "
			}
			if link.Title != "" {
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
				highAdded++
			}

			if c.queue.Add(link.URL, priority, source.URL, desc) {
				added++
			}
		}

		if added > 0 {
			fmt.Printf("\n%s", InfoStyle.Render(fmt.Sprintf("Added %d links to queue", added)))
			if highAdded > 0 {
				fmt.Printf(" %s", SuccessStyle.Render(fmt.Sprintf("(%d high priority)", highAdded)))
			}
			fmt.Println()
			c.queue.SaveToFile()
		}
	}

	fmt.Println(FormatSuccess("Source ingestion complete"))
	return nil
}

// createSourceSummary creates a source summary entity in the graph
func (c *CLI) createSourceSummary(source *sources.Source, summary *sources.SourceSummary, url string) string {
	// Generate ID from URL
	u, err := neturl.Parse(url)
	if err != nil {
		return ""
	}

	// Create ID like "sources/domain-date"
	domain := strings.ReplaceAll(u.Hostname(), ".", "-")
	id := fmt.Sprintf("sources/%s-%s", domain, time.Now().Format("2006-01-02"))

	// Create entity
	entity := graph.NewEntity(id, graph.EntityWork)
	entity.Title = summary.Title

	// Build rich content
	var content strings.Builder

	// Metadata section
	if summary.Author != "" {
		content.WriteString(fmt.Sprintf("**Author**: %s\n", summary.Author))
	}
	if summary.Publication != "" {
		content.WriteString(fmt.Sprintf("**Publication**: %s\n", summary.Publication))
	}
	if summary.Date != "" {
		content.WriteString(fmt.Sprintf("**Date**: %s\n", summary.Date))
	}
	content.WriteString(fmt.Sprintf("**Source URL**: %s\n", url))
	content.WriteString(fmt.Sprintf("**Raw Source**: %s\n\n", c.getSourcePath(url)))

	// Key themes
	if len(summary.KeyThemes) > 0 {
		content.WriteString("## Key Themes\n\n")
		for _, theme := range summary.KeyThemes {
			content.WriteString(fmt.Sprintf("- %s\n", theme))
		}
		content.WriteString("\n")
	}

	// Analysis
	if summary.Analysis != "" {
		content.WriteString("## Analysis\n\n")
		content.WriteString(summary.Analysis)
		content.WriteString("\n\n")
	}

	// Key quotes
	if len(summary.KeyQuotes) > 0 {
		content.WriteString("## Key Quotes\n\n")
		for _, quote := range summary.KeyQuotes {
			content.WriteString(fmt.Sprintf("> %s\n\n", quote))
		}
	}

	// Related entities
	if len(summary.People) > 0 || len(summary.Organizations) > 0 || len(summary.Events) > 0 {
		content.WriteString("## Related Entities\n\n")
		if len(summary.People) > 0 {
			content.WriteString("### People\n")
			for _, personID := range summary.People {
				content.WriteString(fmt.Sprintf("- [[%s]]\n", personID))
			}
			content.WriteString("\n")
		}
		if len(summary.Organizations) > 0 {
			content.WriteString("### Organizations\n")
			for _, orgID := range summary.Organizations {
				content.WriteString(fmt.Sprintf("- [[%s]]\n", orgID))
			}
			content.WriteString("\n")
		}
		if len(summary.Events) > 0 {
			content.WriteString("### Events\n")
			for _, eventID := range summary.Events {
				content.WriteString(fmt.Sprintf("- [[%s]]\n", eventID))
			}
			content.WriteString("\n")
		}
	}

	entity.Content = content.String()
	entity.AddSource(url)

	// Save entity
	if err := c.graph.SaveEntity(entity); err != nil {
		fmt.Println(FormatWarning(fmt.Sprintf("Failed to save source summary: %v", err)))
		return ""
	}

	fmt.Printf("  %s Created source summary: %s %s\n",
		SuccessStyle.Render("📄"),
		HighlightStyle.Render(summary.Title),
		DimStyle.Render("("+id+")"))

	return id
}

// getSourcePath returns the file path where a source will be saved
func (c *CLI) getSourcePath(url string) string {
	u, err := neturl.Parse(url)
	if err != nil {
		return "data/sources/unknown"
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", u.Hostname(), timestamp)
	return fmt.Sprintf("data/sources/web/%s", filename)
}
