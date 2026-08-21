// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Theme defines the color theme for the dashboard.
type Theme string

const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

// themeColors holds the color palette for a given theme.
type themeColors struct {
	TextColor       tcell.Color
	LineColor       tcell.Color
	BackgroundColor tcell.Color
}

var darkTheme = themeColors{
	TextColor:       tcell.ColorWhite,
	LineColor:       tcell.ColorWhite,
	BackgroundColor: tcell.ColorBlack,
}

var lightTheme = themeColors{
	TextColor:       tcell.ColorBlack,
	LineColor:       tcell.ColorBlack,
	BackgroundColor: tcell.ColorWhite,
}

// currentTheme is the active theme, set by SetTheme.
var currentTheme = ThemeDark

// CurrentColors returns the color palette for the active theme.
func CurrentColors() themeColors {
	if currentTheme == ThemeLight {
		return lightTheme
	}

	return darkTheme
}

// SetTheme sets the active theme and applies theme-specific styles.
func SetTheme(theme Theme) {
	currentTheme = theme

	colors := CurrentColors()

	tview.Styles.PrimitiveBackgroundColor = colors.BackgroundColor
	tview.Styles.ContrastBackgroundColor = colors.BackgroundColor
	tview.Styles.MoreContrastBackgroundColor = colors.TextColor
	tview.Styles.PrimaryTextColor = colors.TextColor
	tview.Styles.BorderColor = colors.TextColor
	tview.Styles.TitleColor = colors.TextColor
	tview.Styles.GraphicsColor = colors.TextColor
}
