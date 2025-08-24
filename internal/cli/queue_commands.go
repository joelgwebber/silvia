package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"silvia/internal/graph"
	"silvia/internal/sources"
)

// showQueue displays the current source queue
func (c *CLI) showQueue() error {
	items := c.queue.GetAll()

	if len(items) == 0 {
		fmt.Println("Queue is empty.")
		return nil
	}

	fmt.Printf("\nExploration Queue (%d items):\n", len(items))
	for i, item := range items {
		fmt.Printf("%d. [%s] %s", i+1, item.Priority, item.URL)
		if item.FromSource != "" {
			fmt.Printf(" - from %s", item.FromSource)
		}
		if item.Description != "" {
			fmt.Printf("\n   %s", item.Description)
		}
		fmt.Println()
	}
	fmt.Println()

	return nil
}

// exploreQueue interactively processes sources in the queue
func (c *CLI) exploreQueue(ctx context.Context) error {
	for {
		item := c.queue.Peek()
		if item == nil {
			fmt.Println("Queue is empty.")
			return nil
		}

		fmt.Printf("\n📎 Next source: [%s] %s\n", item.Priority, item.URL)
		if item.Description != "" {
			fmt.Printf("   %s\n", item.Description)
		}

		c.readline.SetPrompt("Process this source? (y/n/skip/preview/stop): ")
		response, _ := c.readline.Readline()
		c.readline.SetPrompt("> ")
		response = strings.TrimSpace(strings.ToLower(response))

		switch response {
		case "y", "yes":
			// Remove from queue and process
			c.queue.PopItem()
			if err := c.ingestSource(ctx, item.URL); err != nil {
				fmt.Printf("Error ingesting source: %v\n", err)
				// Ask if we should continue
				c.readline.SetPrompt("Continue with next source? (y/n): ")
				cont, _ := c.readline.Readline()
				c.readline.SetPrompt("> ")
				if strings.TrimSpace(strings.ToLower(cont)) != "y" {
					return nil
				}
			}

		case "skip", "s":
			// Remove from queue without processing
			c.queue.PopItem()
			fmt.Println("Skipped.")

		case "preview", "p":
			// Preview the source without processing
			fmt.Println("Preview not yet implemented.")
			// TODO: Implement source preview

		case "stop", "n", "no":
			// Save queue and stop
			c.queue.SaveToFile()
			fmt.Println("Queue saved.")
			return nil

		default:
			fmt.Println("Invalid response. Please enter y/n/skip/preview/stop")
		}
	}
}

// addToQueue adds sources to the queue interactively
func (c *CLI) addToQueue(sources []string, descriptions []string) error {
	if len(sources) == 0 {
		return nil
	}

	fmt.Printf("\nFound %d linked sources. Add to queue? (all/select/none): ", len(sources))
	c.readline.SetPrompt("Choice: ")
	response, _ := c.readline.Readline()
	c.readline.SetPrompt("> ")
	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "all", "a":
		for i, url := range sources {
			desc := ""
			if i < len(descriptions) {
				desc = descriptions[i]
			}
			c.queue.Add(url, PriorityMedium, "", desc)
		}
		fmt.Printf("✅ Added %d sources to queue\n", len(sources))

	case "select", "s":
		// Show sources with numbers
		for i, url := range sources {
			fmt.Printf("%d. %s", i+1, url)
			if i < len(descriptions) && descriptions[i] != "" {
				fmt.Printf(" - %s", descriptions[i])
			}
			fmt.Println()
		}

		c.readline.SetPrompt("Enter numbers to queue (comma-separated) or 'skip': ")
		selection, _ := c.readline.Readline()
		c.readline.SetPrompt("> ")
		selection = strings.TrimSpace(selection)

		if selection != "skip" {
			nums := strings.Split(selection, ",")
			added := 0
			for _, numStr := range nums {
				numStr = strings.TrimSpace(numStr)
				if num, err := strconv.Atoi(numStr); err == nil && num > 0 && num <= len(sources) {
					desc := ""
					if num-1 < len(descriptions) {
						desc = descriptions[num-1]
					}
					if c.queue.Add(sources[num-1], PriorityMedium, "", desc) {
						added++
					}
				}
			}
			fmt.Printf("✅ Added %d sources to queue\n", added)
		}

	case "none", "n", "skip":
		fmt.Println("No sources added to queue.")

	default:
		fmt.Println("Invalid response. No sources added.")
	}

	// Save queue
	c.queue.SaveToFile()

	// Show current queue size
	fmt.Printf("Queue now has %d pending sources.\n", c.queue.Len())

	return nil
}

// ingestSource processes a source
func (c *CLI) ingestSource(ctx context.Context, url string) error {
	fmt.Printf("📥 Ingesting source: %s\n", url)

	// Fetch the source
	source, err := c.sources.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch source: %w", err)
	}

	fmt.Printf("✓ Fetched: %s\n", source.Title)

	// Save the source content
	if err := c.saveSource(source); err != nil {
		fmt.Printf("⚠️  Failed to save source: %v\n", err)
	}

	// Extract entities and relationships
	fmt.Println("🔍 Extracting entities...")
	extraction, err := c.extractor.Extract(ctx, source)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Process extracted entities
	if len(extraction.Entities) > 0 {
		fmt.Printf("Found %d entities:\n", len(extraction.Entities))
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
					fmt.Printf("  ⚠️  Failed to save %s: %v\n", entity.Name, err)
				} else {
					fmt.Printf("  ✓ Created: %s %s (%s)\n",
						getEntityIcon(entity.Type), entity.Name, id)
				}
			} else {
				// Update existing entity with new source
				existing, err := c.graph.LoadEntity(id)
				if err == nil {
					existing.AddSource(url)
					if err := c.graph.SaveEntity(existing); err != nil {
						fmt.Printf("  ⚠️  Failed to update %s: %v\n", entity.Name, err)
					} else {
						fmt.Printf("  ✓ Updated: %s %s\n",
							getEntityIcon(entity.Type), entity.Name)
					}
				}
			}
		}
	}

	// Process relationships
	if len(extraction.Relationships) > 0 {
		fmt.Printf("Found %d relationships:\n", len(extraction.Relationships))
		for _, rel := range extraction.Relationships {
			fmt.Printf("  • %s → %s → %s\n", rel.Source, rel.Type, rel.Target)
			// TODO: Create relationships in graph
		}
	}

	// Add linked sources to queue
	if len(extraction.LinkedSources) > 0 {
		fmt.Printf("\nFound %d linked sources\n", len(extraction.LinkedSources))

		// Filter out already processed or queued URLs
		var newSources []string
		var descriptions []string

		for _, link := range extraction.LinkedSources {
			// Make relative URLs absolute
			if strings.HasPrefix(link, "/") {
				link = sources.ExtractDomain(url) + link
			}

			// Skip if already in queue or processed
			if !c.queue.Contains(link) && !c.isSourceProcessed(link) {
				newSources = append(newSources, link)
				descriptions = append(descriptions, fmt.Sprintf("Linked from: %s", source.Title))
			}
		}

		if len(newSources) > 0 {
			return c.addToQueue(newSources, descriptions)
		}
	}

	fmt.Println("✅ Source ingestion complete")
	return nil
}

