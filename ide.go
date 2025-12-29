package mcpserve

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// IDEInfo represents a supported IDE and its configuration path resolver
type IDEInfo struct {
	ID           string
	Name         string
	GetConfigDir func() (string, error)
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

// findMCPConfigPaths resolves all mcp.json file paths based on VS Code profile structure.
func findMCPConfigPaths(basePath string) ([]string, error) {
	// Check if the base directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil, errors.New("directory not found")
	}

	profilesPath := filepath.Join(basePath, "profiles")

	// Check if profiles directory exists
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		// No profiles, use base path
		return []string{filepath.Join(basePath, "mcp.json")}, nil
	}

	// Get all profile directories
	entries, err := os.ReadDir(profilesPath)
	if err != nil {
		return nil, err
	}

	configPaths := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			configPaths = append(configPaths, filepath.Join(profilesPath, entry.Name(), "mcp.json"))
		}
	}

	if len(configPaths) == 0 {
		return []string{filepath.Join(basePath, "mcp.json")}, nil
	}

	return configPaths, nil
}
