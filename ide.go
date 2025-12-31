package mcpserve

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// IDEInfo represents a supported IDE and its configuration path resolver
type IDEInfo struct {
	ID             string
	Name           string
	GetConfigDir   func() (string, error)
	ConfigFileName string
}

// ConfigureIDEs automatically configures supported IDEs with this MCP server
func (h *Handler) ConfigureIDEs() {
	ides := []IDEInfo{
		{
			ID:             "vsc",
			Name:           "Visual Studio Code",
			GetConfigDir:   getVSCodeConfigPath,
			ConfigFileName: "mcp.json",
		},
		{
			ID:   "antigravity",
			Name: "Antigravity",
			GetConfigDir: func() (string, error) {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				// The correct path for Antigravity config is ~/.gemini/antigravity
				return filepath.Join(homeDir, ".gemini", "antigravity"), nil
			},
			ConfigFileName: "mcp_config.json",
		},
	}

	for _, ide := range ides {
		basePath, err := ide.GetConfigDir()
		if err != nil {
			continue
		}

		// Create the directory if it doesn't exist
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			if err := os.MkdirAll(basePath, 0755); err != nil {
				continue // Silent failure
			}
		}

		configPaths, err := findMCPConfigPaths(basePath, ide.ConfigFileName)
		if err != nil {
			continue
		}

		for _, configPath := range configPaths {
			_ = updateMCPConfig(configPath, h.config.Port)
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

// findMCPConfigPaths resolves all mcp.json (or specified) file paths based on IDE profile structure.
func findMCPConfigPaths(basePath string, configFileName string) ([]string, error) {
	// Check if the base directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil, errors.New("directory not found")
	}

	profilesPath := filepath.Join(basePath, "profiles")

	// Check if profiles directory exists
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		// No profiles, use base path
		return []string{filepath.Join(basePath, configFileName)}, nil
	}

	// Get all profile directories
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
