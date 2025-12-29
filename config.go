package mcpserve

import (
	"encoding/json"
	"fmt"
	"os"
)

// mcpConfig represents the structure of VS Code's mcp.json file
type mcpConfig struct {
	Servers map[string]mcpServerConfig `json:"servers"`
	Inputs  []any                      `json:"inputs"`
}

// mcpServerConfig represents a single MCP server configuration
type mcpServerConfig struct {
	URL     string   `json:"url,omitempty"`
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// updateMCPConfig reads, updates, and writes the mcp.json file.
// Adds or updates the "tinywasm-mcp" entry with the current configuration.
// Creates new file if it doesn't exist.
// Returns nil for permission errors (silent failure).
func updateMCPConfig(configPath string, mcpPort string) error {
	var config mcpConfig

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new config structure
			config = mcpConfig{
				Servers: make(map[string]mcpServerConfig),
				Inputs:  []any{},
			}
		} else if os.IsPermission(err) {
			// No permissions, return silently
			return nil
		} else {
			return err
		}
	} else {
		// Parse existing config
		if err := json.Unmarshal(data, &config); err != nil {
			// Invalid JSON, fail silently
			return nil
		}

		if config.Servers == nil {
			config.Servers = make(map[string]mcpServerConfig)
		}
		if config.Inputs == nil {
			config.Inputs = []any{}
		}
	}

	// Add/update TinyWasm MCP entry
	config.Servers["tinywasm-mcp"] = mcpServerConfig{
		URL:  fmt.Sprintf("http://localhost:%s/mcp", mcpPort),
		Type: "http",
	}

	// Marshal with proper formatting (tabs for consistency with VS Code)
	updatedData, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}

	// Write back (fail silently on permission errors)
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		if os.IsPermission(err) {
			return nil // Silent failure
		}
		return err
	}

	return nil
}
