package mcpserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// validateAppName checks if appName is valid (not empty or whitespace)
func validateAppName(appName string) error {
	if strings.TrimSpace(appName) == "" {
		return errors.New("appName cannot be empty")
	}
	return nil
}

// needsUpdate checks if the server entry needs to be updated by comparing URL and ExtraFields
func needsUpdate(existingEntry map[string]any, newEntry map[string]any, ide IDEInfo) bool {
	// Compare URL
	existingURL, _ := existingEntry[ide.URLKey].(string)
	newURL, _ := newEntry[ide.URLKey].(string)
	if existingURL != newURL {
		return true
	}
	// Compare ExtraFields
	for k, v := range ide.ExtraFields {
		if existingEntry[k] != v {
			return true
		}
	}
	return false
}

// writeMCPConfig is the unified config writer for all IDEs.
// It reads existing config, preserves all servers, and adds/updates our entry only if needed.
func writeMCPConfig(configPath string, appName string, mcpPort string, ide IDEInfo) (bool, error) {
	// Validate appName first
	if err := validateAppName(appName); err != nil {
		return false, err
	}

	// Read existing config as raw JSON to preserve all fields
	var rawConfig map[string]any

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			rawConfig = make(map[string]any)
		} else if os.IsPermission(err) {
			return false, nil // Silent failure
		} else {
			return false, err
		}
	} else {
		if err := json.Unmarshal(data, &rawConfig); err != nil {
			rawConfig = make(map[string]any)
		}
	}

	// Get or create the servers map (e.g., "servers" or "mcpServers")
	serversRaw, exists := rawConfig[ide.ServersKey]
	var servers map[string]any
	if exists {
		servers, _ = serversRaw.(map[string]any)
	}
	if servers == nil {
		servers = make(map[string]any)
	}

	// Cleanup duplicate URL entries (e.g., old "tinywasm-mcp" and new "tinywasm" with same URL)
	expectedURL := fmt.Sprintf("http://localhost:%s/mcp", mcpPort)
	serverID := strings.ToLower(appName)

	// Find all entries with our URL
	duplicatesRemoved := false
	for key, entry := range servers {
		if serverEntry, ok := entry.(map[string]any); ok {
			if url, _ := serverEntry[ide.URLKey].(string); url == expectedURL {
				// Remove any entry with our URL that is not our serverID
				if key != serverID {
					delete(servers, key)
					duplicatesRemoved = true
				}
			}
		}
	}

	// Build our server entry
	serverEntry := map[string]any{
		ide.URLKey: fmt.Sprintf("http://localhost:%s/mcp", mcpPort),
	}

	// Add extra fields (e.g., "type": "http", "autoStart": true)
	for k, v := range ide.ExtraFields {
		serverEntry[k] = v
	}

	// Check if entry already exists and is identical (skip if duplicates were cleaned)
	if !duplicatesRemoved {
		if existingEntry, hasEntry := servers[serverID]; hasEntry {
			if existing, ok := existingEntry.(map[string]any); ok {
				if !needsUpdate(existing, serverEntry, ide) {
					// Config is identical, no need to write
					return false, nil
				}
			}
		}
	}

	// Add/update our server entry
	servers[serverID] = serverEntry
	rawConfig[ide.ServersKey] = servers

	// Ensure inputs array exists for IDEs that need it
	if ide.HasInputs {
		if _, hasInputs := rawConfig["inputs"]; !hasInputs {
			rawConfig["inputs"] = []any{}
		}
	}

	// Marshal with tabs
	updatedData, err := json.MarshalIndent(rawConfig, "", "\t")
	if err != nil {
		return false, err
	}

	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		if os.IsPermission(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
