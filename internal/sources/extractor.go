package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/prompts"
)

// ExtractedEntity represents an entity found in text
type ExtractedEntity struct {
	Name        string
	Type        graph.EntityType
	Description string // Brief one-line description
	Content     string // Rich markdown content with sections
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
	LinkedSources []string        // Simple list for backward compatibility
	Links         []ExtractedLink // Enhanced link information
	SourceSummary *SourceSummary  // Structured summary of the source
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
	llm   *llm.Client
	debug bool
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

// SetDebug enables or disables debug mode
func (e *Extractor) SetDebug(debug bool) {
	e.debug = debug
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

	if e.debug {
		fmt.Printf("\n[DEBUG] LLM Response for source summary:\n%s\n", response)
	}

	// Parse response
	var result struct {
		Title       string   `json:"title"`
		Author      string   `json:"author"`
		Publication string   `json:"publication"`
		Date        string   `json:"date"`
		KeyThemes   []string `json:"key_themes"`
		KeyQuotes   []string `json:"key_quotes"`
		Analysis    string   `json:"analysis"`
	}

	// Clean and parse JSON
	jsonStr := response

	// Try to extract JSON if wrapped in markdown or other text
	if !strings.HasPrefix(strings.TrimSpace(response), "{") {
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			jsonStr = response[jsonStart : jsonEnd+1]
		}
	}

	// Remove any markdown code block markers
	jsonStr = strings.ReplaceAll(jsonStr, "```json", "")
	jsonStr = strings.ReplaceAll(jsonStr, "```", "")
	jsonStr = strings.TrimSpace(jsonStr)

	if e.debug {
		fmt.Printf("[DEBUG] Cleaned JSON:\n%s\n", jsonStr)
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		if e.debug {
			fmt.Printf("[DEBUG] JSON parse error: %v\n", err)
			// Try to identify the specific issue
			var syntaxErr *json.SyntaxError
			if errors.As(err, &syntaxErr) {
				fmt.Printf("[DEBUG] Syntax error at byte %d: %s\n", syntaxErr.Offset, syntaxErr.Error())
				if syntaxErr.Offset > 0 && syntaxErr.Offset < int64(len(jsonStr)) {
					start := syntaxErr.Offset - 20
					if start < 0 {
						start = 0
					}
					end := syntaxErr.Offset + 20
					if end > int64(len(jsonStr)) {
						end = int64(len(jsonStr))
					}
					fmt.Printf("[DEBUG] Context: ...%s...\n", jsonStr[start:end])
				}
			}
		}
		return nil, fmt.Errorf("failed to parse summary JSON: %w (response length: %d)", err, len(response))
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

	if e.debug {
		fmt.Printf("[DEBUG] Extract: Source has %d raw links, %d after cleaning\n", len(source.Links), len(cleanedLinks))
	}

	systemPrompt := `You are an intelligent content analyzer for a knowledge graph system. Analyze the provided article and extract:
1. Important entities (people, organizations, concepts, works, events)
2. Relationships between entities
3. Relevant links from the provided list that would be valuable to explore further

IMPORTANT: You will be provided with a list of links found in the article. Review these links and include the most relevant ones in your response.

` + prompts.GetCitationGuidelines() + `

For EACH entity, provide:
- A brief one-line description
- Rich markdown content with proper source attribution that includes:
  - Overview paragraph with context and significance (citing sources)
  - Key Activities, Key Themes, or relevant section headers
  - Important quotes with their sources identified
  - Relationships to other entities using [[type/name]] wiki-link format
  
For events specifically, structure the content with:
- Opening paragraph describing the event with concise source attribution
- **Date**: When it occurred with citation if specific
- **Location**: Where it took place
- **Participants**: List of people/organizations involved with [[type/name]] links
- **Significance**: Why this event matters with source
- **Context**: Background and related events
- **Outcomes**: What resulted from this event with attribution

For organizations, include:
- **Leadership**: Key people with [[people/name]] links and source attribution
- **Mission/Purpose**: What the organization does with citation
- **Key Activities**: Major initiatives or functions with source reference
- **Connections**: Related organizations and movements

For people, include:
- **Role/Position**: Their title or significance with citation
- **Key Activities**: What they've done relevant to the article with attribution
- **Affiliations**: Organizations they're connected to with [[organizations/name]] links
- **Notable Statements**: Important quotes with source identified

IMPORTANT: Use direct, concise citations. Instead of "According to an article by X in Y", write "X (Y, date) states..." or similar. Keep citations brief but complete.

For links provided in the source, evaluate each one and:
- Include links that could provide valuable context or additional information
- Look for links to sources, references, related articles, or supporting documentation
- Categorize each link: reference (cited source), discussion (related topic), resource (tool/data), navigation (site nav), advertisement (promotional)
- Rate relevance: high (directly related to main topic), medium (provides context), low (tangentially related)
- You can include up to 20 of the most relevant links
- Prioritize reference and discussion links over navigation/ads

You MUST output valid JSON and nothing else. Output JSON with this structure:
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

Create rich, interconnected entities that capture the full context and significance from the article, with proper source attribution for all claims.`

	// Include the links in the prompt so the LLM can analyze them
	linksSection := ""
	if len(cleanedLinks) > 0 {
		linksSection = "\n\n=== LINKS TO ANALYZE ===\nPlease review these links and include the most relevant ones in your 'links' array:\n"
		for i, link := range cleanedLinks {
			if i >= 30 { // Limit to first 30 to save tokens but get better coverage
				linksSection += fmt.Sprintf("\n... and %d more links available\n", len(cleanedLinks)-30)
				break
			}
			linksSection += fmt.Sprintf("%d. %s\n", i+1, link)
		}
		linksSection += "\n=== END OF LINKS ===\n"
		if e.debug {
			fmt.Printf("[DEBUG] Including %d links in LLM prompt (showing first 30)\n", len(cleanedLinks))
		}
	} else if e.debug {
		fmt.Printf("[DEBUG] No links to include in LLM prompt\n")
	}

	// Build source metadata string
	sourceInfo := fmt.Sprintf("Title: %s\nSource URL: %s", source.Title, source.URL)

	// Generate the source entity ID that will be created
	sourceEntityID := e.generateSourceEntityID(source.URL)
	sourceInfo += fmt.Sprintf("\nSource Entity ID: [[%s]]", sourceEntityID)

	if author, ok := source.Metadata["author"]; ok && author != "" {
		sourceInfo += fmt.Sprintf("\nAuthor: %s", author)
	}
	if date, ok := source.Metadata["date"]; ok && date != "" {
		sourceInfo += fmt.Sprintf("\nPublication Date: %s", date)
	}
	if publication, ok := source.Metadata["publication"]; ok && publication != "" {
		sourceInfo += fmt.Sprintf("\nPublication: %s", publication)
	}

	userPrompt := fmt.Sprintf("Analyze this content:\n\n%s\n\nIMPORTANT: When citing this source, use the wiki-link format [[%s]] rather than verbose inline citations.\n\nContent:\n%s%s",
		sourceInfo, sourceEntityID, source.Content, linksSection)

	// Limit content length for API
	if len(userPrompt) > 10000 {
		userPrompt = userPrompt[:10000] + "\n[content truncated]"
	}

	// Use structured output for type-safe JSON responses
	var llmResult LLMExtractionResult
	if err := e.llm.CompleteWithStructuredOutput(ctx, systemPrompt, userPrompt, &llmResult, ""); err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	if e.debug {
		fmt.Printf("[DEBUG] LLM returned: %d entities, %d relationships, %d links\n",
			len(llmResult.Entities), len(llmResult.Relationships), len(llmResult.Links))
		if len(llmResult.Links) > 0 {
			fmt.Printf("[DEBUG] First link from LLM: %+v\n", llmResult.Links[0])
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
	skippedNav := 0
	skippedLow := 0
	for _, link := range llmResult.Links {
		// Skip navigation and advertisement links
		if link.Category == "navigation" || link.Category == "advertisement" {
			skippedNav++
			continue
		}

		// Skip low relevance links unless we have very few
		if link.Relevance == "low" && len(result.Links) > 10 {
			skippedLow++
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

	if e.debug && (skippedNav > 0 || skippedLow > 0) {
		fmt.Printf("[DEBUG] Link filtering: skipped %d nav/ads, %d low relevance, kept %d links\n",
			skippedNav, skippedLow, len(result.Links))
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

// generateSourceEntityID creates a consistent source entity ID from a URL
func (e *Extractor) generateSourceEntityID(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		// Fallback to simple domain extraction
		return fmt.Sprintf("sources/source-%s", time.Now().Format("2006-01-02"))
	}

	// Create ID like "sources/domain-date"
	domain := strings.ReplaceAll(u.Hostname(), ".", "-")
	return fmt.Sprintf("sources/%s-%s", domain, time.Now().Format("2006-01-02"))
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
