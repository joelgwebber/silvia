package sources

// LLMExtractionResult represents the structured output from the LLM
type LLMExtractionResult struct {
	Entities      []LLMExtractedEntity       `json:"entities"`
	Relationships []LLMExtractedRelationship `json:"relationships"`
	Links         []LLMExtractedLink         `json:"links"`
}

// LLMExtractedEntity represents an entity extracted by the LLM
type LLMExtractedEntity struct {
	Name        string   `json:"name" jsonschema:"required,description=Entity name"`
	Type        string   `json:"type" jsonschema:"required,enum=person;organization;concept;work;event,description=Entity type"`
	Description string   `json:"description" jsonschema:"required,description=One-line description"`
	Content     string   `json:"content" jsonschema:"required,description=Rich markdown content with sections and wiki-links"`
	Aliases     []string `json:"aliases,omitempty" jsonschema:"description=Alternative names mentioned"`
	WikiLinks   []string `json:"wiki_links,omitempty" jsonschema:"description=Related entities in type/id format"`
}

// LLMExtractedRelationship represents a relationship extracted by the LLM
type LLMExtractedRelationship struct {
	Source string `json:"source" jsonschema:"required,description=Source entity name"`
	Target string `json:"target" jsonschema:"required,description=Target entity name"`
	Type   string `json:"type" jsonschema:"required,description=Relationship type"`
	Note   string `json:"note,omitempty" jsonschema:"description=Context from article"`
}

// LLMExtractedLink represents a link extracted by the LLM
type LLMExtractedLink struct {
	URL         string `json:"url" jsonschema:"required,description=Full URL"`
	Title       string `json:"title" jsonschema:"required,description=Link text or inferred title"`
	Description string `json:"description,omitempty" jsonschema:"description=What this link is about"`
	Relevance   string `json:"relevance" jsonschema:"required,enum=high;medium;low,description=Relevance level"`
	Category    string `json:"category" jsonschema:"required,enum=reference;discussion;resource;navigation;advertisement,description=Link category"`
}