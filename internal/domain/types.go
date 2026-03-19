package domain

// MemoryType categorizes the nature of a memory
type MemoryType string

const (
	MemoryTypeShortTerm MemoryType = "short_term"
	MemoryTypeLongTerm  MemoryType = "long_term"
	MemoryTypeEpisodic  MemoryType = "episodic"
	MemoryTypeSemantic  MemoryType = "semantic"
)

func (t MemoryType) IsValid() bool {
	switch t {
	case MemoryTypeShortTerm, MemoryTypeLongTerm, MemoryTypeEpisodic, MemoryTypeSemantic:
		return true
	}
	return false
}

func (t MemoryType) String() string { return string(t) }

// DefaultDecayRate returns the type-specific lambda for exponential decay
func (t MemoryType) DefaultDecayRate() float64 {
	switch t {
	case MemoryTypeShortTerm:
		return 0.1
	case MemoryTypeLongTerm:
		return 0.005
	case MemoryTypeEpisodic:
		return 0.02
	case MemoryTypeSemantic:
		return 0.001
	default:
		return 0.02
	}
}

// MemoryStatus represents the lifecycle state of a memory
type MemoryStatus string

const (
	MemoryStatusActive   MemoryStatus = "active"
	MemoryStatusArchived MemoryStatus = "archived"
	MemoryStatusDeleted  MemoryStatus = "deleted"
)

func (s MemoryStatus) IsValid() bool {
	switch s {
	case MemoryStatusActive, MemoryStatusArchived, MemoryStatusDeleted:
		return true
	}
	return false
}

func (s MemoryStatus) String() string { return string(s) }

// RelationType defines how two memories relate to each other
type RelationType string

const (
	RelationTypeRelatesTo   RelationType = "relates_to"
	RelationTypeDependsOn   RelationType = "depends_on"
	RelationTypeContradicts RelationType = "contradicts"
	RelationTypeSupersedes  RelationType = "supersedes"
	RelationTypeDerivedFrom RelationType = "derived_from"
	RelationTypePartOf      RelationType = "part_of"
	RelationTypeCausedBy    RelationType = "caused_by"
	RelationTypeReferences  RelationType = "references" // alias for relates_to
)

func (r RelationType) IsValid() bool {
	switch r {
	case RelationTypeRelatesTo, RelationTypeDependsOn, RelationTypeContradicts,
		RelationTypeSupersedes, RelationTypeDerivedFrom, RelationTypePartOf, RelationTypeCausedBy,
		RelationTypeReferences:
		return true
	}
	return false
}

func (r RelationType) String() string { return string(r) }

// Built-in categories
const (
	CategoryCode         = "code"
	CategoryArchitecture = "architecture"
	CategoryDecision     = "decision"
	CategoryBug          = "bug"
	CategoryFeature      = "feature"
	CategoryAPI          = "api"
	CategoryDatabase     = "database"
	CategorySecurity     = "security"
	CategoryPerformance  = "performance"
	CategoryTesting      = "testing"
	CategoryDeployment   = "deployment"
	CategoryDocumentation = "documentation"
	CategoryGeneral      = "general"
)
