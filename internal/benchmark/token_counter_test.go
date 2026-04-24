package benchmark

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenCounter(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)
	assert.NotNil(t, counter)
	assert.NotNil(t, counter.encoding)
}

func TestCountTokens_HappyPath(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "simple sentence",
			text:     "Hello, world!",
			minCount: 3,
			maxCount: 5,
		},
		{
			name:     "longer text",
			text:     "This is a longer sentence with more words to count tokens.",
			minCount: 11,
			maxCount: 15,
		},
		{
			name:     "code snippet",
			text:     "func main() { fmt.Println(\"Hello\") }",
			minCount: 8,
			maxCount: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := counter.CountTokens(tt.text)
			assert.GreaterOrEqual(t, count, tt.minCount, "token count should be at least minimum")
			assert.LessOrEqual(t, count, tt.maxCount, "token count should be at most maximum")
		})
	}
}

func TestCountTokens_EdgeCases(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	t.Run("empty string", func(t *testing.T) {
		count := counter.CountTokens("")
		assert.Equal(t, 0, count)
	})

	t.Run("very long text", func(t *testing.T) {
		// Create a text with >10k tokens
		longText := strings.Repeat("This is a test sentence with multiple words. ", 500)
		count := counter.CountTokens(longText)
		assert.Greater(t, count, 4000, "should handle very long text")
	})

	t.Run("unicode characters", func(t *testing.T) {
		unicodeText := "Hello 世界 🌍 Привет"
		count := counter.CountTokens(unicodeText)
		assert.Greater(t, count, 0, "should handle unicode")
		assert.Less(t, count, 20, "unicode token count should be reasonable")
	})

	t.Run("special characters", func(t *testing.T) {
		specialText := "!@#$%^&*()_+-=[]{}|;':\",./<>?"
		count := counter.CountTokens(specialText)
		assert.Greater(t, count, 0, "should handle special characters")
	})
}

func TestCountRequest(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	t.Run("valid request", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "mnemos_store",
				Arguments: map[string]any{
					"content": "This is a test memory content",
					"tags":    "test,benchmark",
				},
			},
		}
		count := counter.CountRequest(req)
		assert.Greater(t, count, 0, "should count tokens in request")
	})

	t.Run("empty request", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		count := counter.CountRequest(req)
		assert.GreaterOrEqual(t, count, 0, "should handle empty request")
	})

	t.Run("large request", func(t *testing.T) {
		largeContent := strings.Repeat("This is a large memory content. ", 100)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "mnemos_store",
				Arguments: map[string]any{
					"content": largeContent,
				},
			},
		}
		count := counter.CountRequest(req)
		assert.Greater(t, count, 100, "should handle large request")
	})
}

func TestCountResponse(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	t.Run("valid response", func(t *testing.T) {
		resp := mcp.NewToolResultText("Memory stored successfully with ID: abc-123")
		count := counter.CountResponse(resp)
		assert.Greater(t, count, 0, "should count tokens in response")
	})

	t.Run("nil response", func(t *testing.T) {
		count := counter.CountResponse(nil)
		assert.Equal(t, 0, count, "should return 0 for nil response")
	})

	t.Run("empty response", func(t *testing.T) {
		resp := mcp.NewToolResultText("")
		count := counter.CountResponse(resp)
		assert.GreaterOrEqual(t, count, 0, "should handle empty response")
	})

	t.Run("large response", func(t *testing.T) {
		largeText := strings.Repeat("This is a large response content. ", 100)
		resp := mcp.NewToolResultText(largeText)
		count := counter.CountResponse(resp)
		assert.Greater(t, count, 100, "should handle large response")
	})
}

func TestTokenCounter_Consistency(t *testing.T) {
	counter, err := NewTokenCounter()
	require.NoError(t, err)

	text := "This is a test sentence for consistency checking."

	// Count the same text multiple times
	count1 := counter.CountTokens(text)
	count2 := counter.CountTokens(text)
	count3 := counter.CountTokens(text)

	assert.Equal(t, count1, count2, "counts should be consistent")
	assert.Equal(t, count2, count3, "counts should be consistent")
}
