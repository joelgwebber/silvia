package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
		
		fmt.Print("Process this source? (y/n/skip/preview/stop): ")
		response, _ := c.reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		switch response {
		case "y", "yes":
			// Remove from queue and process
			c.queue.PopItem()
			if err := c.ingestSource(ctx, item.URL); err != nil {
				fmt.Printf("Error ingesting source: %v\n", err)
				// Ask if we should continue
				fmt.Print("Continue with next source? (y/n): ")
				cont, _ := c.reader.ReadString('\n')
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
	response, _ := c.reader.ReadString('\n')
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
		
		fmt.Print("Enter numbers to queue (comma-separated) or 'skip': ")
		selection, _ := c.reader.ReadString('\n')
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

// ingestSource processes a source (placeholder for now)
func (c *CLI) ingestSource(ctx context.Context, url string) error {
	fmt.Printf("📥 Ingesting source: %s\n", url)
	
	// TODO: Implement actual source ingestion
	// This will involve:
	// 1. Fetching the source content
	// 2. Converting to markdown
	// 3. Extracting entities and relationships
	// 4. Updating the graph
	// 5. Finding linked sources
	
	fmt.Println("⚠️  Source ingestion not yet fully implemented")
	
	// For now, just simulate finding some linked sources
	if strings.Contains(url, "bsky.app") {
		fmt.Println("Found Bluesky thread...")
		// Simulate finding linked sources
		linkedSources := []string{
			"https://example.com/article1",
			"https://example.com/article2",
		}
		descriptions := []string{
			"Referenced article about topic X",
			"Related discussion thread",
		}
		
		return c.addToQueue(linkedSources, descriptions)
	}
	
	return nil
}