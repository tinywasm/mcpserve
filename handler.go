package mcpserve

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"
	"unsafe"

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
	toolHandlers []any // Handlers that implement GetMCPToolsMetadata (discovered via reflection)
	tui          TuiInterface
	exitChan     chan bool
	log          func(messages ...any) // Private logger, set via SetLog

	// Internal state
	httpServer any // *http.Server or compatible
	mu         sync.Mutex
	running    bool
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

	// Load tools from all registered handlers (using reflection)
	for _, handler := range h.toolHandlers {
		if handler == nil {
			continue
		}
		tools, err := h.mcpToolsFromHandler(handler)
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

	h.mu.Lock()
	h.httpServer = httpServer
	h.mu.Unlock()

	h.log("Starting MCP HTTP server on port", h.config.Port)
	h.log("MCP endpoint: http://localhost:" + h.config.Port + "/mcp")

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

	// Use reflection to call Shutdown if it exists (defensive against mcp-go changes)
	if h.httpServer != nil {
		// Try direct interface assertion first
		if srv, ok := h.httpServer.(interface {
			Shutdown(context.Context) error
			Close() error
		}); ok {
			err := srv.Shutdown(ctx)
			if err != nil {
				h.log("Error shutting down MCP server:", err)
			}
			srv.Close()
		} else {
			// Try to find an unexported 'server' field of type *http.Server
			v := reflect.ValueOf(h.httpServer)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}
			if v.Kind() == reflect.Struct {
				f := v.FieldByName("httpServer")
				if !f.IsValid() {
					f = v.FieldByName("server")
				}
				if !f.IsValid() {
					f = v.FieldByName("Server")
				}

				if f.IsValid() {
					// Access unexported field using unsafe
					ptr := unsafe.Pointer(f.UnsafeAddr())

					// We suspect it's an *http.Server or something that implements Shutdown
					// If it's *http.Server, we can cast it directly
					if f.Type().String() == "*http.Server" {
						srv := (*http.Server)(*(*unsafe.Pointer)(ptr))
						if srv != nil {
							err := srv.Shutdown(ctx)
							if err != nil {
								h.log("Error shutting down MCP server via unsafe field:", err)
							}
							srv.Close()
						}
					}
				}
			}
		}
	}

	h.running = false
	h.httpServer = nil
	return nil
}
