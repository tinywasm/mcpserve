package mcpserve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWriteMCPConfig_CleansDuplicateURLEntries verifies cleanup of duplicate URL entries
func TestWriteMCPConfig_CleansDuplicateURLEntries(t *testing.T) {
	t.Run("Antigravity", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "mcp_config.json")

		// Real corrupted config from user's Antigravity (has "-mcp" and "tinywasm" with same URL)
		corruptedConfig := `{
	"mcpServers": {
		"-mcp": {
			"serverUrl": "http://localhost:3030/mcp"
		},
		"google-maps-platform-code-assist": {
			"args": ["-y", "@googlemaps/code-assist-mcp@latest"],
			"command": "npx",
			"disabled": true,
			"env": {
				"SOURCE": "antigravity"
			}
		},
		"tinywasm": {
			"serverUrl": "http://localhost:3030/mcp"
		}
	}
}`
		if err := os.WriteFile(configPath, []byte(corruptedConfig), 0644); err != nil {
			t.Fatalf("Failed to write corrupted config: %v", err)
		}

		// Run writeMCPConfig - should remove duplicate "-mcp"
		_, err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
		if err != nil {
			t.Fatalf("writeMCPConfig failed: %v", err)
		}

		// Read and validate JSON
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}
		var config map[string]map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Invalid JSON after update: %v", err)
		}

		// Verify "-mcp" was removed (duplicate URL)
		if _, exists := config["mcpServers"]["-mcp"]; exists {
			t.Error("Duplicate '-mcp' entry should have been removed")
		}

		// Verify valid servers preserved
		if _, exists := config["mcpServers"]["tinywasm"]; !exists {
			t.Error("tinywasm should exist")
		}
		if _, exists := config["mcpServers"]["google-maps-platform-code-assist"]; !exists {
			t.Error("google-maps-platform-code-assist should be preserved")
		}
	})

	t.Run("VSCode", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "mcp.json")

		// Real corrupted config from user's VS Code (has "-mcp" and "tinywasm" with same URL)
		corruptedConfig := `{
	"inputs": [],
	"servers": {
		"-mcp": {
			"autoStart": true,
			"type": "http",
			"url": "http://localhost:3030/mcp"
		},
		"cloudflare": {
			"args": ["mcp-remote", "https://docs.mcp.cloudflare.com/sse"],
			"command": "npx",
			"type": "stdio"
		},
		"tinywasm": {
			"autoStart": true,
			"type": "http",
			"url": "http://localhost:3030/mcp"
		}
	}
}`
		if err := os.WriteFile(configPath, []byte(corruptedConfig), 0644); err != nil {
			t.Fatalf("Failed to write corrupted config: %v", err)
		}

		// Run writeMCPConfig - should remove duplicate "-mcp"
		_, err := writeMCPConfig(configPath, "tinywasm", "3030", testVSCodeIDE())
		if err != nil {
			t.Fatalf("writeMCPConfig failed: %v", err)
		}

		// Read and validate JSON
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}
		var rawConfig map[string]any
		if err := json.Unmarshal(data, &rawConfig); err != nil {
			t.Fatalf("Invalid JSON after update: %v", err)
		}
		config := rawConfig["servers"].(map[string]any)

		// Verify "-mcp" was removed (duplicate URL)
		if _, exists := config["-mcp"]; exists {
			t.Error("Duplicate '-mcp' entry should have been removed")
		}

		// Verify valid servers preserved
		if _, exists := config["tinywasm"]; !exists {
			t.Error("tinywasm should exist")
		}
		if _, exists := config["cloudflare"]; !exists {
			t.Error("cloudflare should be preserved")
		}
	})

	t.Run("NoDuplicates_NoChange", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "mcp_config.json")

		// Config with different URLs (no duplicates)
		validConfig := `{
	"mcpServers": {
		"other-server": {
			"serverUrl": "http://localhost:9999/mcp"
		},
		"tinywasm": {
			"serverUrl": "http://localhost:3030/mcp"
		}
	}
}`
		if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		_, err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
		if err != nil {
			t.Fatalf("writeMCPConfig failed: %v", err)
		}

		// Read and validate JSON
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}
		var config map[string]map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Invalid JSON after update: %v", err)
		}

		// Both servers should exist (no cleanup needed)
		if _, exists := config["mcpServers"]["other-server"]; !exists {
			t.Error("other-server should be preserved (different URL)")
		}
		if _, exists := config["mcpServers"]["tinywasm"]; !exists {
			t.Error("tinywasm should exist")
		}
	})
}

// TestWriteMCPConfig_MigratesOldServerID verifies migration from old "tinywasm-mcp" to new "tinywasm"
func TestWriteMCPConfig_MigratesOldServerID(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Config with old "-mcp" suffix server ID (the old format)
	oldConfig := `{
	"mcpServers": {
		"tinywasm-mcp": {
			"serverUrl": "http://localhost:3030/mcp"
		},
		"google-maps": {
			"serverUrl": "http://localhost:9999/mcp"
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Run with appName "tinywasm" - now generates "tinywasm" (not "tinywasm-mcp")
	_, err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("writeMCPConfig failed: %v", err)
	}

	// Read and validate
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	var config map[string]map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Old "tinywasm-mcp" should be removed (same URL as new "tinywasm")
	if _, exists := config["mcpServers"]["tinywasm-mcp"]; exists {
		t.Error("Old 'tinywasm-mcp' should have been removed as duplicate URL")
	}

	// New "tinywasm" should exist
	if _, exists := config["mcpServers"]["tinywasm"]; !exists {
		t.Error("New 'tinywasm' should exist")
	}

	// Unrelated server should be preserved
	if _, exists := config["mcpServers"]["google-maps"]; !exists {
		t.Error("google-maps should be preserved (different URL)")
	}
}

// TestWriteMCPConfig_UpdatesOnlyWhenDifferent verifies file IS updated when config differs
func TestWriteMCPConfig_UpdatesOnlyWhenDifferent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Create initial config with port 9999
	_, err := writeMCPConfig(configPath, "tinywasm", "9999", testAntigravityIDE())
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Get initial modification time
	initialStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	initialModTime := initialStat.ModTime()

	// Wait to ensure ModTime would differ
	time.Sleep(10 * time.Millisecond)

	// Write config with different port (3030)
	_, err = writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
	if err != nil {
		t.Fatalf("Update write failed: %v", err)
	}

	// Get new modification time
	newStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// File SHOULD have been modified
	if newStat.ModTime().Equal(initialModTime) {
		t.Error("File should have been modified when config differs")
	}

	// Verify the URL was actually updated
	data, _ := os.ReadFile(configPath)
	var config map[string]map[string]any
	json.Unmarshal(data, &config)
	server := config["mcpServers"]["tinywasm"].(map[string]any)
	if server["serverUrl"] != "http://localhost:3030/mcp" {
		t.Errorf("URL not updated: %v", server["serverUrl"])
	}
}
