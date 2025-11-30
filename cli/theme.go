package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Color mapping for terminal colors
var colorMappings = map[string]color.Color{
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

// Cyber theme colors (from your CSS)
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

// NativeTheme provides a more native terminal appearance
type NativeTheme struct {
	fyne.Theme
	isDark bool
}

func NewNativeTheme(dark bool) *NativeTheme {
	return &NativeTheme{
		Theme:  theme.DefaultTheme(),
		isDark: dark,
	}
}

func (t *NativeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Force variant based on our dark/light setting
	if t.isDark {
		variant = theme.VariantDark
	} else {
		variant = theme.VariantLight
	}

	// Dark theme = Cyber theme
	if t.isDark {
		return t.cyberColor(name)
	}

	// Light theme (your existing light theme)
	return t.lightColor(name, variant)
}

// cyberColor returns colors for the cyber/dark theme
func (t *NativeTheme) cyberColor(name fyne.ThemeColorName) color.Color {
	switch name {
	// Primary colors
	case theme.ColorNamePrimary:
		return cyberPrimary
	case theme.ColorNameFocus:
		return cyberPrimary

	// Foreground/text colors
	case theme.ColorNameForeground:
		return cyberOnSurface

	// Background colors
	case theme.ColorNameBackground:
		return cyberBackground

	// Button colors
	case theme.ColorNameButton:
		return cyberSurfaceVariant
	case theme.ColorNameDisabledButton:
		return color.RGBA{0x00, 0x22, 0x2a, 0xff}
	case theme.ColorNameDisabled:
		return color.RGBA{0x00, 0x66, 0x77, 0x99} // Dimmed cyan

	// Input fields
	case theme.ColorNameInputBackground:
		return cyberSurface
	case theme.ColorNameInputBorder:
		return cyberOutlineVariant
	case theme.ColorNamePlaceHolder:
		return color.RGBA{0x00, 0x88, 0xaa, 0xaa} // Dimmed cyan

	// Menus and overlays
	case theme.ColorNameMenuBackground:
		return cyberSurfaceContainer
	case theme.ColorNameOverlayBackground:
		return cyberSurfaceContainer

	// Hover state
	case theme.ColorNameHover:
		return color.RGBA{0x00, 0x33, 0x44, 0xff}

	// Selection
	case theme.ColorNameSelection:
		return color.RGBA{0x00, 0xcc, 0xff, 0x40} // Cyan with transparency

	// Separators and scrollbars
	case theme.ColorNameSeparator:
		return cyberOutlineVariant
	case theme.ColorNameScrollBar:
		return cyberPrimary

	// Hyperlinks
	case theme.ColorNameHyperlink:
		return cyberSecondary // Matrix green for links

	// Error states
	case theme.ColorNameError:
		return cyberError

	// Success (used for some widgets)
	case theme.ColorNameSuccess:
		return cyberSuccess

	// Warning
	case theme.ColorNameWarning:
		return cyberWarning

	// Header background (for list headers, etc.)
	case theme.ColorNameHeaderBackground:
		return cyberSurfaceVariant

	// Shadow (minimal for flat look)
	case theme.ColorNameShadow:
		return color.RGBA{0x00, 0x00, 0x00, 0x66}
	}

	// Fallback to default dark theme
	return t.Theme.Color(name, theme.VariantDark)
}

// lightColor returns colors for the light theme
func (t *NativeTheme) lightColor(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameForeground:
		return color.RGBA{0x2e, 0x34, 0x40, 0xff}

	case theme.ColorNameBackground:
		return color.RGBA{0xfa, 0xfa, 0xfa, 0xff}

	case theme.ColorNameButton:
		return color.RGBA{0xe0, 0xe0, 0xe0, 0xff}

	case theme.ColorNameDisabledButton:
		return color.RGBA{0xcc, 0xcc, 0xcc, 0xff}

	case theme.ColorNameDisabled:
		return color.RGBA{0x99, 0x99, 0x99, 0xff}

	case theme.ColorNameInputBackground:
		return color.RGBA{0xff, 0xff, 0xff, 0xff}

	case theme.ColorNameInputBorder:
		return color.RGBA{0xcc, 0xcc, 0xcc, 0xff}

	case theme.ColorNameMenuBackground:
		return color.RGBA{0xf5, 0xf5, 0xf5, 0xff}

	case theme.ColorNameOverlayBackground:
		return color.RGBA{0xf5, 0xf5, 0xf5, 0xff}

	case theme.ColorNameHover:
		return color.RGBA{0xd5, 0xd5, 0xd5, 0xff}

	case theme.ColorNameSelection:
		return color.RGBA{0x00, 0x7a, 0xcc, 0x40}

	case theme.ColorNamePrimary:
		return color.RGBA{0x00, 0x78, 0xd4, 0xff}

	case theme.ColorNameSeparator:
		return color.RGBA{0xcc, 0xcc, 0xcc, 0xff}

	case theme.ColorNameScrollBar:
		return color.RGBA{0xaa, 0xaa, 0xaa, 0xff}
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