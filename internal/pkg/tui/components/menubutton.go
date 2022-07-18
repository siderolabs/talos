// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewMenuButton creates new menu button.
func NewMenuButton(label string) *MenuButton {
	button := &MenuButton{
		Button: tview.NewButton(label),
		label:  label,
		colors: [4]tcell.Color{
			tcell.ColorBlack,
			tcell.ColorBlack,
			tcell.ColorBlack,
			tcell.ColorBlack,
		},
		active: false,
	}

	button.updateColors()

	return button
}

const (
	menuButtonBgColor = iota
	menuButtonLabelColor
	menuButtonActiveBgColor
	menuButtonActiveLabelColor
)

// MenuButton creates a new menu button.
type MenuButton struct {
	*tview.Button
	label  string
	colors [4]tcell.Color
	active bool
}

// SetActiveColors sets active state colors.
// 1st value is bg color.
// 2nd value is label color.
func (b *MenuButton) SetActiveColors(colors ...tcell.Color) {
	for i, color := range colors {
		b.colors[i+2] = color
	}

	b.updateColors()
}

// SetInactiveColors sets inactive state colors.
// 1st value is bg color.
// 2nd value is label color.
func (b *MenuButton) SetInactiveColors(colors ...tcell.Color) {
	copy(b.colors[:], colors)

	b.updateColors()
}

// SetActive changes menu button active state.
func (b *MenuButton) SetActive(active bool) {
	b.active = active
	b.updateColors()

	format := "%s"

	if b.active {
		format = "[::u]%s[::-]"
	}

	b.SetLabel(fmt.Sprintf(format, b.label))
}

func (b *MenuButton) updateColors() {
	if b.active {
		b.SetLabelColor(b.colors[menuButtonActiveLabelColor])
		b.SetLabelColorActivated(b.colors[menuButtonActiveLabelColor])
		b.SetBackgroundColor(b.colors[menuButtonActiveBgColor])
		b.SetBackgroundColorActivated(b.colors[menuButtonActiveBgColor])
	} else {
		b.SetLabelColor(b.colors[menuButtonLabelColor])
		b.SetLabelColorActivated(b.colors[menuButtonLabelColor])
		b.SetBackgroundColor(b.colors[menuButtonBgColor])
		b.SetBackgroundColorActivated(b.colors[menuButtonBgColor])
	}
}
