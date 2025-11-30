// settings.go - Application settings modal with persistent configuration
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// AppSettings holds all application configuration
type AppSettings struct {
	// Terminal Display
	RowOffset    int `json:"row_offset"`    // Row adjustment for terminal sizing (default: 2, retina: 4)
	ColOffset    int `json:"col_offset"`    // Column adjustment for terminal sizing (default: 0)
	FontSize     int `json:"font_size"`     // Terminal font size in points (default: 12)
	
	// Appearance
	DarkTheme    bool `json:"dark_theme"`   // Use dark theme (default: true)
	
	// SSH Defaults
	DefaultKeyPath    string `json:"default_key_path"`    // Default SSH key path (default: ~/.ssh/id_rsa)
	DefaultPort       int    `json:"default_port"`        // Default SSH port (default: 22)
	DefaultUsername   string `json:"default_username"`    // Default username for new sessions
	
	// Connection
	ConnectionTimeout int  `json:"connection_timeout"` // SSH connection timeout in seconds (default: 30)
	KeepaliveInterval int  `json:"keepalive_interval"` // Keepalive interval in seconds (default: 60)
	
	// Logging
	EnableLogging     bool   `json:"enable_logging"`     // Enable per-session logging (default: false)
	LogDirectory      string `json:"log_directory"`      // Directory for session logs (default: ./logs)
	TimestampLogs     bool   `json:"timestamp_logs"`     // Add timestamps to log entries (default: true)
	
	// Terminal Behavior
	ScrollbackLines   int  `json:"scrollback_lines"`   // Number of scrollback lines (default: 1000)
	CopyOnSelect      bool `json:"copy_on_select"`     // Copy to clipboard on selection (default: false)
	
	// Window
	RememberWindowSize bool `json:"remember_window_size"` // Remember window size on exit (default: true)
	WindowWidth        int  `json:"window_width"`         // Saved window width
	WindowHeight       int  `json:"window_height"`        // Saved window height
}

// DefaultSettings returns settings with sensible defaults
func DefaultSettings() *AppSettings {
	homeDir, _ := os.UserHomeDir()
	defaultKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
	
	return &AppSettings{
		// Terminal Display
		RowOffset:    2,
		ColOffset:    0,
		FontSize:     12,
		
		// Appearance
		DarkTheme:    true,
		
		// SSH Defaults
		DefaultKeyPath:    defaultKeyPath,
		DefaultPort:       22,
		DefaultUsername:   "",
		
		// Connection
		ConnectionTimeout: 30,
		KeepaliveInterval: 60,
		
		// Logging
		EnableLogging:     false,
		LogDirectory:      "./logs",
		TimestampLogs:     true,
		
		// Terminal Behavior
		ScrollbackLines:   1000,
		CopyOnSelect:      false,
		
		// Window
		RememberWindowSize: true,
		WindowWidth:        1200,
		WindowHeight:       800,
	}
}

// SettingsManager handles loading, saving, and editing settings
type SettingsManager struct {
	settings     *AppSettings
	settingsPath string
	onSave       func(*AppSettings) // Callback when settings are saved
}

// NewSettingsManager creates a new settings manager
func NewSettingsManager() *SettingsManager {
	sm := &SettingsManager{
		settings:     DefaultSettings(),
		settingsPath: getSettingsPath(),
	}
	sm.Load()
	return sm
}

// getSettingsPath returns the path to the settings file
func getSettingsPath() string {
	// Store settings in app directory
	return "./settings.json"
}

// Load reads settings from disk
func (sm *SettingsManager) Load() error {
	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No settings file - use defaults and create one
			log.Printf("No settings file found, using defaults")
			return sm.Save()
		}
		return fmt.Errorf("failed to read settings: %w", err)
	}
	
	// Start with defaults, then overlay saved settings
	sm.settings = DefaultSettings()
	if err := json.Unmarshal(data, sm.settings); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}
	
	log.Printf("Loaded settings from %s", sm.settingsPath)
	return nil
}

// Save writes settings to disk
func (sm *SettingsManager) Save() error {
	data, err := json.MarshalIndent(sm.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	
	if err := os.WriteFile(sm.settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}
	
	log.Printf("Saved settings to %s", sm.settingsPath)
	return nil
}

// Get returns the current settings
func (sm *SettingsManager) Get() *AppSettings {
	return sm.settings
}

// SetOnSave sets a callback for when settings are saved
func (sm *SettingsManager) SetOnSave(callback func(*AppSettings)) {
	sm.onSave = callback
}

// ShowSettingsDialog displays the settings modal
func (sm *SettingsManager) ShowSettingsDialog(window fyne.Window) {
	// Create a copy of settings to edit
	editSettings := *sm.settings
	
	// === Terminal Display Tab ===
	rowOffsetEntry := widget.NewEntry()
	rowOffsetEntry.SetText(strconv.Itoa(editSettings.RowOffset))
	rowOffsetEntry.SetPlaceHolder("2")
	
	colOffsetEntry := widget.NewEntry()
	colOffsetEntry.SetText(strconv.Itoa(editSettings.ColOffset))
	colOffsetEntry.SetPlaceHolder("0")
	
	fontSizeEntry := widget.NewEntry()
	fontSizeEntry.SetText(strconv.Itoa(editSettings.FontSize))
	fontSizeEntry.SetPlaceHolder("12")
	fontSizeEntry.Disable() // TODO: Not yet implemented
	
	scrollbackEntry := widget.NewEntry()
	scrollbackEntry.SetText(strconv.Itoa(editSettings.ScrollbackLines))
	scrollbackEntry.SetPlaceHolder("1000")
	scrollbackEntry.Disable() // TODO: Not yet implemented
	
	copyOnSelectCheck := widget.NewCheck("Copy text to clipboard when selected", nil)
	copyOnSelectCheck.SetChecked(editSettings.CopyOnSelect)
	copyOnSelectCheck.Disable() // TODO: Not yet implemented
	
	terminalForm := widget.NewForm(
		widget.NewFormItem("Row Offset", container.NewBorder(nil, nil, nil, 
			widget.NewLabel("(increase for Retina: 4)"), rowOffsetEntry)),
		widget.NewFormItem("Column Offset", colOffsetEntry),
		widget.NewFormItem("Font Size", fontSizeEntry),
		widget.NewFormItem("Scrollback Lines", scrollbackEntry),
		widget.NewFormItem("", copyOnSelectCheck),
	)
	
	terminalTab := container.NewVBox(
		widget.NewLabel("Terminal Display Settings"),
		widget.NewSeparator(),
		terminalForm,
	)
	
	// === Appearance Tab ===
	darkThemeCheck := widget.NewCheck("Dark Theme", nil)
	darkThemeCheck.SetChecked(editSettings.DarkTheme)
	
	rememberSizeCheck := widget.NewCheck("Remember window size on exit", nil)
	rememberSizeCheck.SetChecked(editSettings.RememberWindowSize)
	
	appearanceForm := widget.NewForm(
		widget.NewFormItem("", darkThemeCheck),
		widget.NewFormItem("", rememberSizeCheck),
	)
	
	appearanceTab := container.NewVBox(
		widget.NewLabel("Appearance Settings"),
		widget.NewSeparator(),
		appearanceForm,
	)
	
	// === SSH Defaults Tab ===
	defaultKeyEntry := widget.NewEntry()
	defaultKeyEntry.SetText(editSettings.DefaultKeyPath)
	defaultKeyEntry.SetPlaceHolder("~/.ssh/id_rsa")
	defaultKeyEntry.Disable() // TODO: Not yet implemented
	
	defaultPortEntry := widget.NewEntry()
	defaultPortEntry.SetText(strconv.Itoa(editSettings.DefaultPort))
	defaultPortEntry.SetPlaceHolder("22")
	defaultPortEntry.Disable() // TODO: Not yet implemented
	
	defaultUserEntry := widget.NewEntry()
	defaultUserEntry.SetText(editSettings.DefaultUsername)
	defaultUserEntry.SetPlaceHolder("(optional)")
	defaultUserEntry.Disable() // TODO: Not yet implemented
	
	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(editSettings.ConnectionTimeout))
	timeoutEntry.SetPlaceHolder("30")
	timeoutEntry.Disable() // TODO: Not yet implemented
	
	keepaliveEntry := widget.NewEntry()
	keepaliveEntry.SetText(strconv.Itoa(editSettings.KeepaliveInterval))
	keepaliveEntry.SetPlaceHolder("60")
	keepaliveEntry.Disable() // TODO: Not yet implemented
	
	sshForm := widget.NewForm(
		widget.NewFormItem("Default SSH Key", defaultKeyEntry),
		widget.NewFormItem("Default Port", defaultPortEntry),
		widget.NewFormItem("Default Username", defaultUserEntry),
		widget.NewFormItem("Connection Timeout (s)", timeoutEntry),
		widget.NewFormItem("Keepalive Interval (s)", keepaliveEntry),
	)
	
	sshTab := container.NewVBox(
		widget.NewLabel("SSH Default Settings"),
		widget.NewSeparator(),
		sshForm,
	)
	
	// === Logging Tab ===
	enableLoggingCheck := widget.NewCheck("Enable per-session logging", nil)
	enableLoggingCheck.SetChecked(editSettings.EnableLogging)
	enableLoggingCheck.Disable() // TODO: Not yet implemented
	
	logDirEntry := widget.NewEntry()
	logDirEntry.SetText(editSettings.LogDirectory)
	logDirEntry.SetPlaceHolder("./logs")
	logDirEntry.Disable() // TODO: Not yet implemented
	
	timestampCheck := widget.NewCheck("Add timestamps to log entries", nil)
	timestampCheck.SetChecked(editSettings.TimestampLogs)
	timestampCheck.Disable() // TODO: Not yet implemented
	
	// Note: Removed dynamic enable/disable since all are disabled for now
	
	loggingForm := widget.NewForm(
		widget.NewFormItem("", enableLoggingCheck),
		widget.NewFormItem("Log Directory", logDirEntry),
		widget.NewFormItem("", timestampCheck),
	)
	
	loggingTab := container.NewVBox(
		widget.NewLabel("Session Logging Settings"),
		widget.NewSeparator(),
		loggingForm,
		widget.NewLabel(""),
		widget.NewLabel("Log files: {session_name}_{timestamp}.log"),
	)
	
	// === Create Tabs ===
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Terminal", theme.ComputerIcon(), terminalTab),
		container.NewTabItemWithIcon("Appearance", theme.ColorPaletteIcon(), appearanceTab),
		container.NewTabItemWithIcon("SSH", theme.SettingsIcon(), sshTab),
		container.NewTabItemWithIcon("Logging", theme.DocumentIcon(), loggingTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	
	// === Dialog Content ===
	content := container.NewBorder(
		nil,
		nil,
		nil,
		nil,
		tabs,
	)
	
	// === Create Dialog ===
	d := dialog.NewCustomConfirm(
		"Settings",
		"Save",
		"Cancel",
		content,
		func(save bool) {
			if !save {
				return
			}
			
			// Parse and validate entries
			var parseErrors []string
			
			if v, err := strconv.Atoi(rowOffsetEntry.Text); err == nil {
				editSettings.RowOffset = v
			} else {
				parseErrors = append(parseErrors, "Row Offset must be a number")
			}
			
			if v, err := strconv.Atoi(colOffsetEntry.Text); err == nil {
				editSettings.ColOffset = v
			} else {
				parseErrors = append(parseErrors, "Column Offset must be a number")
			}
			
			if v, err := strconv.Atoi(fontSizeEntry.Text); err == nil && v > 0 {
				editSettings.FontSize = v
			} else {
				parseErrors = append(parseErrors, "Font Size must be a positive number")
			}
			
			if v, err := strconv.Atoi(scrollbackEntry.Text); err == nil && v > 0 {
				editSettings.ScrollbackLines = v
			} else {
				parseErrors = append(parseErrors, "Scrollback Lines must be a positive number")
			}
			
			if v, err := strconv.Atoi(defaultPortEntry.Text); err == nil && v > 0 && v < 65536 {
				editSettings.DefaultPort = v
			} else {
				parseErrors = append(parseErrors, "Port must be 1-65535")
			}
			
			if v, err := strconv.Atoi(timeoutEntry.Text); err == nil && v > 0 {
				editSettings.ConnectionTimeout = v
			} else {
				parseErrors = append(parseErrors, "Timeout must be a positive number")
			}
			
			if v, err := strconv.Atoi(keepaliveEntry.Text); err == nil && v >= 0 {
				editSettings.KeepaliveInterval = v
			} else {
				parseErrors = append(parseErrors, "Keepalive must be a non-negative number")
			}
			
			if len(parseErrors) > 0 {
				errMsg := "Please fix the following errors:\n"
				for _, e := range parseErrors {
					errMsg += "• " + e + "\n"
				}
				dialog.ShowError(fmt.Errorf(errMsg), window)
				return
			}
			
			// Get remaining values
			editSettings.CopyOnSelect = copyOnSelectCheck.Checked
			editSettings.DarkTheme = darkThemeCheck.Checked
			editSettings.RememberWindowSize = rememberSizeCheck.Checked
			editSettings.DefaultKeyPath = defaultKeyEntry.Text
			editSettings.DefaultUsername = defaultUserEntry.Text
			editSettings.EnableLogging = enableLoggingCheck.Checked
			editSettings.LogDirectory = logDirEntry.Text
			editSettings.TimestampLogs = timestampCheck.Checked
			
			// Apply settings
			sm.settings = &editSettings
			
			// Save to disk
			if err := sm.Save(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save settings: %w", err), window)
				return
			}
			
			// Notify listeners
			if sm.onSave != nil {
				sm.onSave(sm.settings)
			}
			
			dialog.ShowInformation("Settings Saved", 
				"Settings have been saved.\n\nSome changes may require restarting the application.", 
				window)
		},
		window,
	)
	
	d.Resize(fyne.NewSize(500, 450))
	d.Show()
}

// Global settings manager instance
var globalSettings *SettingsManager

// GetSettings returns the global settings manager, creating it if needed
func GetSettings() *SettingsManager {
	if globalSettings == nil {
		globalSettings = NewSettingsManager()
	}
	return globalSettings
}