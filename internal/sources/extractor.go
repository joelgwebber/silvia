package sources

import (
	"context"
	"encoding/json"
	"fmt"
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

// ExtractionResult contains entities and relationships found in a source
type ExtractionResult struct {
	Entities      []ExtractedEntity
	Relationships []ExtractedRelationship
	LinkedSources []string
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
	systemPrompt := `You are an entity extraction system for a knowledge graph. Extract entities, relationships, and linked sources from the provided text.

Output JSON with this structure:
{
  "entities": [
    {
      "name": "Entity Name",
      "type": "person|organization|concept|work|event",
      "description": "Brief description",
      "aliases": ["alternative", "names"]
    }
  ],
  "relationships": [
    {
      "source": "Source Entity",
      "target": "Target Entity",
      "type": "relationship type",
      "note": "optional note"
    }
  ],
  "linked_sources": ["urls mentioned or referenced"]
}

Focus on notable entities only. Be conservative - only extract entities that are clearly important to the content.`

	userPrompt := fmt.Sprintf("Extract entities from this content:\n\nTitle: %s\nURL: %s\n\nContent:\n%s",
		source.Title, source.URL, source.Content)

	// Limit content length for API
	if len(userPrompt) > 8000 {
		userPrompt = userPrompt[:8000] + "\n[content truncated]"
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
		LinkedSources []string `json:"linked_sources"`
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
		LinkedSources: append(source.Links, llmResult.LinkedSources...),
	}

	for _, e := range llmResult.Entities {
		entityType := parseEntityType(e.Type)
		result.Entities = append(result.Entities, ExtractedEntity{
			Name:        e.Name,
			Type:        entityType,
			Description: e.Description,
			Aliases:     e.Aliases,
		})
	}

	for _, r := range llmResult.Relationships {
		result.Relationships = append(result.Relationships, ExtractedRelationship{
			Source: r.Source,
			Target: r.Target,
			Type:   r.Type,
			Note:   r.Note,
		})
	}

	return result, nil
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