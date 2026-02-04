package mcpserve

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Config contains the configuration for Handler
type Config struct {
	Port          string
	ServerName    string // MCP server name
	ServerVersion string // MCP server version
	AppName       string // Application name (used to generate MCP server ID)
}

// TuiInterface defines what the MCP handler needs from the TUI
type TuiInterface interface {
	RefreshUI()
}

// Handler handles the Model Context Protocol server and configuration
type Handler struct {
	config       Config
	toolHandlers []ToolProvider // Handlers that implement ToolProvider interface
	tui          TuiInterface
	exitChan     chan bool
	log          func(messages ...any) // Private logger, set via SetLog
	ideStatus    string                // Summary of IDE configuration

	// Internal state
	httpServer any // *http.Server or compatible
	mu         sync.Mutex
	running    bool
}

// NewHandler creates a new MCP handler with minimal dependencies
func NewHandler(config Config, toolHandlers []ToolProvider, tui TuiInterface, exitChan chan bool) *Handler {
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
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	// Create MCP server with tool capabilities
	s := server.NewMCPServer(
		h.config.ServerName,
		h.config.ServerVersion,
		server.WithToolCapabilities(true),
	)

	// Load tools from all registered handlers
	for _, handler := range h.toolHandlers {
		if handler == nil {
			continue
		}
		tools := handler.GetMCPToolsMetadata()
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

	h.mu.Lock()
	h.httpServer = httpServer
	ideMsg := h.ideStatus
	h.mu.Unlock()

	// Consolidate startup messages into ONE log
	startupMsg := fmt.Sprintf("Started on :%s/mcp", h.config.Port)
	if ideMsg != "" {
		startupMsg = fmt.Sprintf("%s (%s)", startupMsg, ideMsg)
	}
	h.log(startupMsg)

	go func() {
		if err := httpServer.Start(":" + h.config.Port); err != nil && err != http.ErrServerClosed {
			h.log("MCP HTTP server stopped:", err)
		}
	}()

	// Wait for exit signal (value or close)
	<-h.exitChan

	// ALWAYS shutdown on exit
	h.Stop()
}

// Stop gracefully shuts down the MCP HTTP server
func (h *Handler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running || h.httpServer == nil {
		return nil
	}

	h.log("Shutting down MCP server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the exported Shutdown method of StreamableHTTPServer
	if h.httpServer != nil {
		if srv, ok := h.httpServer.(interface {
			Shutdown(context.Context) error
		}); ok {
			err := srv.Shutdown(ctx)
			if err != nil {
				h.log("Error shutting down MCP server:", err)
			}
		}
	}

	h.running = false
	h.httpServer = nil
	return nil
}
