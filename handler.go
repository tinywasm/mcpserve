package mcpserve

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

// Config contains the configuration for Handler
type Config struct {
	Port          string
	ServerName    string // MCP server name
	ServerVersion string // MCP server version
}

// TuiInterface defines what the MCP handler needs from the TUI
type TuiInterface interface {
	RefreshUI()
}

// Handler handles the Model Context Protocol server and configuration
type Handler struct {
	config       Config
	toolHandlers []any // Handlers that implement GetMCPToolsMetadata (discovered via reflection)
	tui          TuiInterface
	exitChan     chan bool
	log          func(messages ...any) // Private logger, set via SetLog

	// Internal state
	server any
}

// NewHandler creates a new MCP handler with minimal dependencies
func NewHandler(config Config, toolHandlers []any, tui TuiInterface, exitChan chan bool) *Handler {
	return &Handler{
		config:       config,
		toolHandlers: toolHandlers,
		tui:          tui,
		exitChan:     exitChan,
		log:          func(messages ...any) {}, // No-op logger by default
	}
}

// Name returns the handler name for Loggable interface
func (h *Handler) Name() string {
	return "MCP"
}

// SetLog implements Loggable interface
func (h *Handler) SetLog(f func(message ...any)) {
	if f != nil {
		h.log = f
	}
}

// Serve starts the Model Context Protocol server for LLM integration via HTTP
func (h *Handler) Serve() {
	// Create MCP server with tool capabilities
	s := server.NewMCPServer(
		h.config.ServerName,
		h.config.ServerVersion,
		server.WithToolCapabilities(true),
	)

	// Load tools from all registered handlers (using reflection)

	// Load tools from all registered handlers (using reflection)
	for _, handler := range h.toolHandlers {
		if handler == nil {
			continue
		}
		tools, err := mcpToolsFromHandler(handler)
		if err != nil {
			h.log(fmt.Sprintf("Warning: Failed to load tools from handler %T: %v", handler, err))
			continue
		}
		for _, toolMeta := range tools {
			tool := buildMCPTool(toolMeta)
			s.AddTool(*tool, h.mcpExecuteTool(handler, toolMeta.Execute))
		}
	}

	// Start MCP HTTP server
	httpServer := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	h.server = httpServer

	h.log("Starting MCP HTTP server on port", h.config.Port)
	h.log("MCP endpoint: http://localhost:" + h.config.Port + "/mcp")

	go func() {
		if err := httpServer.Start(":" + h.config.Port); err != nil {
			h.log("MCP HTTP server stopped:", err)
		}
	}()

	_, ok := <-h.exitChan
	if !ok {
		h.log("Shutting down MCP server...")
		ctx := context.Background()
		if err := httpServer.Shutdown(ctx); err != nil {
			h.log("Error shutting down MCP server:", err)
		}
	}
}
