package mcpserve

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IDEInfo represents a supported IDE and its MCP configuration format
type IDEInfo struct {
	ID             string
	Name           string
	GetConfigDir   func() (string, error)
	ConfigFileName string

	// IDE-specific JSON format configuration
	ServersKey  string         // "servers" for VS Code, "mcpServers" for Antigravity
	URLKey      string         // "url" for VS Code, "serverUrl" for Antigravity
	ExtraFields  map[string]any // Additional fields like "type", "autoStart"
	HasInputs    bool           // VS Code has "inputs" array, Antigravity doesn't
	SkipProfiles bool           // true = single config file, no profile scanning
}

// ConfigureIDEs automatically configures supported IDEs with this MCP server
func (h *Handler) ConfigureIDEs() {
	ides := []IDEInfo{
		{
			ID:             "vsc",
			Name:           "Visual Studio Code",
			GetConfigDir:   getVSCodeConfigPath,
			ConfigFileName: "mcp.json",
			ServersKey:     "servers",
			URLKey:         "url",
			ExtraFields:    map[string]any{"type": "http", "autoStart": true},
			HasInputs:      true,
		},
		{
			ID:             "antigravity",
			Name:           "Antigravity",
			GetConfigDir:   getAntigravityConfigPath,
			ConfigFileName: "mcp_config.json",
			ServersKey:     "mcpServers",
			URLKey:         "serverUrl",
			ExtraFields:    nil,
			HasInputs:      false,
		},
		{
			ID:             "claude-code",
			Name:           "Claude Code",
			GetConfigDir:   getClaudeCodeConfigPath,
			ConfigFileName: ".claude.json",
			ServersKey:     "mcpServers",
			URLKey:         "url",
			ExtraFields:    map[string]any{"type": "http"},
			HasInputs:      false,
			SkipProfiles:   true,
		},
	}

	updatedIDEs := []string{}

	for _, ide := range ides {
		basePath, err := ide.GetConfigDir()
		if err != nil {
			// Silently skip if we can't get the config dir (e.g., unsupported OS)
			continue
		}

		var configPaths []string
		if ide.SkipProfiles {
			configPaths = []string{filepath.Join(basePath, ide.ConfigFileName)}
		} else {
			// Create the directory if it doesn't exist
			if _, err := os.Stat(basePath); os.IsNotExist(err) {
				if err := os.MkdirAll(basePath, 0755); err != nil {
					continue
				}
			}

			configPaths, err = findMCPConfigPaths(basePath, ide.ConfigFileName)
			if err != nil {
				continue
			}
		}

		ideUpdated := false
		for _, configPath := range configPaths {
			updated, err := writeMCPConfig(configPath, h.config.AppName, h.config.Port, ide)
			if err == nil && updated {
				ideUpdated = true
			}
		}
		if ideUpdated {
			updatedIDEs = append(updatedIDEs, ide.Name)
		}
	}

	totalIDEs := len(ides)
	status := fmt.Sprintf("%d of %d IDEs updated", len(updatedIDEs), totalIDEs)
	if len(updatedIDEs) > 0 {
		status = fmt.Sprintf("%s: %s", status, strings.Join(updatedIDEs, ", "))
	}

	h.mu.Lock()
	h.ideStatus = status
	h.mu.Unlock()
}

// getVSCodeConfigPath returns the platform-specific VS Code User directory path.
func getVSCodeConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "linux":
		return filepath.Join(homeDir, ".config", "Code", "User"), nil
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Code", "User"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", errors.New("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "Code", "User"), nil
	default:
		return "", errors.New("unsupported platform: " + runtime.GOOS)
	}
}

// getAntigravityConfigPath returns the Antigravity config directory path.
func getAntigravityConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".gemini", "antigravity"), nil
}

// getClaudeCodeConfigPath returns the home directory (Claude Code config is ~/.claude.json).
func getClaudeCodeConfigPath() (string, error) {
	return os.UserHomeDir()
}

// findMCPConfigPaths resolves all config file paths based on IDE profile structure.
func findMCPConfigPaths(basePath string, configFileName string) ([]string, error) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil, errors.New("directory not found")
	}

	profilesPath := filepath.Join(basePath, "profiles")

	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		return []string{filepath.Join(basePath, configFileName)}, nil
	}

	entries, err := os.ReadDir(profilesPath)
	if err != nil {
		return nil, err
	}

	configPaths := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			configPaths = append(configPaths, filepath.Join(profilesPath, entry.Name(), configFileName))
		}
	}

	if len(configPaths) == 0 {
		return []string{filepath.Join(basePath, configFileName)}, nil
	}

	return configPaths, nil
}
