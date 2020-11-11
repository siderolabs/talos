// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewFormLabel creates a new FormLabel.
func NewFormLabel(label string) *FormLabel {
	res := &FormLabel{
		tview.NewTextView().SetText(label),
	}

	res.SetWordWrap(true)

	return res
}

// FormLabel text paragraph that can be used in form.
type FormLabel struct {
	*tview.TextView
}

// GetLabel implements FormItem interface.
func (fl *FormLabel) GetLabel() string {
	return ""
}

// SetFormAttributes implements FormItem interface.
func (fl *FormLabel) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	fl.SetBackgroundColor(bgColor)

	return fl
}

// GetFieldWidth implements FormItem interface.
func (fl *FormLabel) GetFieldWidth() int {
	return 0
}

// SetFinishedFunc implements FormItem interface.
func (fl *FormLabel) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	return fl
}
