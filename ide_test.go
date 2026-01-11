package mcpserve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper to create IDEInfo for testing
func testAntigravityIDE() IDEInfo {
	return IDEInfo{
		ID:         "antigravity",
		Name:       "Antigravity",
		ServersKey: "mcpServers",
		URLKey:     "serverUrl",
		HasInputs:  false,
	}
}

func testVSCodeIDE() IDEInfo {
	return IDEInfo{
		ID:          "vsc",
		Name:        "Visual Studio Code",
		ServersKey:  "servers",
		URLKey:      "url",
		ExtraFields: map[string]any{"type": "http", "autoStart": true},
		HasInputs:   true,
	}
}

// TestWriteMCPConfig_Antigravity verifies Antigravity-specific config format
func TestWriteMCPConfig_Antigravity(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("writeMCPConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var rawConfig map[string]any
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify mcpServers key
	if _, exists := rawConfig["mcpServers"]; !exists {
		t.Error("Should have mcpServers key")
	}

	// Verify no inputs for Antigravity
	if _, exists := rawConfig["inputs"]; exists {
		t.Error("Antigravity should NOT have inputs key")
	}

	servers := rawConfig["mcpServers"].(map[string]any)
	server := servers["tinywasm"].(map[string]any)

	if server["serverUrl"] != "http://localhost:3030/mcp" {
		t.Errorf("Wrong serverUrl: %v", server["serverUrl"])
	}

}

// TestWriteMCPConfig_VSCode verifies VS Code-specific config format
func TestWriteMCPConfig_VSCode(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp.json")

	err := writeMCPConfig(configPath, "tinywasm", "3030", testVSCodeIDE())
	if err != nil {
		t.Fatalf("writeMCPConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var rawConfig map[string]any
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify servers key
	if _, exists := rawConfig["servers"]; !exists {
		t.Error("Should have servers key")
	}

	// Verify inputs exists for VS Code
	if _, exists := rawConfig["inputs"]; !exists {
		t.Error("VS Code should have inputs key")
	}

	servers := rawConfig["servers"].(map[string]any)
	server := servers["tinywasm"].(map[string]any)

	if server["url"] != "http://localhost:3030/mcp" {
		t.Errorf("Wrong url: %v", server["url"])
	}
	if server["type"] != "http" {
		t.Errorf("Wrong type: %v", server["type"])
	}
	if server["autoStart"] != true {
		t.Errorf("Wrong autoStart: %v", server["autoStart"])
	}

}

// TestWriteMCPConfig_PreservesExistingServers verifies that other servers are preserved
func TestWriteMCPConfig_PreservesExistingServers(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Create existing config with google-maps (including env property)
	existingConfig := `{
	"mcpServers": {
		"google-maps-platform-code-assist": {
			"command": "npx",
			"args": ["-y", "@googlemaps/code-assist-mcp@latest"],
			"env": {
				"SOURCE": "antigravity"
			}
		},
		"other-server": {
			"serverUrl": "http://localhost:9999/mcp"
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0644); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Write tinywasm config
	err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("writeMCPConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var rawConfig map[string]map[string]any
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	mcpServers := rawConfig["mcpServers"]

	// Verify tinywasm was added
	if _, exists := mcpServers["tinywasm"]; !exists {
		t.Error("tinywasm should be present")
	}

	// Verify google-maps was preserved with all its properties
	googleMaps, exists := mcpServers["google-maps-platform-code-assist"]
	if !exists {
		t.Error("google-maps-platform-code-assist server should be preserved")
	} else {
		gm := googleMaps.(map[string]any)
		if _, hasEnv := gm["env"]; !hasEnv {
			t.Error("google-maps env property should be preserved")
		}
		if _, hasCommand := gm["command"]; !hasCommand {
			t.Error("google-maps command property should be preserved")
		}
		if _, hasArgs := gm["args"]; !hasArgs {
			t.Error("google-maps args property should be preserved")
		}
	}

	// Verify other-server was preserved
	if _, exists := mcpServers["other-server"]; !exists {
		t.Error("other-server should be preserved")
	}

}

// TestWriteMCPConfig_UpdatesExistingEntry verifies that existing tinywasm entry is updated
func TestWriteMCPConfig_UpdatesExistingEntry(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	existingConfig := `{
	"mcpServers": {
		"tinywasm": {
			"serverUrl": "http://localhost:9999/old"
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0644); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("writeMCPConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var rawConfig map[string]map[string]any
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	server := rawConfig["mcpServers"]["tinywasm"].(map[string]any)
	expectedURL := "http://localhost:3030/mcp"
	if server["serverUrl"] != expectedURL {
		t.Errorf("Expected URL '%s', got '%v'", expectedURL, server["serverUrl"])
	}

}

// TestFindMCPConfigPaths_NoProfiles verifies behavior when no profiles directory exists
func TestFindMCPConfigPaths_NoProfiles(t *testing.T) {
	tempDir := t.TempDir()

	paths, err := findMCPConfigPaths(tempDir, "mcp_config.json")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("Expected 1 path, got %d", len(paths))
	}

	expected := filepath.Join(tempDir, "mcp_config.json")
	if paths[0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, paths[0])
	}
}

// TestFindMCPConfigPaths_WithProfiles verifies behavior with profiles directory
func TestFindMCPConfigPaths_WithProfiles(t *testing.T) {
	tempDir := t.TempDir()

	profilesDir := filepath.Join(tempDir, "profiles")
	profile1 := filepath.Join(profilesDir, "profile1")
	profile2 := filepath.Join(profilesDir, "profile2")

	if err := os.MkdirAll(profile1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(profile2, 0755); err != nil {
		t.Fatal(err)
	}

	paths, err := findMCPConfigPaths(tempDir, "mcp_config.json")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("Expected 2 paths, got %d: %v", len(paths), paths)
	}
}

// TestFindMCPConfigPaths_DirectoryNotFound verifies error when directory doesn't exist
func TestFindMCPConfigPaths_DirectoryNotFound(t *testing.T) {
	_, err := findMCPConfigPaths("/nonexistent/path", "mcp_config.json")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

// TestWriteMCPConfig_EmptyAppName_ReturnsError verifies that empty appName is rejected
func TestWriteMCPConfig_EmptyAppName_ReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Test empty appName
	err := writeMCPConfig(configPath, "", "3030", testAntigravityIDE())
	if err == nil {
		t.Error("Expected error for empty appName")
	}

	// Verify file was NOT created
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("File should NOT have been created with empty appName")
	}

	// Test whitespace-only appName
	err = writeMCPConfig(configPath, "   ", "3030", testAntigravityIDE())
	if err == nil {
		t.Error("Expected error for whitespace-only appName")
	}
}

// TestWriteMCPConfig_NoWriteWhenIdentical verifies no file modification when config is identical
func TestWriteMCPConfig_NoWriteWhenIdentical(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Create initial config
	err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Get initial modification time
	initialStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	initialModTime := initialStat.ModTime()

	// Wait a bit to ensure any write would have different ModTime
	time.Sleep(10 * time.Millisecond)

	// Write same config again
	err = writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	// Get new modification time
	newStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat file after second write: %v", err)
	}

	// File should NOT have been modified
	if !newStat.ModTime().Equal(initialModTime) {
		t.Errorf("File was modified when config was identical. Initial: %v, New: %v",
			initialModTime, newStat.ModTime())
	}
}
