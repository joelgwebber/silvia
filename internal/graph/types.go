package graph

import (
	"time"
)

// EntityType represents the type of entity in the graph
type EntityType string

const (
	EntityPerson       EntityType = "person"
	EntityOrganization EntityType = "organization"
	EntityConcept      EntityType = "concept"
	EntityWork         EntityType = "work"
	EntityEvent        EntityType = "event"
)

// Metadata contains the frontmatter metadata for an entity
type Metadata struct {
	ID       string    `yaml:"id"`
	Type     EntityType `yaml:"type"`
	Aliases  []string  `yaml:"aliases,omitempty"`
	Created  time.Time `yaml:"created"`
	Updated  time.Time `yaml:"updated"`
	Sources  []string  `yaml:"sources,omitempty"`
	Tags     []string  `yaml:"tags,omitempty"`
}

// Entity represents a node in the knowledge graph
type Entity struct {
	Metadata      Metadata
	Title         string
	Content       string
	Relationships []Relationship
	BackRefs      []BackReference
}

// Relationship represents a connection from this entity to another
type Relationship struct {
	Type   string   `yaml:"type"`   // e.g., "founded", "authored", "attended"
	Target string   `yaml:"target"` // ID of the target entity
	Date   *time.Time `yaml:"date,omitempty"`
	Note   string   `yaml:"note,omitempty"`
}

// BackReference represents an incoming reference from another entity
type BackReference struct {
	Source string `yaml:"source"` // ID of the referring entity
	Type   string `yaml:"type"`   // Type of relationship
	Note   string `yaml:"note,omitempty"`
}

// RelationshipType defines common relationship types
type RelationshipType string

const (
	RelFounded     RelationshipType = "founded"
	RelAuthored    RelationshipType = "authored"
	RelRecommended RelationshipType = "recommended"
	RelAttended    RelationshipType = "attended"
	RelMemberOf    RelationshipType = "member_of"
	RelSpokeAt     RelationshipType = "spoke_at"
	RelConnected   RelationshipType = "connected"
)