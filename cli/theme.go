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
	switch name {
	case theme.ColorNameForeground:
		if t.isDark {
			return color.RGBA{0xe0, 0xe0, 0xe0, 0xff}
		}
		return color.RGBA{0x2e, 0x34, 0x40, 0xff}
	case theme.ColorNameBackground:
		if t.isDark {
			return color.RGBA{0x1e, 0x1e, 0x1e, 0xff}
		}
		return color.RGBA{0xfa, 0xfa, 0xfa, 0xff}
	case theme.ColorNameSelection:
		if t.isDark {
			return color.RGBA{0x44, 0x47, 0x5a, 0x80}
		}
		return color.RGBA{0x00, 0x7a, 0xcc, 0x40}
	case theme.ColorNamePrimary:
		if t.isDark {
			return color.RGBA{0x00, 0xd4, 0xaa, 0xff}
		}
		return color.RGBA{0x00, 0x78, 0xd4, 0xff}
	}
	return t.Theme.Color(name, variant)
}

func (t *NativeTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}
