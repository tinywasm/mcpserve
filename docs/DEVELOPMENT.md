# Adding Tools to Handlers

To add MCP tools to a package (e.g., `client`, `browser`) without adding MCP dependencies:

## 1. Define Metadata Structs
Copy these exact structs into your package. Reflection will map them to `mcpserve` types.

```go
type ToolExecutor func(args map[string]any, progress chan<- any)

type ToolMetadata struct {
	Name        string
	Description string
	Parameters  []ParameterMetadata
	Execute     ToolExecutor
}

type ParameterMetadata struct {
	Name        string
	Description string
	Required    bool
	Type        string // "string", "number", "boolean"
	EnumValues  []string
	Default     any
}
```

## 2. Implement the Discovery Method
Implement `GetMCPToolsMetadata() []ToolMetadata` on your handler.

```go
func (h *MyHandler) GetMCPToolsMetadata() []ToolMetadata {
	return []ToolMetadata{
		{
			Name: "tool_name",
			Description: "What it does",
			Parameters: []ParameterMetadata{
				{Name: "param1", Type: "string", Required: true},
			},
			Execute: func(args map[string]any, progress chan<- any) {
				// Use progress <- "message" or progress <- BinaryData{}
				progress <- "Starting..."
				h.DoInternalLogic(args["param1"].(string))
			},
		},
	}
}
```

## 3. Registration
Pass your handler instance to `mcpserve.NewHandler`. It is automatically discovered via reflection in [tools.go](../tools.go).
