package mcpserve

import "github.com/tinywasm/mcp"

// Loggable defines the interface for handlers that support logging
type Loggable interface {
	Name() string
	SetLog(logger func(message ...any))
}

// ToolProvider defines the interface for handlers that support MCP tools
type ToolProvider interface {
	GetMCPTools() []Tool
}

// ToolExecutor defines how a tool should be executed
// Handlers implement this to provide execution logic without exposing internals
// args: map of parameter name to value from MCP request
type ToolExecutor func(args map[string]any)

// Tool provides MCP tool configuration metadata
// This is the standard interface that all handlers should implement
type Tool struct {
	Name        string
	Description string
	Parameters  []Parameter
	Execute     ToolExecutor // Handler provides execution function
}

// Parameter describes a tool parameter
type Parameter struct {
	Name        string
	Description string
	Required    bool
	Type        string // "string", "number", "boolean"
	EnumValues  []string
	Default     any
}

// buildMCPTool converts mcpserve.Tool to mcp.Tool for registration.
// mcpserve.Tool and mcp.Tool are structurally identical, just cast the parameters.
func buildMCPTool(meta Tool) *mcp.Tool {
	// Convert mcpserve.Parameter[] to mcp.Parameter[]
	params := make([]mcp.Parameter, len(meta.Parameters))
	for i, p := range meta.Parameters {
		params[i] = mcp.Parameter{
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
			Type:        p.Type,
			EnumValues:  p.EnumValues,
			Default:     p.Default,
		}
	}

	tool := &mcp.Tool{
		Name:        meta.Name,
		Description: meta.Description,
		Parameters:  params,
		Execute:     meta.Execute,
	}
	return tool
}
