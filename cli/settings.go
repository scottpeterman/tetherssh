// settings.go - Application settings modal with persistent configuration
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ColorOverrides allows customizing theme colors via settings
// Empty strings mean "use default theme color"
type ColorOverrides struct {
	Primary         string `json:"primary,omitempty"`          // Main accent color (buttons, focus)
	Secondary       string `json:"secondary,omitempty"`        // Secondary accent (links)
	Background      string `json:"background,omitempty"`       // Main background
	Surface         string `json:"surface,omitempty"`          // Panel/card backgrounds
	SurfaceVariant  string `json:"surface_variant,omitempty"`  // Alternate surface (headers)
	Foreground      string `json:"foreground,omitempty"`       // Primary text color
	InputBackground string `json:"input_background,omitempty"` // Entry field background
	InputBorder     string `json:"input_border,omitempty"`     // Entry field border
	Selection       string `json:"selection,omitempty"`        // Text selection highlight
	Hover           string `json:"hover,omitempty"`            // Hover state background
	Error           string `json:"error,omitempty"`            // Error states
	Success         string `json:"success,omitempty"`          // Success indicators
	Warning         string `json:"warning,omitempty"`          // Warning indicators
}

// AppSettings holds all application configuration
type AppSettings struct {
	// Terminal Display
	RowOffset int `json:"row_offset"` // Row adjustment for terminal sizing (default: 2, retina: 4)
	ColOffset int `json:"col_offset"` // Column adjustment for terminal sizing (default: 0)
	FontSize  int `json:"font_size"`  // Terminal font size in points (default: 12)

	// Appearance
	DarkTheme bool `json:"dark_theme"` // Use dark theme (default: true)

	// Theme Color Overrides
	DarkThemeColors  ColorOverrides `json:"dark_theme_colors"`
	LightThemeColors ColorOverrides `json:"light_theme_colors"`

	// SSH Defaults
	DefaultKeyPath  string `json:"default_key_path"`  // Default SSH key path (default: ~/.ssh/id_rsa)
	DefaultPort     int    `json:"default_port"`      // Default SSH port (default: 22)
	DefaultUsername string `json:"default_username"`  // Default username for new sessions

	// Connection
	ConnectionTimeout int `json:"connection_timeout"` // SSH connection timeout in seconds (default: 30)
	KeepaliveInterval int `json:"keepalive_interval"` // Keepalive interval in seconds (default: 60)

	// Logging
	EnableLogging bool   `json:"enable_logging"` // Enable per-session logging (default: false)
	LogDirectory  string `json:"log_directory"`  // Directory for session logs (default: ./logs)
	TimestampLogs bool   `json:"timestamp_logs"` // Add timestamps to log entries (default: true)

	// Terminal Behavior
	ScrollbackLines int  `json:"scrollback_lines"` // Number of scrollback lines (default: 1000)
	CopyOnSelect    bool `json:"copy_on_select"`   // Copy to clipboard on selection (default: false)

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
		RowOffset: 2,
		ColOffset: 0,
		FontSize:  12,

		// Appearance
		DarkTheme: true,

		// Theme overrides start empty (use built-in colors)
		DarkThemeColors:  ColorOverrides{},
		LightThemeColors: ColorOverrides{},

		// SSH Defaults
		DefaultKeyPath:  defaultKeyPath,
		DefaultPort:     22,
		DefaultUsername: "",

		// Connection
		ConnectionTimeout: 30,
		KeepaliveInterval: 60,

		// Logging
		EnableLogging: false,
		LogDirectory:  "./logs",
		TimestampLogs: true,

		// Terminal Behavior
		ScrollbackLines: 1000,
		CopyOnSelect:    false,

		// Window
		RememberWindowSize: true,
		WindowWidth:        1200,
		WindowHeight:       800,
	}
}

// ParseHexColor parses a hex color string (with or without #) into color.Color
// Returns nil if the string is empty or invalid
func ParseHexColor(hex string) color.Color {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return nil
	}

	// Remove # prefix if present
	hex = strings.TrimPrefix(hex, "#")

	// Support 3-char shorthand (#RGB -> #RRGGBB)
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	if len(hex) != 6 && len(hex) != 8 {
		return nil
	}

	// Parse RGB
	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return nil
	}
	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return nil
	}
	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return nil
	}

	// Parse alpha if present, default to 255
	a := uint64(255)
	if len(hex) == 8 {
		a, err = strconv.ParseUint(hex[6:8], 16, 8)
		if err != nil {
			return nil
		}
	}

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}
}

// ColorToHex converts a color.Color to hex string (without #)
func ColorToHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, a := c.RGBA()
	// RGBA returns 16-bit values, convert to 8-bit
	r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
	if a8 == 255 {
		return fmt.Sprintf("%02x%02x%02x", r8, g8, b8)
	}
	return fmt.Sprintf("%02x%02x%02x%02x", r8, g8, b8, a8)
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

// createColorEditor creates the color editing UI for a theme
func createColorEditor(overrides *ColorOverrides, isDark bool, window fyne.Window) (fyne.CanvasObject, map[string]*widget.Entry) {
	entries := make(map[string]*widget.Entry)

	// Get default colors for this theme
	defaults := getDefaultColors(isDark)

	// Color definitions: name, field pointer, default color
	type colorDef struct {
		name     string
		field    *string
		defColor color.Color
	}

	colors := []colorDef{
		{"Primary", &overrides.Primary, defaults["primary"]},
		{"Secondary", &overrides.Secondary, defaults["secondary"]},
		{"Background", &overrides.Background, defaults["background"]},
		{"Surface", &overrides.Surface, defaults["surface"]},
		{"Surface Variant", &overrides.SurfaceVariant, defaults["surface_variant"]},
		{"Foreground", &overrides.Foreground, defaults["foreground"]},
		{"Input Background", &overrides.InputBackground, defaults["input_background"]},
		{"Input Border", &overrides.InputBorder, defaults["input_border"]},
		{"Selection", &overrides.Selection, defaults["selection"]},
		{"Hover", &overrides.Hover, defaults["hover"]},
		{"Error", &overrides.Error, defaults["error"]},
		{"Success", &overrides.Success, defaults["success"]},
		{"Warning", &overrides.Warning, defaults["warning"]},
	}

	// Build form items
	formItems := make([]*widget.FormItem, 0, len(colors))
	for _, cd := range colors {
		// Create entry
		entry := widget.NewEntry()
		entry.SetPlaceHolder("default")
		entry.SetText(*cd.field)
		entries[cd.name] = entry

		// Create color swatch
		swatchColor := cd.defColor
		if c := ParseHexColor(*cd.field); c != nil {
			swatchColor = c
		}
		swatch := canvas.NewRectangle(swatchColor)
		swatch.SetMinSize(fyne.NewSize(24, 24))
		swatch.StrokeColor = color.RGBA{128, 128, 128, 255}
		swatch.StrokeWidth = 1

		// Capture for closures
		capturedDef := cd
		capturedEntry := entry
		capturedSwatch := swatch

		// Update swatch when entry changes
		entry.OnChanged = func(text string) {
			if c := ParseHexColor(text); c != nil {
				capturedSwatch.FillColor = c
				capturedSwatch.Refresh()
			} else if text == "" {
				capturedSwatch.FillColor = capturedDef.defColor
				capturedSwatch.Refresh()
			}
		}

		// Picker button
		pickerBtn := widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
			startColor := capturedDef.defColor
			if c := ParseHexColor(capturedEntry.Text); c != nil {
				startColor = c
			}
			picker := dialog.NewColorPicker("Pick Color: "+capturedDef.name, "Select a color", func(c color.Color) {
				if c != nil {
					hex := ColorToHex(c)
					capturedEntry.SetText(hex)
					capturedSwatch.FillColor = c
					capturedSwatch.Refresh()
				}
			}, window)
			picker.Advanced = true
			picker.SetColor(startColor)
			picker.Show()
		})

		// Reset button
		resetBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
			capturedEntry.SetText("")
			capturedSwatch.FillColor = capturedDef.defColor
			capturedSwatch.Refresh()
		})
		resetBtn.Importance = widget.LowImportance

		// Make entry expand, buttons on right
		row := container.NewBorder(nil, nil, nil, container.NewHBox(swatch, pickerBtn, resetBtn), entry)

		formItems = append(formItems, widget.NewFormItem(cd.name, row))
	}

	form := widget.NewForm(formItems...)

	// Preview panel
	preview := createThemePreview(overrides, isDark)

	// Update preview when any entry changes
	for name, entry := range entries {
		capturedName := name
		capturedEntry := entry
		origOnChanged := capturedEntry.OnChanged
		capturedEntry.OnChanged = func(text string) {
			if origOnChanged != nil {
				origOnChanged(text)
			}
			// Update the override field
			updateOverrideField(overrides, capturedName, text)
			// Refresh preview
			refreshPreview(preview, overrides, isDark)
		}
	}

	// Wrap in scroll container since there are many colors
	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(350, 250))

	// Reset all button
	resetAllBtn := widget.NewButton("Reset All to Defaults", func() {
		for name, entry := range entries {
			entry.SetText("")
			updateOverrideField(overrides, name, "")
		}
		refreshPreview(preview, overrides, isDark)
	})

	content := container.NewBorder(
		nil,
		container.NewVBox(widget.NewSeparator(), resetAllBtn),
		nil,
		container.NewVBox(
			widget.NewLabel("Preview"),
			preview,
		),
		scroll,
	)

	return content, entries
}

// updateOverrideField updates the appropriate field in ColorOverrides
func updateOverrideField(overrides *ColorOverrides, name string, value string) {
	switch name {
	case "Primary":
		overrides.Primary = value
	case "Secondary":
		overrides.Secondary = value
	case "Background":
		overrides.Background = value
	case "Surface":
		overrides.Surface = value
	case "Surface Variant":
		overrides.SurfaceVariant = value
	case "Foreground":
		overrides.Foreground = value
	case "Input Background":
		overrides.InputBackground = value
	case "Input Border":
		overrides.InputBorder = value
	case "Selection":
		overrides.Selection = value
	case "Hover":
		overrides.Hover = value
	case "Error":
		overrides.Error = value
	case "Success":
		overrides.Success = value
	case "Warning":
		overrides.Warning = value
	}
}

// getDefaultColors returns the default colors for dark or light theme
func getDefaultColors(isDark bool) map[string]color.Color {
	if isDark {
		return map[string]color.Color{
			"primary":          color.RGBA{0x00, 0xcc, 0xff, 0xff}, // Cyber cyan
			"secondary":        color.RGBA{0x00, 0xff, 0x41, 0xff}, // Matrix green
			"background":       color.RGBA{0x00, 0x05, 0x10, 0xff},
			"surface":          color.RGBA{0x00, 0x10, 0x19, 0xff},
			"surface_variant":  color.RGBA{0x00, 0x1a, 0x1f, 0xff},
			"foreground":       color.RGBA{0x00, 0xcc, 0xff, 0xff},
			"input_background": color.RGBA{0x00, 0x10, 0x19, 0xff},
			"input_border":     color.RGBA{0x00, 0x44, 0x55, 0xff},
			"selection":        color.RGBA{0x00, 0xcc, 0xff, 0x40},
			"hover":            color.RGBA{0x00, 0x33, 0x44, 0xff},
			"error":            color.RGBA{0xff, 0x33, 0x33, 0xff},
			"success":          color.RGBA{0x00, 0xff, 0x88, 0xff},
			"warning":          color.RGBA{0xff, 0xaa, 0x00, 0xff},
		}
	}
	return map[string]color.Color{
		"primary":          color.RGBA{0x00, 0x78, 0xd4, 0xff},
		"secondary":        color.RGBA{0x00, 0x78, 0xd4, 0xff},
		"background":       color.RGBA{0xfa, 0xfa, 0xfa, 0xff},
		"surface":          color.RGBA{0xf5, 0xf5, 0xf5, 0xff},
		"surface_variant":  color.RGBA{0xe0, 0xe0, 0xe0, 0xff},
		"foreground":       color.RGBA{0x2e, 0x34, 0x40, 0xff},
		"input_background": color.RGBA{0xff, 0xff, 0xff, 0xff},
		"input_border":     color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
		"selection":        color.RGBA{0x00, 0x7a, 0xcc, 0x40},
		"hover":            color.RGBA{0xd5, 0xd5, 0xd5, 0xff},
		"error":            color.RGBA{0xd3, 0x2f, 0x2f, 0xff},
		"success":          color.RGBA{0x38, 0x8e, 0x3c, 0xff},
		"warning":          color.RGBA{0xf5, 0x7c, 0x00, 0xff},
	}
}

// getEffectiveColor returns the override color if set, otherwise the default
func getEffectiveColor(overrideHex string, defaultColor color.Color) color.Color {
	if c := ParseHexColor(overrideHex); c != nil {
		return c
	}
	return defaultColor
}

// createThemePreview creates a preview panel showing current theme colors
func createThemePreview(overrides *ColorOverrides, isDark bool) *fyne.Container {
	defaults := getDefaultColors(isDark)

	// Background
	bgColor := getEffectiveColor(overrides.Background, defaults["background"])
	bg := canvas.NewRectangle(bgColor)
	bg.SetMinSize(fyne.NewSize(180, 160))

	// Surface panel
	surfaceColor := getEffectiveColor(overrides.Surface, defaults["surface"])
	surface := canvas.NewRectangle(surfaceColor)
	surface.SetMinSize(fyne.NewSize(160, 120))

	// Foreground text
	fgColor := getEffectiveColor(overrides.Foreground, defaults["foreground"])
	textLabel := canvas.NewText("Sample Text", fgColor)
	textLabel.TextSize = 12

	// Primary colored element
	primaryColor := getEffectiveColor(overrides.Primary, defaults["primary"])
	primaryRect := canvas.NewRectangle(primaryColor)
	primaryRect.SetMinSize(fyne.NewSize(60, 20))

	// Secondary colored element
	secondaryColor := getEffectiveColor(overrides.Secondary, defaults["secondary"])
	secondaryRect := canvas.NewRectangle(secondaryColor)
	secondaryRect.SetMinSize(fyne.NewSize(60, 20))

	// Error indicator
	errorColor := getEffectiveColor(overrides.Error, defaults["error"])
	errorRect := canvas.NewRectangle(errorColor)
	errorRect.SetMinSize(fyne.NewSize(40, 10))

	// Success indicator
	successColor := getEffectiveColor(overrides.Success, defaults["success"])
	successRect := canvas.NewRectangle(successColor)
	successRect.SetMinSize(fyne.NewSize(40, 10))

	// Build preview layout
	indicators := container.NewHBox(errorRect, successRect)
	accents := container.NewHBox(primaryRect, secondaryRect)

	innerContent := container.NewVBox(
		textLabel,
		accents,
		indicators,
	)

	// Stack surface on background
	preview := container.NewStack(
		bg,
		container.NewPadded(
			container.NewStack(
				surface,
				container.NewPadded(innerContent),
			),
		),
	)

	return preview
}

// refreshPreview updates the preview panel colors
func refreshPreview(preview *fyne.Container, overrides *ColorOverrides, isDark bool) {
	defaults := getDefaultColors(isDark)

	// Navigate the container structure to update colors
	// Structure: Stack[bg, Padded[Stack[surface, Padded[VBox[text, accents, indicators]]]]]

	if len(preview.Objects) < 2 {
		return
	}

	// Update background
	if bg, ok := preview.Objects[0].(*canvas.Rectangle); ok {
		bg.FillColor = getEffectiveColor(overrides.Background, defaults["background"])
		bg.Refresh()
	}

	// Get padded container
	padded, ok := preview.Objects[1].(*fyne.Container)
	if !ok || len(padded.Objects) < 1 {
		return
	}

	// Get inner stack
	innerStack, ok := padded.Objects[0].(*fyne.Container)
	if !ok || len(innerStack.Objects) < 2 {
		return
	}

	// Update surface
	if surface, ok := innerStack.Objects[0].(*canvas.Rectangle); ok {
		surface.FillColor = getEffectiveColor(overrides.Surface, defaults["surface"])
		surface.Refresh()
	}

	// Get inner padded
	innerPadded, ok := innerStack.Objects[1].(*fyne.Container)
	if !ok || len(innerPadded.Objects) < 1 {
		return
	}

	// Get VBox
	vbox, ok := innerPadded.Objects[0].(*fyne.Container)
	if !ok || len(vbox.Objects) < 3 {
		return
	}

	// Update text color
	if text, ok := vbox.Objects[0].(*canvas.Text); ok {
		text.Color = getEffectiveColor(overrides.Foreground, defaults["foreground"])
		text.Refresh()
	}

	// Update accents (HBox with primary and secondary)
	if accents, ok := vbox.Objects[1].(*fyne.Container); ok && len(accents.Objects) >= 2 {
		if primary, ok := accents.Objects[0].(*canvas.Rectangle); ok {
			primary.FillColor = getEffectiveColor(overrides.Primary, defaults["primary"])
			primary.Refresh()
		}
		if secondary, ok := accents.Objects[1].(*canvas.Rectangle); ok {
			secondary.FillColor = getEffectiveColor(overrides.Secondary, defaults["secondary"])
			secondary.Refresh()
		}
	}

	// Update indicators (HBox with error and success)
	if indicators, ok := vbox.Objects[2].(*fyne.Container); ok && len(indicators.Objects) >= 2 {
		if errorRect, ok := indicators.Objects[0].(*canvas.Rectangle); ok {
			errorRect.FillColor = getEffectiveColor(overrides.Error, defaults["error"])
			errorRect.Refresh()
		}
		if successRect, ok := indicators.Objects[1].(*canvas.Rectangle); ok {
			successRect.FillColor = getEffectiveColor(overrides.Success, defaults["success"])
			successRect.Refresh()
		}
	}
}

// ShowSettingsDialog displays the settings modal
func (sm *SettingsManager) ShowSettingsDialog(window fyne.Window) {
	// Create a copy of settings to edit
	editSettings := *sm.settings
	// Deep copy color overrides
	editDarkColors := sm.settings.DarkThemeColors
	editLightColors := sm.settings.LightThemeColors

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

	// === Colors Tab ===
	darkColorEditor, darkEntries := createColorEditor(&editDarkColors, true, window)
	lightColorEditor, lightEntries := createColorEditor(&editLightColors, false, window)

	colorTabs := container.NewAppTabs(
		container.NewTabItem("Dark Theme", darkColorEditor),
		container.NewTabItem("Light Theme", lightColorEditor),
	)

	colorsTab := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Theme Color Overrides"),
			widget.NewLabel("Leave empty to use default colors"),
			widget.NewSeparator(),
		),
		nil, nil, nil,
		colorTabs,
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
		container.NewTabItemWithIcon("Colors", theme.ColorChromaticIcon(), colorsTab),
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

			// Validate color hex values
			allColorEntries := make(map[string]*widget.Entry)
			for k, v := range darkEntries {
				allColorEntries["Dark "+k] = v
			}
			for k, v := range lightEntries {
				allColorEntries["Light "+k] = v
			}
			for name, entry := range allColorEntries {
				if entry.Text != "" && ParseHexColor(entry.Text) == nil {
					parseErrors = append(parseErrors, fmt.Sprintf("%s: invalid hex color", name))
				}
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

			// Get color overrides from entries
			editSettings.DarkThemeColors = editDarkColors
			editSettings.LightThemeColors = editLightColors

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

	d.Resize(fyne.NewSize(650, 550))
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