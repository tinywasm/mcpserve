# Adding Tools to Handlers

To add MCP tools to a package (e.g., `client`, `browser`), implementation must follow the `ToolProvider` interface.

## 1. Import Metadata Types
Import `github.com/tinywasm/mcpserve` to use standard metadata structs.

```go
import "github.com/tinywasm/mcpserve"
```

Implement `GetMCPToolsMetadata() []mcpserve.ToolMetadata` on your handler.

```go
func (h *MyHandler) GetMCPToolsMetadata() []mcpserve.ToolMetadata {
	return []mcpserve.ToolMetadata{
		{
			Name: "tool_name",
			Description: "What it does",
			Parameters: []mcpserve.ParameterMetadata{
				{Name: "param1", Type: "string", Required: true},
			},
			Execute: func(args map[string]any) {
				// Access arguments directly
				param := args["param1"].(string)
				h.DoInternalLogic(param)
			},
		},
	}
}
```

## 3. Registration
Pass your handler instance to `mcpserve.NewHandler`. It must implement the `ToolProvider` interface.
