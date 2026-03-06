# Adding Tools to Handlers

To add MCP tools to a package (e.g., `client`, `browser`), implementation must follow the `ToolProvider` interface.

## 1. Import Metadata Types
Import `github.com/tinywasm/mcpserve` to use standard metadata structs.

```go
import "github.com/tinywasm/mcpserve"
```

Implement `GetMCPTools() []mcpserve.Tool` on your handler.

```go
func (h *MyHandler) GetMCPTools() []mcpserve.Tool {
	return []mcpserve.Tool{
		{
			Name: "tool_name",
			Description: "What it does",
			Parameters: []mcpserve.Parameter{
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
