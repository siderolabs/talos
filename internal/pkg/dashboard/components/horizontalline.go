// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HorizontalLine is a widget that draws a horizontal line.
type HorizontalLine struct {
	tview.TextView
}

// NewHorizontalLine initializes HorizontalLine.
func NewHorizontalLine() *HorizontalLine {
	widget := &HorizontalLine{
		TextView: *tview.NewTextView(),
	}

	// set the background to be a horizontal line
	widget.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		for i := x; i < x+width; i++ {
			for j := y; j < y+height; j++ {
				screen.SetContent(i, j, tview.BoxDrawingsLightHorizontal, nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
			}
		}

		return x, y, width, height
	})

	return widget
}
