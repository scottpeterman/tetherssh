package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Color mapping for terminal colors (16-color ANSI palette)
// Dark theme colors - optimized for dark backgrounds
var darkColorMappings = map[string]color.Color{
	"black":          color.RGBA{0x00, 0x00, 0x00, 0xff},
	"red":            color.RGBA{0xff, 0x00, 0x00, 0xff},
	"green":          color.RGBA{0x00, 0xff, 0x00, 0xff},
	"yellow":         color.RGBA{0xff, 0xff, 0x00, 0xff},
	"blue":           color.RGBA{0x00, 0x00, 0xff, 0xff},
	"magenta":        color.RGBA{0xff, 0x00, 0xff, 0xff},
	"cyan":           color.RGBA{0x00, 0xff, 0xff, 0xff},
	"white":          color.RGBA{0xff, 0xff, 0xff, 0xff},
	"bright_black":   color.RGBA{0x7f, 0x7f, 0x7f, 0xff},
	"bright_red":     color.RGBA{0xff, 0x5f, 0x5f, 0xff},
	"bright_green":   color.RGBA{0x5f, 0xff, 0x5f, 0xff},
	"bright_yellow":  color.RGBA{0xff, 0xff, 0x5f, 0xff},
	"bright_blue":    color.RGBA{0x5f, 0x5f, 0xff, 0xff},
	"bright_magenta": color.RGBA{0xff, 0x5f, 0xff, 0xff},
	"bright_cyan":    color.RGBA{0x5f, 0xff, 0xff, 0xff},
	"bright_white":   color.RGBA{0xff, 0xff, 0xff, 0xff},
	"default":        color.RGBA{0xe0, 0xe0, 0xe0, 0xff},
}

// Light theme colors - darker/more saturated for readability on light backgrounds
var lightColorMappings = map[string]color.Color{
	"black":          color.RGBA{0x00, 0x00, 0x00, 0xff},
	"red":            color.RGBA{0xcc, 0x00, 0x00, 0xff}, // Darker red
	"green":          color.RGBA{0x00, 0x80, 0x00, 0xff}, // Dark green (not lime)
	"yellow":         color.RGBA{0x80, 0x80, 0x00, 0xff}, // Olive/dark yellow
	"blue":           color.RGBA{0x00, 0x00, 0xcc, 0xff}, // Darker blue
	"magenta":        color.RGBA{0x80, 0x00, 0x80, 0xff}, // Purple
	"cyan":           color.RGBA{0x00, 0x80, 0x80, 0xff}, // Teal
	"white":          color.RGBA{0x80, 0x80, 0x80, 0xff}, // Gray (for contrast)
	"bright_black":   color.RGBA{0x54, 0x54, 0x54, 0xff}, // Dark gray
	"bright_red":     color.RGBA{0xff, 0x00, 0x00, 0xff}, // Bright red
	"bright_green":   color.RGBA{0x00, 0xaa, 0x00, 0xff}, // Medium green
	"bright_yellow":  color.RGBA{0xaa, 0xaa, 0x00, 0xff}, // Darker yellow
	"bright_blue":    color.RGBA{0x00, 0x00, 0xff, 0xff}, // Bright blue
	"bright_magenta": color.RGBA{0xaa, 0x00, 0xaa, 0xff}, // Bright purple
	"bright_cyan":    color.RGBA{0x00, 0xaa, 0xaa, 0xff}, // Bright teal
	"bright_white":   color.RGBA{0x00, 0x00, 0x00, 0xff}, // Black (inverted for light bg)
	"default":        color.RGBA{0x2e, 0x34, 0x40, 0xff}, // Dark text
}

// GetTerminalColorMappings returns the appropriate color mappings for current theme
func GetTerminalColorMappings() map[string]color.Color {
	settings := GetSettings()
	if settings != nil && !settings.Get().DarkTheme {
		return lightColorMappings
	}
	return darkColorMappings
}

// colorMappings is kept for backward compatibility but now returns dark theme
var colorMappings = darkColorMappings

// Default Cyber theme colors (built-in dark theme)
var (
	// Primary: Cyan (#00ccff)
	cyberPrimary   = color.RGBA{0x00, 0xcc, 0xff, 0xff}
	cyberOnPrimary = color.RGBA{0x00, 0x00, 0x00, 0xff}

	// Secondary: Matrix Green (#00ff41)
	cyberSecondary   = color.RGBA{0x00, 0xff, 0x41, 0xff}
	cyberOnSecondary = color.RGBA{0x00, 0x00, 0x00, 0xff}

	// Tertiary: Hot Pink (#ff0080)
	cyberTertiary = color.RGBA{0xff, 0x00, 0x80, 0xff}

	// Backgrounds - Deep blue-black
	cyberBackground         = color.RGBA{0x00, 0x05, 0x10, 0xff} // #000510
	cyberSurface            = color.RGBA{0x00, 0x10, 0x19, 0xff} // #001019
	cyberSurfaceVariant     = color.RGBA{0x00, 0x1a, 0x1f, 0xff} // #001a1f
	cyberSurfaceContainer   = color.RGBA{0x00, 0x1a, 0x1f, 0xff} // #001a1f
	cyberSurfaceContainerHi = color.RGBA{0x00, 0x20, 0x28, 0xff} // #002028

	// Text on dark backgrounds
	cyberOnBackground = color.RGBA{0x00, 0xcc, 0xff, 0xff} // Cyan text
	cyberOnSurface    = color.RGBA{0x00, 0xcc, 0xff, 0xff} // Cyan text

	// Outline: Cyan
	cyberOutline        = color.RGBA{0x00, 0xcc, 0xff, 0xff}
	cyberOutlineVariant = color.RGBA{0x00, 0x44, 0x55, 0xff} // #004455

	// Error: Bright red
	cyberError = color.RGBA{0xff, 0x33, 0x33, 0xff}

	// Success: Bright green
	cyberSuccess = color.RGBA{0x00, 0xff, 0x88, 0xff}

	// Warning: Orange
	cyberWarning = color.RGBA{0xff, 0xaa, 0x00, 0xff}
)

// Default light theme colors
var (
	lightPrimary         = color.RGBA{0x00, 0x78, 0xd4, 0xff}
	lightBackground      = color.RGBA{0xfa, 0xfa, 0xfa, 0xff}
	lightSurface         = color.RGBA{0xf5, 0xf5, 0xf5, 0xff}
	lightSurfaceVariant  = color.RGBA{0xe0, 0xe0, 0xe0, 0xff}
	lightForeground      = color.RGBA{0x2e, 0x34, 0x40, 0xff}
	lightInputBackground = color.RGBA{0xff, 0xff, 0xff, 0xff}
	lightInputBorder     = color.RGBA{0xcc, 0xcc, 0xcc, 0xff}
	lightSelection       = color.RGBA{0x00, 0x7a, 0xcc, 0x40}
	lightHover           = color.RGBA{0xd5, 0xd5, 0xd5, 0xff}
	lightError           = color.RGBA{0xd3, 0x2f, 0x2f, 0xff}
	lightSuccess         = color.RGBA{0x38, 0x8e, 0x3c, 0xff}
	lightWarning         = color.RGBA{0xf5, 0x7c, 0x00, 0xff}
)

// NativeTheme provides a customizable theme with override support
type NativeTheme struct {
	fyne.Theme
	isDark    bool
	overrides *ColorOverrides
}

// NewNativeTheme creates a new theme, loading overrides from settings
func NewNativeTheme(dark bool) *NativeTheme {
	t := &NativeTheme{
		Theme:  theme.DefaultTheme(),
		isDark: dark,
	}

	// Load overrides from settings
	settings := GetSettings()
	if settings != nil {
		if dark {
			t.overrides = &settings.Get().DarkThemeColors
		} else {
			t.overrides = &settings.Get().LightThemeColors
		}
	}

	return t
}

// getOverrideColor checks if there's an override for a color, returns nil if not
func (t *NativeTheme) getOverrideColor(name string) color.Color {
	if t.overrides == nil {
		return nil
	}

	var hex string
	switch name {
	case "primary":
		hex = t.overrides.Primary
	case "secondary":
		hex = t.overrides.Secondary
	case "background":
		hex = t.overrides.Background
	case "surface":
		hex = t.overrides.Surface
	case "surface_variant":
		hex = t.overrides.SurfaceVariant
	case "foreground":
		hex = t.overrides.Foreground
	case "input_background":
		hex = t.overrides.InputBackground
	case "input_border":
		hex = t.overrides.InputBorder
	case "selection":
		hex = t.overrides.Selection
	case "hover":
		hex = t.overrides.Hover
	case "error":
		hex = t.overrides.Error
	case "success":
		hex = t.overrides.Success
	case "warning":
		hex = t.overrides.Warning
	}

	return ParseHexColor(hex)
}

func (t *NativeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Force variant based on our dark/light setting
	if t.isDark {
		variant = theme.VariantDark
	} else {
		variant = theme.VariantLight
	}

	// Dark theme = Cyber theme with overrides
	if t.isDark {
		return t.cyberColor(name)
	}

	// Light theme with overrides
	return t.lightColor(name, variant)
}

// cyberColor returns colors for the cyber/dark theme, checking overrides first
func (t *NativeTheme) cyberColor(name fyne.ThemeColorName) color.Color {
	switch name {
	// Primary colors
	case theme.ColorNamePrimary:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return cyberPrimary
	case theme.ColorNameFocus:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return cyberPrimary

	// Foreground/text colors
	case theme.ColorNameForeground:
		if c := t.getOverrideColor("foreground"); c != nil {
			return c
		}
		return cyberOnSurface

	// Background colors
	case theme.ColorNameBackground:
		if c := t.getOverrideColor("background"); c != nil {
			return c
		}
		return cyberBackground

	// Button colors
	case theme.ColorNameButton:
		if c := t.getOverrideColor("surface_variant"); c != nil {
			return c
		}
		return cyberSurfaceVariant
	case theme.ColorNameDisabledButton:
		return color.RGBA{0x00, 0x22, 0x2a, 0xff}
	case theme.ColorNameDisabled:
		return color.RGBA{0x00, 0x66, 0x77, 0x99} // Dimmed cyan

	// Input fields
	case theme.ColorNameInputBackground:
		if c := t.getOverrideColor("input_background"); c != nil {
			return c
		}
		return cyberSurface
	case theme.ColorNameInputBorder:
		if c := t.getOverrideColor("input_border"); c != nil {
			return c
		}
		return cyberOutlineVariant
	case theme.ColorNamePlaceHolder:
		return color.RGBA{0x00, 0x88, 0xaa, 0xaa} // Dimmed cyan

	// Menus and overlays
	case theme.ColorNameMenuBackground:
		if c := t.getOverrideColor("surface"); c != nil {
			return c
		}
		return cyberSurfaceContainer
	case theme.ColorNameOverlayBackground:
		if c := t.getOverrideColor("surface"); c != nil {
			return c
		}
		return cyberSurfaceContainer

// Hover state
	case theme.ColorNameHover:
		if c := t.getOverrideColor("hover"); c != nil {
			return c
		}
		return color.RGBA{0x00, 0x55, 0x66, 0xff}  // Solid dark teal


	// Separators and scrollbars
	case theme.ColorNameSeparator:
		if c := t.getOverrideColor("input_border"); c != nil {
			return c
		}
		return cyberOutlineVariant
	case theme.ColorNameScrollBar:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return cyberPrimary

	// Hyperlinks
	case theme.ColorNameHyperlink:
		if c := t.getOverrideColor("secondary"); c != nil {
			return c
		}
		return cyberSecondary // Matrix green for links

	// Error states
	case theme.ColorNameError:
		if c := t.getOverrideColor("error"); c != nil {
			return c
		}
		return cyberError

	// Success (used for some widgets)
	case theme.ColorNameSuccess:
		if c := t.getOverrideColor("success"); c != nil {
			return c
		}
		return cyberSuccess

	// Warning
	case theme.ColorNameWarning:
		if c := t.getOverrideColor("warning"); c != nil {
			return c
		}
		return cyberWarning

	// Header background (for list headers, etc.)
	case theme.ColorNameHeaderBackground:
		if c := t.getOverrideColor("surface_variant"); c != nil {
			return c
		}
		return cyberSurfaceVariant

	// Shadow (minimal for flat look)
	case theme.ColorNameShadow:
		return color.RGBA{0x00, 0x00, 0x00, 0x66}
	}

	// Fallback to default dark theme
	return t.Theme.Color(name, theme.VariantDark)
}

// lightColor returns colors for the light theme, checking overrides first
func (t *NativeTheme) lightColor(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return lightPrimary

	case theme.ColorNameFocus:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return lightPrimary

	case theme.ColorNameForeground:
		if c := t.getOverrideColor("foreground"); c != nil {
			return c
		}
		return lightForeground

	case theme.ColorNameBackground:
		if c := t.getOverrideColor("background"); c != nil {
			return c
		}
		return lightBackground

	case theme.ColorNameButton:
		if c := t.getOverrideColor("surface_variant"); c != nil {
			return c
		}
		return lightSurfaceVariant

	case theme.ColorNameDisabledButton:
		return color.RGBA{0xcc, 0xcc, 0xcc, 0xff}

	case theme.ColorNameDisabled:
		return color.RGBA{0x99, 0x99, 0x99, 0xff}

	case theme.ColorNameInputBackground:
		if c := t.getOverrideColor("input_background"); c != nil {
			return c
		}
		return lightInputBackground

	case theme.ColorNameInputBorder:
		if c := t.getOverrideColor("input_border"); c != nil {
			return c
		}
		return lightInputBorder

	case theme.ColorNameMenuBackground:
		if c := t.getOverrideColor("surface"); c != nil {
			return c
		}
		return lightSurface

	case theme.ColorNameOverlayBackground:
		if c := t.getOverrideColor("surface"); c != nil {
			return c
		}
		return lightSurface

	case theme.ColorNameHover:
		if c := t.getOverrideColor("hover"); c != nil {
			return c
		}
		return lightHover

	case theme.ColorNameSelection:
		if c := t.getOverrideColor("selection"); c != nil {
			return c
		}
		return lightSelection

	case theme.ColorNameSeparator:
		if c := t.getOverrideColor("input_border"); c != nil {
			return c
		}
		return lightInputBorder

	case theme.ColorNameScrollBar:
		if c := t.getOverrideColor("primary"); c != nil {
			return c
		}
		return color.RGBA{0xaa, 0xaa, 0xaa, 0xff}

	case theme.ColorNameHyperlink:
		if c := t.getOverrideColor("secondary"); c != nil {
			return c
		}
		return lightPrimary

	case theme.ColorNameError:
		if c := t.getOverrideColor("error"); c != nil {
			return c
		}
		return lightError

	case theme.ColorNameSuccess:
		if c := t.getOverrideColor("success"); c != nil {
			return c
		}
		return lightSuccess

	case theme.ColorNameWarning:
		if c := t.getOverrideColor("warning"); c != nil {
			return c
		}
		return lightWarning

	case theme.ColorNameHeaderBackground:
		if c := t.getOverrideColor("surface_variant"); c != nil {
			return c
		}
		return lightSurfaceVariant
	}

	// Fall back to default theme with our forced variant
	return t.Theme.Color(name, variant)
}

func (t *NativeTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *NativeTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func (t *NativeTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}