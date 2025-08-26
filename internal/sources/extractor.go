package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"silvia/internal/graph"
	"silvia/internal/llm"
)

// ExtractedEntity represents an entity found in text
type ExtractedEntity struct {
	Name        string
	Type        graph.EntityType
	Description string   // Brief one-line description
	Content     string   // Rich markdown content with sections
	Aliases     []string
	WikiLinks   []string // Related entities in [[type/id]] format
}

// ExtractedRelationship represents a relationship found in text
type ExtractedRelationship struct {
	Source string
	Target string
	Type   string
	Note   string
}

// ExtractedLink represents a link found in the content with context
type ExtractedLink struct {
	URL         string
	Title       string
	Description string
	Relevance   string // high, medium, low
	Category    string // reference, discussion, resource, navigation, advertisement
}

// ExtractionResult contains entities and relationships found in a source
type ExtractionResult struct {
	Entities      []ExtractedEntity
	Relationships []ExtractedRelationship
	LinkedSources []string // Simple list for backward compatibility
	Links         []ExtractedLink // Enhanced link information
	SourceSummary *SourceSummary // Structured summary of the source
}

// SourceSummary represents a structured summary of a source
type SourceSummary struct {
	Title         string
	Author        string
	Publication   string
	Date          string
	KeyThemes     []string
	KeyQuotes     []string
	Events        []string // Event IDs referenced
	People        []string // Person IDs referenced
	Organizations []string // Organization IDs referenced
	Analysis      string   // Analysis and significance
}

// Extractor uses LLM to extract entities from content
type Extractor struct {
	llm *llm.Client
}

// NewExtractor creates a new entity extractor
func NewExtractor(llmClient *llm.Client) *Extractor {
	if llmClient == nil {
		panic("LLM client is required for entity extraction")
	}
	return &Extractor{
		llm: llmClient,
	}
}

// GenerateSourceSummary creates a structured summary of a source
func (e *Extractor) GenerateSourceSummary(ctx context.Context, source *Source, extraction *ExtractionResult) (*SourceSummary, error) {
	systemPrompt := `You are creating a structured summary of a source document for a knowledge graph system.

Generate a comprehensive summary that includes:
1. Key themes and topics discussed
2. Important quotes that capture essential points
3. Analysis of the document's significance
4. Connections to entities mentioned

Output JSON with this structure:
{
  "title": "article title",
  "author": "author name if known",
  "publication": "publication name",
  "date": "publication date if known",
  "key_themes": ["theme 1", "theme 2"],
  "key_quotes": ["important quote 1", "important quote 2"],
  "analysis": "Multi-paragraph analysis of significance, context, and implications"
}`

	// Build entity context
	entityContext := ""
	if extraction != nil && len(extraction.Entities) > 0 {
		entityContext = "\n\nKey entities found:\n"
		for _, entity := range extraction.Entities {
			entityContext += fmt.Sprintf("- %s (%s): %s\n", entity.Name, entity.Type, entity.Description)
		}
	}

	userPrompt := fmt.Sprintf("Create a structured summary of this source:\n\nTitle: %s\nURL: %s\n\nContent:\n%s%s",
		source.Title, source.URL, source.Content, entityContext)

	// Limit content length
	if len(userPrompt) > 10000 {
		userPrompt = userPrompt[:10000] + "\n[content truncated]"
	}

	response, err := e.llm.CompleteWithSystem(ctx, systemPrompt, userPrompt, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Parse response
	var result struct {
		Title      string   `json:"title"`
		Author     string   `json:"author"`
		Publication string  `json:"publication"`
		Date       string   `json:"date"`
		KeyThemes  []string `json:"key_themes"`
		KeyQuotes  []string `json:"key_quotes"`
		Analysis   string   `json:"analysis"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Try to extract JSON from response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
				return nil, fmt.Errorf("failed to parse summary: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse summary: %w", err)
		}
	}

	// Build summary with entity references
	summary := &SourceSummary{
		Title:       result.Title,
		Author:      result.Author,
		Publication: result.Publication,
		Date:        result.Date,
		KeyThemes:   result.KeyThemes,
		KeyQuotes:   result.KeyQuotes,
		Analysis:    result.Analysis,
	}

	// Add entity references if available
	if extraction != nil {
		for _, entity := range extraction.Entities {
			id := e.generateEntityID(entity.Name, entity.Type)
			switch entity.Type {
			case graph.EntityPerson:
				summary.People = append(summary.People, id)
			case graph.EntityOrganization:
				summary.Organizations = append(summary.Organizations, id)
			case graph.EntityEvent:
				summary.Events = append(summary.Events, id)
			}
		}
	}

	return summary, nil
}

// generateEntityID creates a consistent ID for an entity
func (e *Extractor) generateEntityID(name string, entityType graph.EntityType) string {
	// Convert name to ID format
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "'", "")
	id = strings.ReplaceAll(id, "\"", "")
	id = strings.ReplaceAll(id, ".", "")
	id = strings.ReplaceAll(id, ",", "")
	id = strings.ReplaceAll(id, ":", "")
	id = strings.ReplaceAll(id, ";", "")
	id = strings.ReplaceAll(id, "!", "")
	id = strings.ReplaceAll(id, "?", "")
	id = strings.ReplaceAll(id, "(", "")
	id = strings.ReplaceAll(id, ")", "")
	id = strings.ReplaceAll(id, "[", "")
	id = strings.ReplaceAll(id, "]", "")
	id = strings.ReplaceAll(id, "{", "")
	id = strings.ReplaceAll(id, "}", "")
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, "\\", "-")

	// Add type prefix
	var prefix string
	switch entityType {
	case graph.EntityPerson:
		prefix = "people/"
	case graph.EntityOrganization:
		prefix = "organizations/"
	case graph.EntityConcept:
		prefix = "concepts/"
	case graph.EntityWork:
		prefix = "works/"
	case graph.EntityEvent:
		prefix = "events/"
	default:
		prefix = "misc/"
	}

	return prefix + id
}

// Extract analyzes content and extracts entities, relationships, and linked sources
func (e *Extractor) Extract(ctx context.Context, source *Source) (*ExtractionResult, error) {
	// First, clean and prepare the list of raw links
	cleanedLinks := e.cleanLinks(source.Links, source.URL)
	
	systemPrompt := `You are an intelligent content analyzer for a knowledge graph system. Analyze the provided article and extract:
1. Important entities (people, organizations, concepts, works, events)
2. Relationships between entities
3. Relevant links that would be valuable to explore

For EACH entity, provide:
- A brief one-line description
- Rich markdown content with relevant sections like:
  - Overview paragraph with context and significance
  - Key Activities, Key Themes, or relevant section headers
  - Important quotes if applicable
  - Relationships to other entities using [[type/name]] wiki-link format
  
For events specifically, structure the content with:
- Opening paragraph describing the event
- **Date**: When it occurred
- **Location**: Where it took place
- **Participants**: List of people/organizations involved with [[type/name]] links
- **Significance**: Why this event matters
- **Context**: Background and related events
- **Outcomes**: What resulted from this event

For organizations, include:
- **Leadership**: Key people with [[people/name]] links
- **Mission/Purpose**: What the organization does
- **Key Activities**: Major initiatives or functions
- **Connections**: Related organizations and movements

For people, include:
- **Role/Position**: Their title or significance
- **Key Activities**: What they've done relevant to the article
- **Affiliations**: Organizations they're connected to with [[organizations/name]] links
- **Notable Statements**: Important quotes if any

Be comprehensive but focused - extract ALL the relevant information about each entity from the article.

For links, you should:
- ONLY include links that are directly referenced or discussed in the article content
- Categorize each link (reference, discussion, resource, navigation, advertisement)
- Rate relevance (high, medium, low) based on how central the link is to the article's topic
- Filter out navigation and advertisement links

Output JSON with this structure:
{
  "entities": [
    {
      "name": "Entity Name",
      "type": "person|organization|concept|work|event",
      "description": "One-line description",
      "content": "Rich markdown content with sections, context, and wiki-links to related entities",
      "aliases": ["alternative names mentioned"],
      "wiki_links": ["people/related-person", "organizations/related-org"]
    }
  ],
  "relationships": [
    {
      "source": "Source Entity",
      "target": "Target Entity",
      "type": "relationship type",
      "note": "context from article"
    }
  ],
  "links": [
    {
      "url": "full URL",
      "title": "link text or inferred title",
      "description": "what this link is about based on article context",
      "relevance": "high|medium|low",
      "category": "reference|discussion|resource|navigation|advertisement"
    }
  ]
}

Create rich, interconnected entities that capture the full context and significance from the article.`

	// Include the links in the prompt so the LLM can analyze them
	linksSection := ""
	if len(cleanedLinks) > 0 {
		linksSection = "\n\nLinks found in content:\n"
		for i, link := range cleanedLinks {
			if i > 50 { // Limit to first 50 to save tokens
				linksSection += fmt.Sprintf("... and %d more links\n", len(cleanedLinks)-50)
				break
			}
			linksSection += fmt.Sprintf("- %s\n", link)
		}
	}

	userPrompt := fmt.Sprintf("Analyze this content:\n\nTitle: %s\nSource URL: %s\n\nContent:\n%s%s",
		source.Title, source.URL, source.Content, linksSection)

	// Limit content length for API
	if len(userPrompt) > 10000 {
		userPrompt = userPrompt[:10000] + "\n[content truncated]"
	}

	response, err := e.llm.CompleteWithSystem(ctx, systemPrompt, userPrompt, "")
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse JSON response
	var llmResult struct {
		Entities []struct {
			Name        string   `json:"name"`
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Content     string   `json:"content"`
			Aliases     []string `json:"aliases"`
			WikiLinks   []string `json:"wiki_links"`
		} `json:"entities"`
		Relationships []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Type   string `json:"type"`
			Note   string `json:"note"`
		} `json:"relationships"`
		Links []struct {
			URL         string `json:"url"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Relevance   string `json:"relevance"`
			Category    string `json:"category"`
		} `json:"links"`
	}

	if err := json.Unmarshal([]byte(response), &llmResult); err != nil {
		// Try to extract JSON from response if it's wrapped in text
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
				return nil, fmt.Errorf("failed to parse LLM response: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse LLM response: %w", err)
		}
	}

	// Convert to our format
	result := &ExtractionResult{
		LinkedSources: []string{},
		Links:         []ExtractedLink{},
	}

	// Process entities
	for _, e := range llmResult.Entities {
		entityType := parseEntityType(e.Type)
		content := e.Content
		if content == "" {
			// Fallback to description if no rich content provided
			content = e.Description
		}
		result.Entities = append(result.Entities, ExtractedEntity{
			Name:        e.Name,
			Type:        entityType,
			Description: e.Description,
			Content:     content,
			Aliases:     e.Aliases,
			WikiLinks:   e.WikiLinks,
		})
	}

	// Process relationships
	for _, r := range llmResult.Relationships {
		result.Relationships = append(result.Relationships, ExtractedRelationship{
			Source: r.Source,
			Target: r.Target,
			Type:   r.Type,
			Note:   r.Note,
		})
	}

	// Process links - filter out navigation and low-relevance items
	for _, link := range llmResult.Links {
		// Skip navigation and advertisement links
		if link.Category == "navigation" || link.Category == "advertisement" {
			continue
		}
		
		// Skip low relevance links unless we have very few
		if link.Relevance == "low" && len(result.Links) > 5 {
			continue
		}
		
		// Ensure URL is absolute
		absoluteURL := e.makeAbsoluteURL(link.URL, source.URL)
		
		result.Links = append(result.Links, ExtractedLink{
			URL:         absoluteURL,
			Title:       link.Title,
			Description: link.Description,
			Relevance:   link.Relevance,
			Category:    link.Category,
		})
		
		// Also add to simple list for backward compatibility
		result.LinkedSources = append(result.LinkedSources, absoluteURL)
	}

	// Generate source summary
	summary, err := e.GenerateSourceSummary(ctx, source, result)
	if err != nil {
		// Log error but don't fail extraction
		fmt.Printf("Warning: failed to generate source summary: %v\n", err)
	} else {
		result.SourceSummary = summary
	}

	return result, nil
}

// cleanLinks removes duplicates and obviously irrelevant links
func (e *Extractor) cleanLinks(links []string, sourceURL string) []string {
	seen := make(map[string]bool)
	cleaned := []string{}
	
	// Could parse source URL for comparison if needed
	// Currently just removing duplicates and obvious irrelevant links
	
	for _, link := range links {
		// Skip if already seen
		if seen[link] {
			continue
		}
		seen[link] = true
		
		// Skip fragment-only links
		if strings.HasPrefix(link, "#") {
			continue
		}
		
		// Skip javascript links
		if strings.HasPrefix(link, "javascript:") {
			continue
		}
		
		// Skip mailto links
		if strings.HasPrefix(link, "mailto:") {
			continue
		}
		
		// Skip common navigation patterns
		lower := strings.ToLower(link)
		skipPatterns := []string{
			"/login", "/signin", "/signup", "/register",
			"/about", "/contact", "/privacy", "/terms",
			"/cookie", "/help", "/support", "/faq",
			"facebook.com", "twitter.com", "instagram.com",
			"linkedin.com", "youtube.com", "tiktok.com",
		}
		
		skip := false
		for _, pattern := range skipPatterns {
			if strings.Contains(lower, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		
		// Make relative URLs absolute
		absoluteURL := e.makeAbsoluteURL(link, sourceURL)
		cleaned = append(cleaned, absoluteURL)
	}
	
	return cleaned
}

// makeAbsoluteURL converts relative URLs to absolute
func (e *Extractor) makeAbsoluteURL(link, baseURL string) string {
	// Already absolute
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	
	// Parse base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return link
	}
	
	// Parse relative URL
	rel, err := url.Parse(link)
	if err != nil {
		return link
	}
	
	// Resolve relative to base
	return base.ResolveReference(rel).String()
}

// parseEntityType converts string to EntityType
func parseEntityType(typeStr string) graph.EntityType {
	switch strings.ToLower(typeStr) {
	case "person":
		return graph.EntityPerson
	case "organization", "org", "company":
		return graph.EntityOrganization
	case "concept", "idea":
		return graph.EntityConcept
	case "work", "project", "book", "paper":
		return graph.EntityWork
	case "event":
		return graph.EntityEvent
	default:
		return graph.EntityConcept
	}
}