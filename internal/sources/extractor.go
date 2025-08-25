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
	Description string
	Aliases     []string
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

// Extract analyzes content and extracts entities, relationships, and linked sources
func (e *Extractor) Extract(ctx context.Context, source *Source) (*ExtractionResult, error) {
	// First, clean and prepare the list of raw links
	cleanedLinks := e.cleanLinks(source.Links, source.URL)
	
	systemPrompt := `You are an intelligent content analyzer for a knowledge graph system. Analyze the provided article and extract:
1. Important entities (people, organizations, concepts, works, events)
2. Relationships between entities
3. Relevant links that would be valuable to explore

For links, you should:
- ONLY include links that are directly referenced or discussed in the article content
- Categorize each link (reference, discussion, resource, navigation, advertisement)
- Rate relevance (high, medium, low) based on how central the link is to the article's topic
- Provide a brief description of what the link is about based on context
- Filter out obvious navigation links (home, about, contact, login, etc.)
- Filter out advertisements and unrelated promotional content
- Prioritize substantive content links (articles, papers, projects, discussions)

Output JSON with this structure:
{
  "entities": [
    {
      "name": "Entity Name",
      "type": "person|organization|concept|work|event",
      "description": "Brief description from context",
      "aliases": ["alternative names mentioned"]
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

Be selective with links - only include those that add value to understanding the topic. Maximum 10-15 most relevant links.`

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
			Aliases     []string `json:"aliases"`
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
		result.Entities = append(result.Entities, ExtractedEntity{
			Name:        e.Name,
			Type:        entityType,
			Description: e.Description,
			Aliases:     e.Aliases,
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