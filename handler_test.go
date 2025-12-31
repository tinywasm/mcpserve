package mcpserve

import (
	"testing"
	"time"
)

// mockHandler implements GetMCPToolsMetadata for testing
type mockHandler struct {
	log func(message ...any)
}

func (m *mockHandler) GetMCPToolsMetadata() []ToolMetadata {
	return []ToolMetadata{
		{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters:  []ParameterMetadata{},
			Execute: func(args map[string]any) {
				if m.log != nil {
					m.log("test_tool executed")
				}
			},
		},
	}
}

func (m *mockHandler) SetLog(f func(message ...any)) {
	m.log = f
}

// mockTUI implements TuiInterface for testing
type mockTUI struct{}

func (m *mockTUI) RefreshUI() {}

// TestNewHandler verifies Handler creation
func TestNewHandler(t *testing.T) {
	config := Config{
		Port:          "3030",
		ServerName:    "Test Server",
		ServerVersion: "1.0.0",
	}

	mockHandlers := []any{&mockHandler{}}
	exitChan := make(chan bool, 1)
	tui := &mockTUI{}

	handler := NewHandler(config, mockHandlers, tui, exitChan)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.Name() != "MCP" {
		t.Errorf("Expected name 'MCP', got '%s'", handler.Name())
	}

	if len(handler.toolHandlers) != 1 {
		t.Errorf("Expected 1 tool handler, got %d", len(handler.toolHandlers))
	}
}

// TestToolDiscovery verifies tool metadata extraction
func TestToolDiscovery(t *testing.T) {
	mock := &mockHandler{}

	// Create a handler to use the method
	config := Config{Port: "3030", ServerName: "Test", ServerVersion: "1.0.0"}
	exitChan := make(chan bool, 1)
	handler := NewHandler(config, []any{mock}, &mockTUI{}, exitChan)

	tools, err := handler.mcpToolsFromHandler(mock)

	if err != nil {
		t.Fatalf("Failed to extract tools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tool.Name)
	}

	if tool.Description != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", tool.Description)
	}
}

// TestToolExecution verifies tool execution directly
func TestToolExecution(t *testing.T) {
	mock := &mockHandler{}

	// Create a handler to use the method
	config := Config{Port: "3030", ServerName: "Test", ServerVersion: "1.0.0"}
	exitChan := make(chan bool, 1)
	handler := NewHandler(config, []any{mock}, &mockTUI{}, exitChan)

	tools, err := handler.mcpToolsFromHandler(mock)
	if err != nil {
		t.Fatalf("Failed to extract tools: %v", err)
	}

	// Track if tool was executed
	executed := false
	mock.SetLog(func(messages ...any) {
		for _, msg := range messages {
			if msg == "test_tool executed" {
				executed = true
			}
		}
	})

	// Execute tool directly
	tools[0].Execute(map[string]any{})

	if !executed {
		t.Error("Tool was not executed")
	}
}

// TestServeStartsServer verifies Serve starts without error
func TestServeStartsServer(t *testing.T) {
	config := Config{
		Port:          "3031", // Use different port to avoid conflicts
		ServerName:    "Test Server",
		ServerVersion: "1.0.0",
	}

	mock := &mockHandler{}
	exitChan := make(chan bool, 1)
	tui := &mockTUI{}

	handler := NewHandler(config, []any{mock}, tui, exitChan)
	handler.SetLog(func(messages ...any) { t.Log(messages...) })

	// Start server in goroutine
	serverStarted := make(chan bool, 1)
	go func() {
		serverStarted <- true
		handler.Serve()
	}()

	// Wait for goroutine to start
	<-serverStarted

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Signal exit
	close(exitChan)

	// Give server time to shutdown
	time.Sleep(200 * time.Millisecond)

	// If we reach here without panic, test passes
	t.Log("✓ Server started and stopped successfully")
}
