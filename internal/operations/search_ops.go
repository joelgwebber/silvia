package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"silvia/internal/graph"
)

// SearchOps handles search and relationship queries
type SearchOps struct {
	graph   *graph.Manager
	dataDir string
}

// NewSearchOps creates a new search operations handler
func NewSearchOps(graphManager *graph.Manager, dataDir string) *SearchOps {
	return &SearchOps{
		graph:   graphManager,
		dataDir: dataDir,
	}
}

// SearchEntities searches for entities matching a query
func (s *SearchOps) SearchEntities(query string) (*SearchResult, error) {
	if query == "" {
		return nil, NewOperationError("search entities", "", fmt.Errorf("query cannot be empty"))
	}

	// Use graph's search function
	matches, err := s.graph.SearchEntities(query)
	if err != nil {
		return nil, NewOperationError("search entities", query, err)
	}

	// Convert to SearchResult
	searchMatches := make([]SearchMatch, len(matches))
	for i, match := range matches {
		// Create snippet from content
		snippet := s.createSnippet(match.Content, query, 150)

		// Find matching terms
		queryLower := strings.ToLower(query)
		titleLower := strings.ToLower(match.Title)
		contentLower := strings.ToLower(match.Content)

		matchedTerms := []string{}
		if strings.Contains(titleLower, queryLower) {
			matchedTerms = append(matchedTerms, "title")
		}
		if strings.Contains(contentLower, queryLower) {
			matchedTerms = append(matchedTerms, "content")
		}
		if strings.Contains(strings.ToLower(match.Metadata.ID), queryLower) {
			matchedTerms = append(matchedTerms, "id")
		}

		searchMatches[i] = SearchMatch{
			Entity:  match,
			Score:   s.calculateScore(match, query),
			Snippet: snippet,
			Matches: matchedTerms,
		}
	}

	return &SearchResult{
		Query:   query,
		Results: searchMatches,
		Total:   len(searchMatches),
	}, nil
}

// GetRelatedEntities gets all entities related to a specific entity
func (s *SearchOps) GetRelatedEntities(entityID string) (*RelatedEntitiesResult, error) {
	// Use graph's GetRelatedEntities which returns the detailed result
	result, err := s.graph.GetRelatedEntities(entityID)
	if err != nil {
		return nil, NewOperationError("get related entities", entityID, err)
	}

	// Convert graph.RelatedEntitiesResult to operations.RelatedEntitiesResult
	return &RelatedEntitiesResult{
		Entity:         result.Entity,
		OutgoingByType: result.OutgoingByType,
		IncomingByType: result.IncomingByType,
		BrokenLinks:    result.BrokenLinks,
		All:            result.All,
	}, nil
}

// GetEntitiesByType returns all entities of a specific type
func (s *SearchOps) GetEntitiesByType(entityType string) ([]*graph.Entity, error) {
	// Validate entity type
	validTypes := []string{"person", "organization", "concept", "work", "event"}
	isValid := false
	for _, t := range validTypes {
		if t == entityType {
			isValid = true
			break
		}
	}
	if !isValid {
		return nil, NewOperationError("get entities by type", entityType,
			fmt.Errorf("invalid entity type: %s", entityType))
	}

	// Get all entities from the graph
	allEntities, err := s.getAllEntities()
	if err != nil {
		return nil, NewOperationError("get entities by type", entityType, err)
	}

	// Filter by type
	var filtered []*graph.Entity
	for _, entity := range allEntities {
		if string(entity.Metadata.Type) == entityType {
			filtered = append(filtered, entity)
		}
	}

	return filtered, nil
}

// SuggestRelated suggests entities that might be related based on content similarity
func (s *SearchOps) SuggestRelated(entityID string, limit int) ([]*graph.Entity, error) {
	// Load the source entity
	entity, err := s.graph.LoadEntity(entityID)
	if err != nil {
		return nil, NewOperationError("suggest related", entityID, err)
	}

	// Get all entities
	// Get all entities from the graph
	allEntities, err := s.getAllEntities()
	if err != nil {
		return nil, NewOperationError("suggest related", entityID, err)
	}

	// Score each entity based on content similarity
	type scoredEntity struct {
		entity *graph.Entity
		score  float64
	}
	scores := []scoredEntity{}

	for _, other := range allEntities {
		if other.Metadata.ID == entityID {
			continue // Skip self
		}

		// Simple scoring based on shared terms
		score := s.calculateSimilarity(entity, other)
		if score > 0 {
			scores = append(scores, scoredEntity{
				entity: other,
				score:  score,
			})
		}
	}

	// Sort by score
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Return top N
	result := []*graph.Entity{}
	for i := 0; i < limit && i < len(scores); i++ {
		result = append(result, scores[i].entity)
	}

	return result, nil
}

// Helper functions

// createSnippet creates a text snippet around the query match
func (s *SearchOps) createSnippet(content, query string, maxLength int) string {
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)

	// Find position of query in content
	pos := strings.Index(contentLower, queryLower)
	if pos == -1 {
		// Query not found in content, return beginning
		if len(content) <= maxLength {
			return content
		}
		return content[:maxLength] + "..."
	}

	// Calculate snippet boundaries
	start := pos - 50
	if start < 0 {
		start = 0
	}
	end := pos + len(query) + 100
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]

	// Add ellipsis if needed
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}

// calculateScore calculates a relevance score for a search match
func (s *SearchOps) calculateScore(entity *graph.Entity, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Title match is worth more
	if strings.Contains(strings.ToLower(entity.Title), queryLower) {
		score += 10.0
	}

	// ID match
	if strings.Contains(strings.ToLower(entity.Metadata.ID), queryLower) {
		score += 5.0
	}

	// Count occurrences in content
	contentLower := strings.ToLower(entity.Content)
	count := strings.Count(contentLower, queryLower)
	score += float64(count)

	return score
}

// calculateSimilarity calculates similarity between two entities
func (s *SearchOps) calculateSimilarity(entity1, entity2 *graph.Entity) float64 {
	// Simple term-based similarity
	terms1 := s.extractTerms(entity1.Content)
	terms2 := s.extractTerms(entity2.Content)

	// Count shared terms
	shared := 0
	for term := range terms1 {
		if terms2[term] {
			shared++
		}
	}

	if len(terms1) == 0 || len(terms2) == 0 {
		return 0
	}

	// Jaccard similarity
	union := len(terms1) + len(terms2) - shared
	if union == 0 {
		return 0
	}

	return float64(shared) / float64(union)
}

// extractTerms extracts significant terms from text
func (s *SearchOps) extractTerms(text string) map[string]bool {
	terms := make(map[string]bool)

	// Simple tokenization
	words := strings.Fields(strings.ToLower(text))
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,!?;:\"'")

		// Skip short words
		if len(word) < 4 {
			continue
		}

		// Skip common words (basic stopword list)
		if isStopword(word) {
			continue
		}

		terms[word] = true
	}

	return terms
}

// isStopword checks if a word is a common stopword
func isStopword(word string) bool {
	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true,
		"with": true, "this": true, "that": true, "from": true,
		"have": true, "been": true, "were": true, "will": true,
		"would": true, "could": true, "should": true, "which": true,
		"their": true, "they": true, "what": true, "when": true,
		"where": true, "there": true, "these": true, "those": true,
	}
	return stopwords[word]
}

// getAllEntities returns all entities in the graph
func (s *SearchOps) getAllEntities() ([]*graph.Entity, error) {
	var entities []*graph.Entity

	graphDir := filepath.Join(s.dataDir, "graph")
	err := filepath.Walk(graphDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Extract entity ID from path
		relPath, _ := filepath.Rel(graphDir, path)
		entityID := strings.TrimSuffix(relPath, ".md")

		entity, err := s.graph.LoadEntity(entityID)
		if err != nil {
			// Log error but continue walking
			fmt.Printf("Warning: failed to load %s: %v\n", entityID, err)
			return nil
		}

		entities = append(entities, entity)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk graph directory: %w", err)
	}

	return entities, nil
}
