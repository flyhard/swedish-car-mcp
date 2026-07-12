package mcpjson

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TextResult marshals v as indented JSON text for MCP tool responses,
// matching the Python FastMCP json.dumps(..., indent=2) style.
func TextResult(v any) (*mcp.CallToolResult, struct{}, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, struct{}{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, struct{}{}, nil
}

// ErrorResult returns a recoverable tool error visible to the model.
func ErrorResult(msg string) (*mcp.CallToolResult, struct{}, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, struct{}{}, nil
}
