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

		// Real corrupted config from user's Antigravity (has "-mcp" and "tinywasm-mcp" with same URL)
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
		"tinywasm-mcp": {
			"serverUrl": "http://localhost:3030/mcp"
		}
	}
}`
		if err := os.WriteFile(configPath, []byte(corruptedConfig), 0644); err != nil {
			t.Fatalf("Failed to write corrupted config: %v", err)
		}

		// Run writeMCPConfig - should remove duplicate "-mcp"
		err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
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
		if _, exists := config["mcpServers"]["tinywasm-mcp"]; !exists {
			t.Error("tinywasm-mcp should exist")
		}
		if _, exists := config["mcpServers"]["google-maps-platform-code-assist"]; !exists {
			t.Error("google-maps-platform-code-assist should be preserved")
		}
	})

	t.Run("VSCode", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "mcp.json")

		// Real corrupted config from user's VS Code (has "-mcp" and "tinywasm-mcp" with same URL)
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
		"tinywasm-mcp": {
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
		err := writeMCPConfig(configPath, "tinywasm", "3030", testVSCodeIDE())
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
		if _, exists := config["tinywasm-mcp"]; !exists {
			t.Error("tinywasm-mcp should exist")
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
		"tinywasm-mcp": {
			"serverUrl": "http://localhost:3030/mcp"
		}
	}
}`
		if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		err := writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
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
		if _, exists := config["mcpServers"]["tinywasm-mcp"]; !exists {
			t.Error("tinywasm-mcp should exist")
		}
	})
}

// TestWriteMCPConfig_UpdatesOnlyWhenDifferent verifies file IS updated when config differs
func TestWriteMCPConfig_UpdatesOnlyWhenDifferent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp_config.json")

	// Create initial config with port 9999
	err := writeMCPConfig(configPath, "tinywasm", "9999", testAntigravityIDE())
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
	err = writeMCPConfig(configPath, "tinywasm", "3030", testAntigravityIDE())
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
	server := config["mcpServers"]["tinywasm-mcp"].(map[string]any)
	if server["serverUrl"] != "http://localhost:3030/mcp" {
		t.Errorf("URL not updated: %v", server["serverUrl"])
	}
}
