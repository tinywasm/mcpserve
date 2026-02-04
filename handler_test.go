package mcpserve

import (
	"net"
	"testing"
	"time"
)

// mockHandler implements ToolProvider for testing
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

	mockHandlers := []ToolProvider{&mockHandler{}}
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

// TestToolMetadataRegistration verifies tool metadata extraction
func TestToolMetadataRegistration(t *testing.T) {
	mock := &mockHandler{}

	// Create a handler to use the method
	config := Config{Port: "3030", ServerName: "Test", ServerVersion: "1.0.0"}
	exitChan := make(chan bool, 1)
	_ = NewHandler(config, []ToolProvider{mock}, &mockTUI{}, exitChan)

	tools := mock.GetMCPToolsMetadata()

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
	_ = NewHandler(config, []ToolProvider{mock}, &mockTUI{}, make(chan bool))

	// Track if tool was executed
	executed := false
	mock.SetLog(func(messages ...any) {
		for _, msg := range messages {
			if msg == "test_tool executed" {
				executed = true
			}
		}
	})

	tools := mock.GetMCPToolsMetadata()
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

	handler := NewHandler(config, []ToolProvider{mock}, tui, exitChan)
	// handler.SetLog(func(messages ...any) { t.Log(messages...) })

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
}

// TestServeStopsOnValue verifies Serve shuts down when receiving a value on exitChan
func TestServeStopsOnValue(t *testing.T) {
	config := Config{
		Port:          "3032",
		ServerName:    "Test Stop Value",
		ServerVersion: "1.0.0",
	}

	exitChan := make(chan bool, 1)
	handler := NewHandler(config, nil, &mockTUI{}, exitChan)
	// handler.SetLog(func(messages ...any) { t.Log(messages...) })

	done := make(chan bool)
	go func() {
		handler.Serve()
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)

	// Send value instead of closing
	exitChan <- true

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for server to stop")
	}
}

// TestPortRelease verifies port is free after Stop
func TestPortRelease(t *testing.T) {
	config := Config{
		Port:          "3033",
		ServerName:    "Test Port Release",
		ServerVersion: "1.0.0",
	}

	exitChan := make(chan bool, 1)
	handler := NewHandler(config, nil, &mockTUI{}, exitChan)

	go handler.Serve()
	time.Sleep(200 * time.Millisecond)

	exitChan <- true
	time.Sleep(200 * time.Millisecond)

	// Wait for port to be released (up to 2 seconds)
	addr := ":3033"
	deadline := time.Now().Add(2 * time.Second)
	released := false
	for time.Now().Before(deadline) {
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			released = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !released {
		t.Errorf("Port 3033 not released after 2 seconds")
	}
}
