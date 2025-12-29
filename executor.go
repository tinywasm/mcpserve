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
func (h *Handler) mcpExecuteTool(executor ToolExecutor) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 1. Extract arguments (generic)
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			// Handle case where no arguments are provided
			args = make(map[string]any)
		}

		// 2. Create channel and collect responses (text or binary)
		progressChan := make(chan any, 10) // Buffered to avoid blocking
		messages := []string{}
		var binaryResponse *BinaryData
		done := make(chan bool)

		// 3. Collect progress messages in goroutine
		go func() {
			for msg := range progressChan {
				switch v := msg.(type) {
				case string:
					messages = append(messages, v)
				case BinaryData:
					binaryResponse = &v
				default:
					// Fallback: convert to string
					messages = append(messages, fmt.Sprintf("%v", v))
				}
			}
			done <- true
		}()

		// 4. Execute handler-specific logic (sends messages to channel)
		executor(args, progressChan)
		close(progressChan)

		// 5. Wait for collection to finish
		<-done

		// 6. Refresh UI (generic)
		if h.tui != nil {
			h.tui.RefreshUI()
		}

		// 7. Handle binary response (if present) - prioritize over text
		if binaryResponse != nil {
			// Encode binary data to base64 for MCP transmission
			base64Data := base64.StdEncoding.EncodeToString(binaryResponse.Data)

			// Build text summary from collected messages
			textSummary := ""
			if len(messages) > 0 {
				textSummary = strings.Join(messages, "\n")
			}

			// Return as ImageContent so VS Code can render it properly
			return mcp.NewToolResultImage(textSummary, base64Data, binaryResponse.MimeType), nil
		}

		// 8. Return text messages (if no binary)
		if len(messages) == 0 {
			return mcp.NewToolResultText("Operation completed successfully"), nil
		}

		return mcp.NewToolResultText(strings.Join(messages, "\n")), nil
	}
}
