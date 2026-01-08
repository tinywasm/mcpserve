package mcpserve

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	ExtraFields map[string]any // Additional fields like "type", "autoStart"
	HasInputs   bool           // VS Code has "inputs" array, Antigravity doesn't
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
	}

	for _, ide := range ides {
		basePath, err := ide.GetConfigDir()
		if err != nil {
			h.log("IDE config skip: %s - GetConfigDir error: %v", ide.Name, err)
			continue
		}

		// Create the directory if it doesn't exist
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			if err := os.MkdirAll(basePath, 0755); err != nil {
				h.log("IDE config skip: %s - MkdirAll error: %v", ide.Name, err)
				continue
			}
		}

		configPaths, err := findMCPConfigPaths(basePath, ide.ConfigFileName)
		if err != nil {
			h.log("IDE config skip: %s - findMCPConfigPaths error: %v", ide.Name, err)
			continue
		}

		for _, configPath := range configPaths {
			if err := writeMCPConfig(configPath, h.config.AppName, h.config.Port, ide); err != nil {
				h.log("IDE config error: %s - writeMCPConfig error: %v", ide.Name, err)
			} else {
				h.log("IDE config updated: %s at %s", ide.Name, configPath)
			}
		}
	}
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
