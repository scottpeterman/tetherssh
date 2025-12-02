// paths.go - Centralized application path management
// All config, data, and log files live under ~/.velocitycmd for consistency
// with the VelociTerm ecosystem
package main

import (
	"log"
	"os"
	"path/filepath"
)

// AppHomeDir is the name of the application's home directory
const AppHomeDir = ".velocitycmd"

// GetAppHome returns the application home directory (~/.velocitycmd)
// Creates it if it doesn't exist
func GetAppHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: Could not get user home directory: %v", err)
		return "."
	}

	appHome := filepath.Join(home, AppHomeDir)

	// Ensure it exists
	if err := os.MkdirAll(appHome, 0755); err != nil {
		log.Printf("Warning: Could not create app home directory %s: %v", appHome, err)
	}

	return appHome
}

// GetSessionsDir returns the sessions directory (~/.velocitycmd/sessions)
func GetSessionsDir() string {
	return filepath.Join(GetAppHome(), "sessions")
}

// GetLogsDir returns the logs directory (~/.velocitycmd/logs)
func GetLogsDir() string {
	return filepath.Join(GetAppHome(), "logs")
}

// GetSettingsPath returns the path to settings.json (~/.velocitycmd/settings.json)
func GetSettingsPath() string {
	return filepath.Join(GetAppHome(), "settings.json")
}

// GetSessionsFilePath returns the path to sessions.yaml (~/.velocitycmd/sessions/sessions.yaml)
func GetSessionsFilePath() string {
	return filepath.Join(GetSessionsDir(), "sessions.yaml")
}