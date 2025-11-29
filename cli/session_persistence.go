// session_persistence.go - YAML session file loading and saving
// Compatible with termtel sessions.yaml format, extended for TetherSSH
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// SessionFolder represents a folder/group of sessions in the YAML file
type SessionFolder struct {
	FolderName string        `yaml:"folder_name"`
	Sessions   []SessionYAML `yaml:"sessions"`
}

// SessionYAML represents a session entry in the YAML file
// Extended from termtel format to include auth settings
type SessionYAML struct {
	// Display and connection
	DisplayName string `yaml:"display_name"`
	Host        string `yaml:"host"`
	Port        string `yaml:"port"`

	// Authentication (TetherSSH extensions)
	Username      string `yaml:"username,omitempty"`
	AuthType      string `yaml:"auth_type,omitempty"`      // "password", "publickey", "keyboard-interactive"
	KeyPath       string `yaml:"key_path,omitempty"`       // Path to private key
	KeyPassphrase string `yaml:"key_passphrase,omitempty"` // Passphrase for encrypted keys (consider security implications)

	// Device info (termtel compatibility)
	DeviceType      string `yaml:"DeviceType,omitempty"`
	Model           string `yaml:"Model,omitempty"`
	SerialNumber    string `yaml:"SerialNumber,omitempty"`
	SoftwareVersion string `yaml:"SoftwareVersion,omitempty"`
	Vendor          string `yaml:"Vendor,omitempty"`
	CredsID         string `yaml:"credsid,omitempty"`
}

// SessionStore handles loading and saving sessions
type SessionStore struct {
	filePath string
	folders  []SessionFolder
}

// NewSessionStore creates a new session store
func NewSessionStore(filePath string) *SessionStore {
	return &SessionStore{
		filePath: filePath,
		folders:  []SessionFolder{},
	}
}

// DefaultSessionPath returns the default path for the sessions file
// Priority: ./sessions/sessions.yaml (app working directory)
// If not found, creates a stub file there
func DefaultSessionPath() string {
	// Always use app's working directory
	sessionDir := "sessions"
	sessionFile := filepath.Join(sessionDir, "sessions.yaml")

	// Check if it exists
	if _, err := os.Stat(sessionFile); err == nil {
		log.Printf("Found sessions file: %s", sessionFile)
		return sessionFile
	}

	// Doesn't exist - create stub
	log.Printf("Sessions file not found, creating stub: %s", sessionFile)
	if err := createStubSessionFile(sessionDir, sessionFile); err != nil {
		log.Printf("Warning: Could not create stub sessions file: %v", err)
	}

	return sessionFile
}

// createStubSessionFile creates a starter sessions.yaml with example entries
func createStubSessionFile(dir, filePath string) error {
	// Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Create stub content with example sessions
	stub := []SessionFolder{
		{
			FolderName: "Examples",
			Sessions: []SessionYAML{
				{
					DisplayName: "Example-Server",
					Host:        "192.168.1.1",
					Port:        "22",
					Username:    "admin",
					AuthType:    "password",
					DeviceType:  "linux",
				},
				{
					DisplayName: "Example-SSH-Key",
					Host:        "192.168.1.2",
					Port:        "22",
					Username:    "admin",
					AuthType:    "publickey",
					KeyPath:     "~/.ssh/id_rsa",
					DeviceType:  "linux",
				},
			},
		},
		{
			FolderName: "Lab",
			Sessions:   []SessionYAML{},
		},
		{
			FolderName: "Production",
			Sessions:   []SessionYAML{},
		},
	}

	data, err := yaml.Marshal(stub)
	if err != nil {
		return fmt.Errorf("failed to marshal stub: %w", err)
	}

	// Add header comment
	header := []byte("# TetherSSH Sessions File\n# Edit with the Session Manager (gear icon) or manually\n#\n# Auth types: password, publickey, keyboard-interactive\n# Key path supports ~ expansion (e.g., ~/.ssh/id_rsa)\n#\n# Format is compatible with termtel sessions.yaml\n\n")
	data = append(header, data...)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write stub file: %w", err)
	}

	log.Printf("Created stub sessions file: %s", filePath)
	return nil
}

// authTypeToString converts AuthMethod enum to string for YAML
func authTypeToString(auth AuthMethod) string {
	switch auth {
	case AuthPublicKey:
		return "publickey"
	case AuthKeyboardInteractive:
		return "keyboard-interactive"
	case AuthAgent:
		return "agent"
	default:
		return "password"
	}
}

// stringToAuthType converts string from YAML to AuthMethod enum
func stringToAuthType(s string) AuthMethod {
	switch s {
	case "publickey", "public_key", "key":
		return AuthPublicKey
	case "keyboard-interactive", "keyboard_interactive", "mfa":
		return AuthKeyboardInteractive
	case "agent":
		return AuthAgent
	default:
		return AuthPassword
	}
}

// Load reads sessions from the YAML file
func (s *SessionStore) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Sessions file not found: %s (will create on save)", s.filePath)
			return nil
		}
		return fmt.Errorf("failed to read sessions file: %w", err)
	}

	if err := yaml.Unmarshal(data, &s.folders); err != nil {
		return fmt.Errorf("failed to parse sessions YAML: %w", err)
	}

	log.Printf("Loaded %d folders from %s", len(s.folders), s.filePath)
	for _, folder := range s.folders {
		log.Printf("  Folder '%s': %d sessions", folder.FolderName, len(folder.Sessions))
	}

	return nil
}

// Save writes sessions to the YAML file
func (s *SessionStore) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	data, err := yaml.Marshal(s.folders)
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	// Add header comment
	header := []byte("# TetherSSH Sessions File\n# Edit with the Session Manager (gear icon) or manually\n#\n# Auth types: password, publickey, keyboard-interactive\n# Key path supports ~ expansion (e.g., ~/.ssh/id_rsa)\n#\n# Format is compatible with termtel sessions.yaml\n\n")
	data = append(header, data...)

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sessions file: %w", err)
	}

	log.Printf("Saved %d folders to %s", len(s.folders), s.filePath)
	return nil
}

// GetFolders returns all folder names
func (s *SessionStore) GetFolders() []string {
	names := make([]string, len(s.folders))
	for i, folder := range s.folders {
		names[i] = folder.FolderName
	}
	return names
}

// GetSessions returns all sessions as SessionInfo structs
func (s *SessionStore) GetSessions() []SessionInfo {
	var sessions []SessionInfo

	for _, folder := range s.folders {
		for i, sess := range folder.Sessions {
			sessions = append(sessions, s.yamlToSessionInfo(folder.FolderName, i, sess))
		}
	}

	return sessions
}

// yamlToSessionInfo converts a SessionYAML to SessionInfo
func (s *SessionStore) yamlToSessionInfo(folderName string, index int, sess SessionYAML) SessionInfo {
	// Parse port
	port := 22
	if sess.Port != "" {
		if p, err := strconv.Atoi(sess.Port); err == nil {
			port = p
		}
	}

	// Create unique ID from folder + index
	id := fmt.Sprintf("%s-%d", folderName, index)

	return SessionInfo{
		ID:            id,
		Name:          sess.DisplayName,
		Host:          sess.Host,
		Port:          port,
		Username:      sess.Username,
		AuthType:      stringToAuthType(sess.AuthType),
		KeyPath:       sess.KeyPath,
		KeyPassphrase: sess.KeyPassphrase,
		Group:         folderName,
		// Extended fields for device info
		DeviceType: sess.DeviceType,
		Vendor:     sess.Vendor,
		Model:      sess.Model,
		CredsID:    sess.CredsID,
	}
}

// sessionInfoToYAML converts a SessionInfo to SessionYAML
func (s *SessionStore) sessionInfoToYAML(session SessionInfo) SessionYAML {
	return SessionYAML{
		DisplayName:   session.Name,
		Host:          session.Host,
		Port:          strconv.Itoa(session.Port),
		Username:      session.Username,
		AuthType:      authTypeToString(session.AuthType),
		KeyPath:       session.KeyPath,
		KeyPassphrase: session.KeyPassphrase,
		DeviceType:    session.DeviceType,
		Vendor:        session.Vendor,
		Model:         session.Model,
		CredsID:       session.CredsID,
	}
}

// GetSessionsByFolder returns sessions organized by folder
func (s *SessionStore) GetSessionsByFolder() map[string][]SessionInfo {
	result := make(map[string][]SessionInfo)

	for _, folder := range s.folders {
		var sessions []SessionInfo
		for i, sess := range folder.Sessions {
			sessions = append(sessions, s.yamlToSessionInfo(folder.FolderName, i, sess))
		}
		result[folder.FolderName] = sessions
	}

	return result
}

// AddSession adds a new session to a folder
func (s *SessionStore) AddSession(folderName string, session SessionInfo) {
	// Find or create folder
	var targetFolder *SessionFolder
	for i := range s.folders {
		if s.folders[i].FolderName == folderName {
			targetFolder = &s.folders[i]
			break
		}
	}

	if targetFolder == nil {
		s.folders = append(s.folders, SessionFolder{
			FolderName: folderName,
			Sessions:   []SessionYAML{},
		})
		targetFolder = &s.folders[len(s.folders)-1]
	}

	// Convert and add
	targetFolder.Sessions = append(targetFolder.Sessions, s.sessionInfoToYAML(session))
}

// RemoveSession removes a session by ID
func (s *SessionStore) RemoveSession(sessionID string) bool {
	for fi := range s.folders {
		for si := range s.folders[fi].Sessions {
			id := fmt.Sprintf("%s-%d", s.folders[fi].FolderName, si)
			if id == sessionID {
				// Remove session
				s.folders[fi].Sessions = append(
					s.folders[fi].Sessions[:si],
					s.folders[fi].Sessions[si+1:]...,
				)
				return true
			}
		}
	}
	return false
}

// AddFolder adds a new empty folder
func (s *SessionStore) AddFolder(name string) {
	// Check if already exists
	for _, folder := range s.folders {
		if folder.FolderName == name {
			return
		}
	}

	s.folders = append(s.folders, SessionFolder{
		FolderName: name,
		Sessions:   []SessionYAML{},
	})
}

// RemoveFolder removes a folder and all its sessions
func (s *SessionStore) RemoveFolder(name string) bool {
	for i, folder := range s.folders {
		if folder.FolderName == name {
			s.folders = append(s.folders[:i], s.folders[i+1:]...)
			return true
		}
	}
	return false
}

// GetFilePath returns the current sessions file path
func (s *SessionStore) GetFilePath() string {
	return s.filePath
}
