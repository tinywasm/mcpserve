package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Config contains the configuration for Handler
type Config struct {
	Port            string
	RootDir         string
	FrameworkName   string
	ServerPort      string
	ServerOutputDir string
	WebPublicDir    string
	Logger          func(messages ...any)
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
	}
}

// Serve starts the Model Context Protocol server for LLM integration via HTTP
func (h *Handler) Serve() {
	// Create MCP server with tool capabilities
	s := server.NewMCPServer(
		"TinyWasm - Full-stack Go+WASM Dev Environment (Server, WASM, Assets, Browser, Deploy)",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// === STATUS & MONITORING TOOLS ===

	s.AddTool(mcp.NewTool("golite_status",
		mcp.WithDescription("Get comprehensive status of TinyWasm full-stack dev environment: Go server (running/port), WASM compilation (output dir), browser (URL), and asset watching. Use this first to understand the current state of the development environment."),
	), h.mcpToolGetStatus)

	// === BUILD CONTROL TOOLS ===

	// Load tools from all registered handlers (using reflection)
	for _, handler := range h.toolHandlers {
		if handler == nil {
			continue
		}
		tools, err := mcpToolsFromHandler(handler)
		if err != nil {
			if h.config.Logger != nil {
				h.config.Logger("Warning: Failed to load tools from handler:", err)
			}
			continue
		}
		for _, toolMeta := range tools {
			tool := buildMCPTool(toolMeta)
			s.AddTool(*tool, h.mcpExecuteTool(toolMeta.Execute))
		}
	}

	// Start MCP HTTP server
	httpServer := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	h.server = httpServer

	if h.config.Logger != nil {
		h.config.Logger("Starting MCP HTTP server on port", h.config.Port)
		h.config.Logger("MCP endpoint: http://localhost:" + h.config.Port + "/mcp")
	}

	go func() {
		if err := httpServer.Start(":" + h.config.Port); err != nil {
			if h.config.Logger != nil {
				h.config.Logger("MCP HTTP server stopped:", err)
			}
		}
	}()

	_, ok := <-h.exitChan
	if !ok {
		if h.config.Logger != nil {
			h.config.Logger("Shutting down MCP server...")
		}
		ctx := context.Background()
		if err := httpServer.Shutdown(ctx); err != nil {
			if h.config.Logger != nil {
				h.config.Logger("Error shutting down MCP server:", err)
			}
		}
	}
}

// ConfigureIDEs automatically configures supported IDEs with this MCP server
func (h *Handler) ConfigureIDEs() {
	ides := []IDEInfo{
		{
			ID:           "vsc",
			Name:         "Visual Studio Code",
			GetConfigDir: getVSCodeConfigPath,
		},
		{
			ID:   "antigravity",
			Name: "Antigravity",
			GetConfigDir: func() (string, error) {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				return filepath.Join(homeDir, ".antigravity", "User"), nil
			},
		},
	}

	for _, ide := range ides {
		basePath, err := ide.GetConfigDir()
		if err != nil {
			continue
		}

		// Create the directory if it doesn't exist
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			if err := os.MkdirAll(basePath, 0755); err != nil {
				continue // Silent failure
			}
		}

		configPaths, err := findMCPConfigPaths(basePath)
		if err != nil {
			continue
		}

		for _, configPath := range configPaths {
			_ = updateMCPConfig(configPath, h.config.Port)
		}
	}
}

// === TOOL IMPLEMENTATIONS ===

func (h *Handler) mcpToolGetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := map[string]any{
		"framework": h.config.FrameworkName,
		"root_dir":  h.config.RootDir,
		"server": map[string]any{
			"running":    true,
			"port":       h.config.ServerPort,
			"output_dir": h.config.ServerOutputDir,
		},
		"wasm": map[string]any{
			"output_dir": h.config.WebPublicDir,
		},
		"browser": map[string]any{
			"url": fmt.Sprintf("http://localhost:%s", h.config.ServerPort),
		},
		"assets": map[string]any{
			"watching":   true,
			"public_dir": h.config.WebPublicDir,
		},
	}

	jsonData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("Failed to marshal status: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
