package mcpserve

import "github.com/tinywasm/mcp/mcp"

// Loggable defines the interface for handlers that support logging
type Loggable interface {
	Name() string
	SetLog(logger func(message ...any))
}

// ToolProvider defines the interface for handlers that support MCP tools
type ToolProvider interface {
	GetMCPToolsMetadata() []ToolMetadata
}

// ToolExecutor defines how a tool should be executed
// Handlers implement this to provide execution logic without exposing internals
// args: map of parameter name to value from MCP request
type ToolExecutor func(args map[string]any)

// ToolMetadata provides MCP tool configuration metadata
// This is the standard interface that all handlers should implement
type ToolMetadata struct {
	Name        string
	Description string
	Parameters  []ParameterMetadata
	Execute     ToolExecutor // Handler provides execution function
}

// ParameterMetadata describes a tool parameter
type ParameterMetadata struct {
	Name        string
	Description string
	Required    bool
	Type        string // "string", "number", "boolean"
	EnumValues  []string
	Default     any
}

// buildMCPTool constructs MCP tool from metadata
func buildMCPTool(meta ToolMetadata) *mcp.Tool {
	options := []mcp.ToolOption{
		mcp.WithDescription(meta.Description),
	}

	for _, param := range meta.Parameters {
		switch param.Type {
		case "string":
			// Build string parameter options directly
			var strOpts []mcp.PropertyOption

			if param.Required {
				strOpts = append(strOpts, mcp.Required())
			}
			if param.Description != "" {
				strOpts = append(strOpts, mcp.Description(param.Description))
			}
			if len(param.EnumValues) > 0 {
				strOpts = append(strOpts, mcp.Enum(param.EnumValues...))
			}
			if param.Default != nil {
				if defaultStr, ok := param.Default.(string); ok {
					strOpts = append(strOpts, mcp.DefaultString(defaultStr))
				}
			}

			options = append(options, mcp.WithString(param.Name, strOpts...))

		case "number":
			// Build number parameter options directly
			var numOpts []mcp.PropertyOption

			if param.Required {
				numOpts = append(numOpts, mcp.Required())
			}
			if param.Description != "" {
				numOpts = append(numOpts, mcp.Description(param.Description))
			}
			if param.Default != nil {
				if defaultNum, ok := param.Default.(float64); ok {
					numOpts = append(numOpts, mcp.DefaultNumber(defaultNum))
				}
			}

			options = append(options, mcp.WithNumber(param.Name, numOpts...))

		case "boolean":
			// Build boolean parameter options directly
			var boolOpts []mcp.PropertyOption

			if param.Required {
				boolOpts = append(boolOpts, mcp.Required())
			}
			if param.Description != "" {
				boolOpts = append(boolOpts, mcp.Description(param.Description))
			}
			// Note: DefaultBoolean might not exist in mcp-go, skip for now

			options = append(options, mcp.WithBoolean(param.Name, boolOpts...))
		}
	}

	tool := mcp.NewTool(meta.Name, options...)
	return &tool
}
