package benchmark

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkoukk/tiktoken-go"
)

// TokenCounter provides token counting using tiktoken approximation.
type TokenCounter struct {
	encoding *tiktoken.Tiktoken
}

// NewTokenCounter creates a new token counter with cl100k_base encoding.
func NewTokenCounter() (*TokenCounter, error) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to load tiktoken encoding: %w", err)
	}
	return &TokenCounter{encoding: encoding}, nil
}

// CountTokens counts tokens in a text string using tiktoken.
func (c *TokenCounter) CountTokens(text string) int {
	if text == "" {
		return 0
	}
	tokens := c.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountRequest counts tokens in an MCP request.
func (c *TokenCounter) CountRequest(req mcp.CallToolRequest) int {
	// Serialize the request to JSON to get a text representation
	data, err := json.Marshal(req)
	if err != nil {
		return 0
	}
	return c.CountTokens(string(data))
}

// CountResponse counts tokens in an MCP response.
func (c *TokenCounter) CountResponse(resp *mcp.CallToolResult) int {
	if resp == nil {
		return 0
	}
	// Serialize the response to JSON to get a text representation
	data, err := json.Marshal(resp)
	if err != nil {
		return 0
	}
	return c.CountTokens(string(data))
}
