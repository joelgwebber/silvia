package prompts

// CitationGuidelines provides Wikipedia-style citation principles for entity content
const CitationGuidelines = `CITATION GUIDELINES:
Follow Wikipedia-style citation principles to ensure all information is properly sourced:

CORE PRINCIPLES:
- Every significant claim, statistic, or characterization must reference its source
- Use explicit, unambiguous source references that include the publication and date
- When multiple sources support a claim, mention the primary or most authoritative one
- Quotations must always identify their specific source article
- Contested or potentially controversial statements require explicit sourcing

WHAT NEEDS CITATIONS:
- Specific facts, dates, numbers, and statistics
- Direct quotes or paraphrased statements
- Controversial or disputed claims
- Characterizations of people, organizations, or events
- Assertions about relationships or connections
- Claims about motivations, beliefs, or intentions

WHAT MAY NOT NEED CITATIONS:
- Uncontroversial background information
- Generally accepted facts
- Simple descriptive statements visible in the source

CITATION FORMAT:
Use direct wiki-link citations to source entities in the graph:
- "According to [[sources/www-dailykos-com-2025-08-29]], Musk's associates..."
- "The investigation [[sources/politico-com-2025-05-23]] revealed..."
- "Wilson announced [[sources/dougwils-mission-babylon-2025]] that..."
- For inline references: "this development [[sources/domain-date]]"
- For quotes: "Quote text" [[sources/domain-date]]

STYLE GUIDANCE:
- Use wiki-links to source entities rather than verbose descriptions
- The source entity ID format is typically: sources/domain-date
- Keep text concise - let the wiki-link provide the full reference
- You can introduce the source briefly if it flows better: "Singer argues [[sources/www-dailykos-com-2025-08-29]] that..."
- Or just use the link directly: "According to [[sources/www-dailykos-com-2025-08-29]]..."

IMPORTANT: When a source entity exists in the graph (which it should for all ingested sources), 
use the wiki-link format [[sources/domain-date]] rather than inline text citations. 
This creates navigable connections in the knowledge graph.
`

// EntityContentGuidelines provides standards for entity content creation
const EntityContentGuidelines = `ENTITY CONTENT STANDARDS:
1. Incorporates relevant details from sources WITH CLEAR ATTRIBUTION
2. Maintains factual accuracy with source citations for verifiable claims
3. Provides context and background, citing sources for new information
4. Keeps a neutral, encyclopedic tone
5. Preserves all existing relationships and cross-references using [[entity-id]] format
6. Clearly indicates which source each significant assertion comes from
`

// GetCitationGuidelines returns the citation guidelines
func GetCitationGuidelines() string {
	return CitationGuidelines
}

// GetEntityContentGuidelines returns the entity content guidelines
func GetEntityContentGuidelines() string {
	return EntityContentGuidelines
}

// GetFullGuidelines returns both citation and content guidelines
func GetFullGuidelines() string {
	return CitationGuidelines + "\n" + EntityContentGuidelines
}
