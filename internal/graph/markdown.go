package graph

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// Regex to match wiki-style links [[target|label]] or [[target]]
	wikiLinkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	// Regex to extract frontmatter
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)`)
)

// LoadEntityFromFile reads an entity from a markdown file
func LoadEntityFromFile(filePath string) (*Entity, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseEntityMarkdown(string(content))
}

// ParseEntityMarkdown parses markdown content into an Entity
func ParseEntityMarkdown(content string) (*Entity, error) {
	matches := frontmatterRegex.FindStringSubmatch(content)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid markdown format: missing frontmatter")
	}

	frontmatter := matches[1]
	body := matches[2]

	// Parse frontmatter
	var metadata Metadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	entity := &Entity{
		Metadata: metadata,
	}

	// Parse body sections
	sections := splitMarkdownSections(body)
	
	// Extract title (first heading)
	if title, ok := sections["title"]; ok {
		entity.Title = strings.TrimPrefix(title, "# ")
	}

	// Extract main content
	if content, ok := sections["content"]; ok {
		entity.Content = content
	}

	// Parse relationships
	if relSection, ok := sections["relationships"]; ok {
		entity.Relationships = parseRelationships(relSection)
	}

	// Parse back-references
	if backRefSection, ok := sections["back-references"]; ok {
		entity.BackRefs = parseBackReferences(backRefSection)
	}

	return entity, nil
}

// SaveEntityToFile writes an entity to a markdown file
func SaveEntityToFile(entity *Entity, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	content := FormatEntityMarkdown(entity)
	
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// FormatEntityMarkdown formats an entity as markdown with frontmatter
func FormatEntityMarkdown(entity *Entity) string {
	var buf bytes.Buffer

	// Write frontmatter
	buf.WriteString("---\n")
	frontmatter, _ := yaml.Marshal(entity.Metadata)
	buf.Write(frontmatter)
	buf.WriteString("---\n\n")

	// Write title
	buf.WriteString(fmt.Sprintf("# %s\n\n", entity.Title))

	// Write content
	if entity.Content != "" {
		buf.WriteString(entity.Content)
		buf.WriteString("\n\n")
	}

	// Write relationships
	if len(entity.Relationships) > 0 {
		buf.WriteString("## Relationships\n\n")
		
		// Group relationships by type
		relsByType := make(map[string][]Relationship)
		for _, rel := range entity.Relationships {
			relsByType[rel.Type] = append(relsByType[rel.Type], rel)
		}

		for relType, rels := range relsByType {
			buf.WriteString(fmt.Sprintf("### %s\n", formatRelationType(relType)))
			for _, rel := range rels {
				buf.WriteString(fmt.Sprintf("- [[%s]]", rel.Target))
				if rel.Note != "" {
					buf.WriteString(fmt.Sprintf(" - %s", rel.Note))
				}
				if rel.Date != nil {
					buf.WriteString(fmt.Sprintf(" (%s)", rel.Date.Format("January 2006")))
				}
				buf.WriteString("\n")
			}
			buf.WriteString("\n")
		}
	}

	// Write back-references
	if len(entity.BackRefs) > 0 {
		buf.WriteString("## Back-references\n")
		buf.WriteString("<!-- Auto-maintained by the system -->\n")
		for _, backRef := range entity.BackRefs {
			buf.WriteString(fmt.Sprintf("- [[%s]] - %s", backRef.Source, backRef.Type))
			if backRef.Note != "" {
				buf.WriteString(fmt.Sprintf(" - %s", backRef.Note))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// splitMarkdownSections splits markdown content into named sections
func splitMarkdownSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	currentSection := "content"
	currentContent := []string{}
	
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			// Main title
			sections["title"] = line
			currentSection = "content"
			currentContent = []string{}
		} else if strings.HasPrefix(line, "## ") {
			// Save previous section
			if len(currentContent) > 0 {
				sections[currentSection] = strings.Join(currentContent, "\n")
			}
			
			// Start new section
			sectionName := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "##")))
			sectionName = strings.ReplaceAll(sectionName, " ", "-")
			currentSection = sectionName
			currentContent = []string{}
		} else {
			currentContent = append(currentContent, line)
		}
	}
	
	// Save last section
	if len(currentContent) > 0 {
		sections[currentSection] = strings.Join(currentContent, "\n")
	}
	
	return sections
}

// parseRelationships extracts relationships from markdown content
func parseRelationships(content string) []Relationship {
	var relationships []Relationship
	lines := strings.Split(content, "\n")
	
	currentType := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "### ") {
			// New relationship type
			currentType = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "###")))
			currentType = strings.ReplaceAll(currentType, " ", "_")
		} else if strings.HasPrefix(line, "- [[") {
			// Extract relationship
			matches := wikiLinkRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				rel := Relationship{
					Type:   currentType,
					Target: matches[1],
				}
				
				// Extract note (text after the link)
				afterLink := strings.TrimSpace(wikiLinkRegex.ReplaceAllString(line, ""))
				afterLink = strings.TrimPrefix(afterLink, "-")
				afterLink = strings.TrimSpace(afterLink)
				
				// Check for date in parentheses
				if dateMatch := regexp.MustCompile(`\((.*?)\)`).FindStringSubmatch(afterLink); len(dateMatch) > 1 {
					if t, err := time.Parse("January 2006", dateMatch[1]); err == nil {
						rel.Date = &t
					}
					// Remove date from note
					afterLink = regexp.MustCompile(`\(.*?\)`).ReplaceAllString(afterLink, "")
				}
				
				rel.Note = strings.TrimSpace(strings.TrimPrefix(afterLink, "-"))
				relationships = append(relationships, rel)
			}
		}
	}
	
	return relationships
}

// parseBackReferences extracts back references from markdown content
func parseBackReferences(content string) []BackReference {
	var backRefs []BackReference
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [[") {
			matches := wikiLinkRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				// Extract the text after the link
				afterLink := strings.TrimSpace(wikiLinkRegex.ReplaceAllString(line, ""))
				afterLink = strings.TrimPrefix(afterLink, "-")
				parts := strings.SplitN(afterLink, "-", 2)
				
				backRef := BackReference{
					Source: matches[1],
				}
				
				if len(parts) > 0 {
					backRef.Type = strings.TrimSpace(parts[0])
				}
				if len(parts) > 1 {
					backRef.Note = strings.TrimSpace(parts[1])
				}
				
				backRefs = append(backRefs, backRef)
			}
		}
	}
	
	return backRefs
}

// formatRelationType converts a relationship type to title case
func formatRelationType(relType string) string {
	words := strings.Split(relType, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// ExtractWikiLinks extracts all wiki-style links from content
func ExtractWikiLinks(content string) []string {
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	return links
}