// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewFormLabel creates a new FormLabel.
func NewFormLabel(label string) *FormLabel {
	res := &FormLabel{
		tview.NewTextView().SetText(label),
	}

	return res
}

// FormLabel text paragraph that can be used in form.
type FormLabel struct {
	*tview.TextView
}

// SetFormAttributes sets form attributes.
func (b *FormLabel) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	b.SetTextColor(labelColor)
	b.SetBackgroundColor(bgColor)
	s := strings.TrimSpace(b.GetText(false))

	for range labelWidth {
		s = " " + s
	}

	b.SetText(s)

	return b
}

// GetFieldWidth implements tview.FormItem.
func (b *FormLabel) GetFieldWidth() int {
	return 0
}

// SetFinishedFunc implements tview.FormItem.
func (b *FormLabel) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	return b
}

// GetLabel implements tview.FormItem.
func (b *FormLabel) GetLabel() string {
	return ""
}

// GetFieldHeight implements tview.FormItem.
func (b *FormLabel) GetFieldHeight() int {
	return 1
}
