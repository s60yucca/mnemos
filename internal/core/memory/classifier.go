package memory

import (
	"strings"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// Classifier classifies memory type and category
type Classifier interface {
	ClassifyType(content string, tags []string) domain.MemoryType
	ClassifyCategory(content string, tags []string) string
}

// RuleClassifier uses keyword rules for classification
type RuleClassifier struct{}

func NewRuleClassifier() *RuleClassifier { return &RuleClassifier{} }

var typeKeywords = map[domain.MemoryType][]string{
	domain.MemoryTypeSemantic:  {"always", "never", "rule", "principle", "pattern", "concept", "definition", "means", "is a", "refers to"},
	domain.MemoryTypeLongTerm:  {"decided", "decision", "architecture", "design", "chosen", "will use", "standard", "convention"},
	domain.MemoryTypeEpisodic:  {"today", "yesterday", "session", "worked on", "fixed", "implemented", "added", "removed", "changed"},
	domain.MemoryTypeShortTerm: {"todo", "next", "need to", "should", "temporary", "quick", "wip", "in progress"},
}

var categoryKeywords = map[string][]string{
	domain.CategoryCode:          {"function", "method", "class", "struct", "interface", "variable", "const", "import", "package", "module"},
	domain.CategoryArchitecture:  {"architecture", "design", "pattern", "layer", "service", "microservice", "monolith", "hexagonal"},
	domain.CategoryDecision:      {"decided", "decision", "chose", "selected", "rejected", "trade-off", "because", "reason"},
	domain.CategoryBug:           {"bug", "fix", "error", "crash", "issue", "problem", "broken", "regression", "defect"},
	domain.CategoryFeature:       {"feature", "implement", "add", "new", "enhance", "support", "enable"},
	domain.CategoryAPI:           {"api", "endpoint", "rest", "graphql", "grpc", "http", "request", "response", "route"},
	domain.CategoryDatabase:      {"database", "db", "sql", "query", "table", "schema", "migration", "index", "sqlite", "postgres"},
	domain.CategorySecurity:      {"security", "auth", "authentication", "authorization", "token", "jwt", "password", "encrypt", "tls"},
	domain.CategoryPerformance:   {"performance", "slow", "fast", "optimize", "cache", "latency", "throughput", "benchmark"},
	domain.CategoryTesting:       {"test", "spec", "assert", "mock", "stub", "coverage", "unit test", "integration test"},
	domain.CategoryDeployment:    {"deploy", "ci", "cd", "docker", "kubernetes", "release", "build", "pipeline"},
	domain.CategoryDocumentation: {"doc", "readme", "comment", "explain", "describe", "documentation"},
}

func (c *RuleClassifier) ClassifyType(content string, tags []string) domain.MemoryType {
	lower := strings.ToLower(content)
	tokens := util.TokenSet(util.Tokenize(lower))

	scores := make(map[domain.MemoryType]int)

	for memType, keywords := range typeKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				scores[memType]++
			}
		}
	}

	// Tag boosts
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		switch {
		case tagLower == "semantic" || tagLower == "concept":
			scores[domain.MemoryTypeSemantic] += 3
		case tagLower == "decision" || tagLower == "architecture":
			scores[domain.MemoryTypeLongTerm] += 3
		case tagLower == "session" || tagLower == "episodic":
			scores[domain.MemoryTypeEpisodic] += 3
		case tagLower == "todo" || tagLower == "temp":
			scores[domain.MemoryTypeShortTerm] += 3
		}
	}

	// Structural analysis: long content with definitions → semantic
	_ = tokens
	if len(content) > 500 && strings.Contains(lower, "is") {
		scores[domain.MemoryTypeSemantic]++
	}

	// Pick highest score; only use semantic as default if it actually scored
	best := domain.MemoryTypeEpisodic
	bestScore := 0
	priority := []domain.MemoryType{
		domain.MemoryTypeLongTerm,
		domain.MemoryTypeEpisodic,
		domain.MemoryTypeShortTerm,
		domain.MemoryTypeSemantic,
	}
	for _, t := range priority {
		if scores[t] > bestScore {
			bestScore = scores[t]
			best = t
		}
	}
	return best
}

func (c *RuleClassifier) ClassifyCategory(content string, tags []string) string {
	lower := strings.ToLower(content)
	scores := make(map[string]int)

	for cat, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				scores[cat]++
			}
		}
	}

	// Tag matching
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		if _, ok := categoryKeywords[tagLower]; ok {
			scores[tagLower] += 5
		}
	}

	best := domain.CategoryGeneral
	bestScore := 0
	for cat, score := range scores {
		if score > bestScore {
			bestScore = score
			best = cat
		}
	}
	return best
}
