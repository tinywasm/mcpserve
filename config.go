package mcpserve

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// writeMCPConfig is the unified config writer for all IDEs.
// It reads existing config, preserves all servers, and adds/updates our entry.
func writeMCPConfig(configPath string, appName string, mcpPort string, ide IDEInfo) error {
	// Read existing config as raw JSON to preserve all fields
	var rawConfig map[string]any

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			rawConfig = make(map[string]any)
		} else if os.IsPermission(err) {
			return nil // Silent failure
		} else {
			return err
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

	// Build our server entry
	serverID := fmt.Sprintf("%s-mcp", strings.ToLower(appName))
	serverEntry := map[string]any{
		ide.URLKey: fmt.Sprintf("http://localhost:%s/mcp", mcpPort),
	}

	// Add extra fields (e.g., "type": "http", "autoStart": true)
	for k, v := range ide.ExtraFields {
		serverEntry[k] = v
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
		return err
	}

	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		if os.IsPermission(err) {
			return nil
		}
		return err
	}

	return nil
}
