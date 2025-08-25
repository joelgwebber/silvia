package cli

import (
	"context"
	"errors"
	"fmt"

	"silvia/internal/graph"
	"silvia/internal/sources"
)

// ingestSource processes a source
func (c *CLI) ingestSource(ctx context.Context, url string) error {
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
				graphEntity.Content = entity.Description
				graphEntity.Metadata.Aliases = entity.Aliases
				graphEntity.AddSource(url)

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
					existing.AddSource(url)
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
			// TODO: Create relationships in graph
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