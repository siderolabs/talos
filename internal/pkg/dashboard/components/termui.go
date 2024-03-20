// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"image"

	"github.com/gdamore/tcell/v2"
	"github.com/gizak/termui/v3"
	"github.com/rivo/tview"
)

// TermUIWrapper is a custom tview component that wraps a legacy termui component and draws it.
type TermUIWrapper struct {
	*tview.Box
	termUIDrawable termui.Drawable
}

// NewTermUIWrapper initializes a new TermUIWrapper.
func NewTermUIWrapper(drawable termui.Drawable) *TermUIWrapper {
	return &TermUIWrapper{
		Box:            tview.NewBox(),
		termUIDrawable: drawable,
	}
}

// Draw implements the tview.Primitive interface.
func (w *TermUIWrapper) Draw(screen tcell.Screen) {
	w.Box.DrawForSubclass(screen, w)
	x, y, width, height := w.GetInnerRect()

	if width == 0 || height == 0 {
		return
	}

	w.termUIDrawable.SetRect(0, 0, width, height)
	buf := termui.NewBuffer(w.termUIDrawable.GetRect())
	w.termUIDrawable.Draw(buf)

	for i := range width {
		for j := range height {
			cell := buf.GetCell(image.Point{X: i, Y: j})

			style := w.convertStyle(cell.Style)

			screen.SetContent(i+x, j+y, cell.Rune, nil, style)
		}
	}
}

// convertStyle converts termui style to tcell (tview) style.
func (w *TermUIWrapper) convertStyle(style termui.Style) tcell.Style {
	fgColor := w.convertColor(style.Fg)
	bgColor := w.convertColor(style.Bg)

	bold := false
	if style.Modifier&termui.ModifierBold != 0 {
		bold = true
	}

	underline := false
	if style.Modifier&termui.ModifierUnderline != 0 {
		underline = true
	}

	reverse := false
	if style.Modifier&termui.ModifierReverse != 0 {
		reverse = true
	}

	return tcell.StyleDefault.Foreground(fgColor).Background(bgColor).Bold(bold).Underline(underline).Reverse(reverse)
}

// convertColor converts termui color to tcell (tview) color.
func (w *TermUIWrapper) convertColor(color termui.Color) tcell.Color {
	switch color {
	case termui.ColorClear:
		return tcell.ColorDefault
	case termui.ColorBlack:
		return tcell.ColorBlack
	case termui.ColorRed:
		return tcell.ColorRed
	case termui.ColorGreen:
		return tcell.ColorGreen
	case termui.ColorYellow:
		return tcell.ColorYellow
	case termui.ColorBlue:
		return tcell.ColorBlue
	case termui.ColorMagenta:
		return tcell.ColorPurple
	case termui.ColorCyan:
		return tcell.ColorTeal
	case termui.ColorWhite:
		return tcell.ColorWhite
	default:
		return tcell.ColorDefault
	}
}
