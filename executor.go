package mcpserve

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// BinaryData represents binary response from tools (imported from handlers)
type BinaryData struct {
	MimeType string
	Data     []byte
}

// mcpExecuteTool creates a GENERIC tool executor that works for ANY handler tool
// It extracts args, collects progress, executes the tool, and returns results
// NO domain-specific logic here - handlers provide their own Execute functions
// mcpExecuteTool creates a GENERIC tool executor that works for ANY handler tool
// It extracts args, collects logs via SetLog, executes the tool, and returns results
func (h *Handler) mcpExecuteTool(targetHandler any, executor ToolExecutor) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 1. Extract arguments (generic)
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			args = make(map[string]any)
		}

		// 2. Setup capturing logger if handler is Loggable
		messages := []string{}
		var binaryResponse *BinaryData

		if loggable, ok := targetHandler.(Loggable); ok {
			// Inject temporary capturing logger
			loggable.SetLog(func(message ...any) {
				if len(message) == 0 {
					return
				}

				for _, m := range message {
					switch v := m.(type) {
					case BinaryData:
						binaryResponse = &v
					case string:
						messages = append(messages, v)
					default:
						// Convert other types to string
						messages = append(messages, fmt.Sprintf("%v", v))
					}
				}
			})
		}

		// 3. Execute handler-specific logic
		executor(args)

		// 4. Refresh UI (generic)
		if h.tui != nil {
			h.tui.RefreshUI()
		}

		// 5. Handle binary response (if present) - prioritize over text
		if binaryResponse != nil {
			base64Data := base64.StdEncoding.EncodeToString(binaryResponse.Data)
			textSummary := ""
			if len(messages) > 0 {
				textSummary = strings.Join(messages, "\n")
			}
			return mcp.NewToolResultImage(textSummary, base64Data, binaryResponse.MimeType), nil
		}

		// 6. Return text messages (if no binary)
		if len(messages) == 0 {
			return mcp.NewToolResultText("Operation completed successfully"), nil
		}

		return mcp.NewToolResultText(strings.Join(messages, "\n")), nil
	}
}
