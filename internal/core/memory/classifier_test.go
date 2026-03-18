package memory

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRuleClassifier_ClassifyType(t *testing.T) {
	c := NewRuleClassifier()

	tests := []struct {
		name     string
		content  string
		tags     []string
		expected domain.MemoryType
	}{
		{
			name:     "semantic - definition",
			content:  "A microservice is a small, independently deployable service that follows the single responsibility principle",
			expected: domain.MemoryTypeSemantic,
		},
		{
			name:     "long_term - decision",
			content:  "We decided to use PostgreSQL as our primary database because of its JSONB support",
			expected: domain.MemoryTypeLongTerm,
		},
		{
			name:     "episodic - session work",
			content:  "Today I fixed the authentication bug in the login handler",
			expected: domain.MemoryTypeEpisodic,
		},
		{
			name:     "short_term - todo",
			content:  "TODO: need to refactor the payment service next sprint",
			expected: domain.MemoryTypeShortTerm,
		},
		{
			name:     "tag boost - semantic",
			content:  "Some content about patterns",
			tags:     []string{"concept"},
			expected: domain.MemoryTypeSemantic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyType(tt.content, tt.tags)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRuleClassifier_ClassifyCategory(t *testing.T) {
	c := NewRuleClassifier()

	tests := []struct {
		name     string
		content  string
		tags     []string
		expected string
	}{
		{
			name:     "code category",
			content:  "The function returns a pointer to the struct",
			expected: domain.CategoryCode,
		},
		{
			name:     "bug category",
			content:  "Fixed a crash bug in the authentication handler",
			expected: domain.CategoryBug,
		},
		{
			name:     "database category",
			content:  "Added an index on the users table for faster queries",
			expected: domain.CategoryDatabase,
		},
		{
			name:     "tag override",
			content:  "Some generic content",
			tags:     []string{"security"},
			expected: domain.CategorySecurity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyCategory(tt.content, tt.tags)
			assert.Equal(t, tt.expected, got)
		})
	}
}
