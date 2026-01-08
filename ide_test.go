package mcpserve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	server := servers["tinywasm-mcp"].(map[string]any)

	if server["serverUrl"] != "http://localhost:3030/mcp" {
		t.Errorf("Wrong serverUrl: %v", server["serverUrl"])
	}

	t.Logf("✓ Antigravity config:\n%s", string(data))
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
	server := servers["tinywasm-mcp"].(map[string]any)

	if server["url"] != "http://localhost:3030/mcp" {
		t.Errorf("Wrong url: %v", server["url"])
	}
	if server["type"] != "http" {
		t.Errorf("Wrong type: %v", server["type"])
	}
	if server["autoStart"] != true {
		t.Errorf("Wrong autoStart: %v", server["autoStart"])
	}

	t.Logf("✓ VS Code config:\n%s", string(data))
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

	// Verify tinywasm-mcp was added
	if _, exists := mcpServers["tinywasm-mcp"]; !exists {
		t.Error("tinywasm-mcp should be present")
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

	t.Logf("✓ All servers preserved:\n%s", string(data))
}

// TestWriteMCPConfig_UpdatesExistingEntry verifies that existing tinywasm entry is updated
func TestWriteMCPConfig_UpdatesExistingEntry(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	existingConfig := `{
	"mcpServers": {
		"tinywasm-mcp": {
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

	server := rawConfig["mcpServers"]["tinywasm-mcp"].(map[string]any)
	expectedURL := "http://localhost:3030/mcp"
	if server["serverUrl"] != expectedURL {
		t.Errorf("Expected URL '%s', got '%v'", expectedURL, server["serverUrl"])
	}

	t.Logf("✓ Entry updated correctly:\n%s", string(data))
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
