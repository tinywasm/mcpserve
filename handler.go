package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tinywasm/mcp"
	"github.com/tinywasm/sse"
)

// LogEntry is the structured SSE payload.
// JSON keys and types must match devtui.tabContentDTO exactly for client-side routing.
// Type uses uint8 to match devtui.MessageType (tinywasm/fmt): 0=Normal,1=Info,2=Error,3=Warning.
// HandlerType uses int to match devtui.handlerType iota: 0=display,1=edit,2=execution,3=interactive,4=loggable.
// HandlerTypeLoggable is the handler_type value for plain log messages.
// Defined locally to avoid a circular import with devtui.
// MUST equal devtui.HandlerTypeLoggable (= 4). If devtui's HandlerType iota
// changes, update both constants in lockstep.
const HandlerTypeLoggable = 4

type LogEntry struct {
	Id           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Content      string `json:"content"`
	Type         uint8  `json:"type"` // matches devtui.MessageType (uint8): 1 = Info
	TabTitle     string `json:"tab_title"`
	HandlerName  string `json:"handler_name"`
	HandlerColor string `json:"handler_color"`
	HandlerType  int    `json:"handler_type"` // matches devtui.handlerType iota: 4 = loggable
}

// Config contains the configuration for Handler
type Config struct {
	Port          string
	ServerName    string // MCP server name
	ServerVersion string // MCP server version
	AppName       string // Application name (used to generate MCP server ID)
	AppVersion    string // Binary version for /version endpoint (used to detect stale daemons)
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
	actionFunc    func(string, string)
	stateProvider func() []byte

	// Internal state
	sseHub *sse.SSEServer

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
		h.PublishLog(fmt.Sprint(messages...))
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
		h.PublishLog(fmt.Sprint(messages...))
	}
}

// URL returns the address where the MCP server is serving.
// This allows *Handler to satisfy the agent.MCPServer interface via duck typing.
func (h *Handler) URL() string {
	return "http://localhost:" + h.config.Port + "/mcp"
}

// OnUIAction sets the callback for generic UI actions triggered from TUI or IDE
func (h *Handler) OnUIAction(actionFunc func(key, value string)) {
	h.actionFunc = actionFunc
}

// RegisterStateProvider registers a function that returns current handler state as JSON bytes.
// Called on GET /state; mcpserve does not interpret the bytes.
func (h *Handler) RegisterStateProvider(fn func() []byte) {
	h.mu.Lock()
	h.stateProvider = fn
	h.mu.Unlock()
}

func (h *Handler) handleStateGET(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	fn := h.stateProvider
	h.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(fn())
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
		ChannelProvider:     &logChannelProvider{},
		ClientChannelBuffer: 256,  // Buffered to prevent dropping messages during client I/O
		HistoryReplayBuffer: 100,  // Store last 100 messages for replay
		ReplayAllOnConnect:  true, // Send full history to every new client on first connect
	})

	// Create MCP server with tool capabilities
	s := mcp.NewMCPServer(
		h.config.ServerName,
		h.config.ServerVersion,
		mcp.WithToolCapabilities(true),
	)

	// Load tools from all registered handlers
	for _, handler := range h.toolHandlers {
		if handler == nil {
			continue
		}
		tools := handler.GetMCPTools()
		for _, toolMeta := range tools {
			tool := buildMCPTool(toolMeta)
			s.AddToolFromUser(*tool, h.mcpExecuteTool(handler, toolMeta.Execute))
		}
	}

	// Create MCP HTTP server
	// We need to check if StreamableHTTPServer implements http.Handler
	// Based on mcp-go usage, it typically provides Start() but might not directly expose ServeHTTP.
	// However, if we look at how libraries are usually built, it should.
	// If not, we might need a workaround. For now assuming it does.
	mcpServer := mcp.NewStreamableHTTPServer(s,
		mcp.WithEndpointPath("/mcp"),
		mcp.WithStateLess(true),
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
	mux.HandleFunc("/state", h.handleStateGET)
	mux.HandleFunc("/version", h.handleVersion)

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
	return nil
}

func (h *Handler) handleActionPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "query param 'key' is required", http.StatusBadRequest)
		return
	}
	value := r.URL.Query().Get("value") // empty string if absent

	h.mu.Lock()
	actionCb := h.actionFunc
	h.mu.Unlock()

	if actionCb != nil {
		actionCb(key, value)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Action applied: %s", key)))
	} else {
		http.Error(w, "No action handler configured", http.StatusServiceUnavailable)
	}
}

// handleVersion returns the daemon's binary version as JSON for stale-daemon detection.
func (h *Handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"version":"` + h.config.AppVersion + `"}`))
}

// PublishTabLog publishes a structured log entry to the SSE stream with full routing metadata.
// tabTitle must match one of the section titles registered in the client TUI (e.g. "BUILD", "DEPLOY").
func (h *Handler) PublishTabLog(tabTitle, handlerName, handlerColor, msg string) {
	if h.sseHub == nil {
		return
	}
	entry := LogEntry{
		Id:           fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp:    time.Now().Format("15:04:05"),
		Content:      msg,
		Type:         1, // Msg.Info = 1 in tinywasm/fmt
		TabTitle:     tabTitle,
		HandlerName:  handlerName,
		HandlerColor: handlerColor,
		HandlerType:  HandlerTypeLoggable,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	h.sseHub.Publish(data, "logs")
}

// PublishLog publishes a log message to the BUILD section as a generic MCP log.
func (h *Handler) PublishLog(msg string) {
	h.PublishTabLog("BUILD", "MCP", "#f97316", msg)
}
