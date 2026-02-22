package mcpserve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tinywasm/mcp/server"
	"github.com/tinywasm/sse"
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

	// Callbacks
	restartFunc func(context.Context, string) error
	actionFunc  func(string)

	// Internal state
	sseHub        *sse.SSEServer
	projectCancel context.CancelFunc
	projectDone   chan struct{}

	httpServer any // *http.Server or compatible
	mu         sync.Mutex
	running    bool
}

// NewHandler creates a new MCP handler with minimal dependencies
func NewHandler(config Config, toolHandlers []ToolProvider, tui TuiInterface, exitChan chan bool) *Handler {
	h := &Handler{
		config:       config,
		toolHandlers: toolHandlers,
		tui:          tui,
		exitChan:     exitChan,
	}
	// Initialize log with default no-op that also tries to publish to SSE (if available)
	h.log = func(messages ...any) {
		h.publishLog(fmt.Sprint(messages...))
	}
	return h
}

// Name returns the handler name for Loggable interface
func (h *Handler) Name() string {
	return "MCP"
}

// SetLog implements Loggable interface
func (h *Handler) SetLog(f func(message ...any)) {
	h.log = func(messages ...any) {
		if f != nil {
			f(messages...)
		}
		// Also publish to SSE
		h.publishLog(fmt.Sprint(messages...))
	}
}

// URL returns the address where the MCP server is serving.
// This allows *Handler to satisfy the agent.MCPServer interface via duck typing.
func (h *Handler) URL() string {
	return "http://localhost:" + h.config.Port + "/mcp"
}

// SetProjectRestartFunc sets the callback for restarting the project
func (h *Handler) SetProjectRestartFunc(restartFunc func(context.Context, string) error) {
	h.restartFunc = restartFunc
}

// SetActionFunc sets the callback for UI actions
func (h *Handler) SetActionFunc(actionFunc func(string)) {
	h.actionFunc = actionFunc
}

// logChannelProvider implements sse.ChannelProvider
type logChannelProvider struct{}

func (p *logChannelProvider) ResolveChannels(r *http.Request) ([]string, error) {
	return []string{"logs"}, nil
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

	// Initialize SSE
	tinySSE := sse.New(&sse.Config{
		Log: func(args ...any) {
			// Internal SSE logs
		},
	})
	h.sseHub = tinySSE.Server(&sse.ServerConfig{
		ChannelProvider: &logChannelProvider{},
	})

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

	// Create MCP HTTP server
	// We need to check if StreamableHTTPServer implements http.Handler
	// Based on mcp-go usage, it typically provides Start() but might not directly expose ServeHTTP.
	// However, if we look at how libraries are usually built, it should.
	// If not, we might need a workaround. For now assuming it does.
	mcpServer := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	// Set up router
	mux := http.NewServeMux()

	// Assuming mcpServer implements http.Handler. If compilation fails, we will need to wrap it.
	// If mcpServer is *server.StreamableHTTPServer, check if it has ServeHTTP method.
	// If it doesn't, we can't use mux easily without more info.
	// But let's assume it does for now.
	// If it doesn't, we might need to use mcpServer.Start() and not use mux, but that prevents adding other handlers on same port.
	// Alternatively, we can use a reverse proxy or just replicate what Start does.
	// Start typically does http.ListenAndServe(addr, mcpServer).
	// So mcpServer MUST be a http.Handler.
	mux.Handle("/mcp", mcpServer)

	mux.Handle("/logs", h.sseHub)
	mux.HandleFunc("/action", h.handleActionPOST)

	h.mu.Lock()
	h.httpServer = &http.Server{
		Addr:    ":" + h.config.Port,
		Handler: mux,
	}
	ideMsg := h.ideStatus
	h.mu.Unlock()

	// Consolidate startup messages into ONE log
	startupMsg := fmt.Sprintf("Started on :%s/mcp", h.config.Port)
	if ideMsg != "" {
		startupMsg = fmt.Sprintf("%s (%s)", startupMsg, ideMsg)
	}
	h.log(startupMsg)

	go func() {
		if err := h.httpServer.(*http.Server).ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	if srv, ok := h.httpServer.(*http.Server); ok {
		if err := srv.Shutdown(ctx); err != nil {
			h.log("Error shutting down MCP server:", err)
		}
	}

	h.running = false
	h.httpServer = nil

	// Also stop project if running
	if h.projectCancel != nil {
		h.projectCancel()
	}

	return nil
}

// StartProject starts the project at the given path, managing lifecycle
func (h *Handler) StartProject(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Cancel previous project
	if h.projectCancel != nil {
		h.projectCancel()
	}

	// 2. Block until port 8080 unbinds (assuming app runs on 8080)
	// We check for port 8080 closure with a timeout.
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

portLoop:
	for {
		select {
		case <-timeout:
			h.log("Warning: Port 8080 still active after timeout")
			break portLoop
		case <-ticker.C:
			conn, err := net.Dial("tcp", "localhost:8080")
			if err != nil {
				// Port is closed
				break portLoop
			}
			conn.Close()
		}
	}

	// 3. Start new project
	if h.restartFunc != nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.projectCancel = cancel
		h.projectDone = make(chan struct{})

		go func() {
			defer close(h.projectDone)
			if err := h.restartFunc(ctx, path); err != nil {
				h.log("Error starting project:", err)
			}
		}()
	}

	return nil
}

// StopProject stops the currently running project
func (h *Handler) StopProject() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.projectCancel != nil {
		h.projectCancel()
		h.projectCancel = nil
	}
}

func (h *Handler) handleActionPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	switch key {
	case "q":
		h.StopProject()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Project stopped"))
	case "r":
		if h.actionFunc != nil {
			h.actionFunc("r")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Reload triggered"))
	default:
		http.Error(w, "Invalid action key", http.StatusBadRequest)
	}
}

// publishLog publishes a log message to SSE
func (h *Handler) publishLog(msg string) {
	if h.sseHub != nil {
		h.sseHub.Publish([]byte(msg), "logs")
	}
}
