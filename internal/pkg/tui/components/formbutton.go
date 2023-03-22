// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewFormModalButton creates a new FormModalButton.
func NewFormModalButton(formLabel, buttonLabel string) *FormModalButton {
	res := &FormModalButton{
		Flex:   tview.NewFlex(),
		button: tview.NewButton(buttonLabel),
		label:  tview.NewTextView(),
	}

	res.label.SetText(formLabel)
	res.AddItem(res.label, 0, 1, false)
	res.AddItem(res.button, len(buttonLabel)+2, 1, false)

	return res
}

// FormModalButton the button that opens modal dialog with extended settings.
type FormModalButton struct {
	*tview.Flex

	label  *tview.TextView
	button *tview.Button
}

// SetSelectedFunc forwards that to underlying button component.
func (b *FormModalButton) SetSelectedFunc(handler func()) *FormModalButton {
	b.button.SetSelectedFunc(handler)

	return b
}

// Focus override default focus behavior.
func (b *FormModalButton) Focus(delegate func(tview.Primitive)) {
	b.button.Focus(delegate)
}

// Blur override default blur behavior.
func (b *FormModalButton) Blur() {
	b.button.Blur()
}

// SetFormAttributes sets form attributes.
func (b *FormModalButton) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	b.label.SetTextColor(labelColor)
	b.label.SetBackgroundColor(bgColor)
	b.SetBackgroundColor(bgColor)
	b.ResizeItem(b.label, labelWidth, 1)

	return b
}

// GetFieldWidth implements tview.FormItem.
func (b *FormModalButton) GetFieldWidth() int {
	return 0
}

// GetFieldHeight implements tview.FormItem.
func (b *FormModalButton) GetFieldHeight() int {
	return 1
}

// SetFinishedFunc implements tview.FormItem.
func (b *FormModalButton) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	return b
}

// SetDisabled implements tview.FormItem.
func (b *FormModalButton) SetDisabled(disabled bool) tview.FormItem {
	b.button.SetDisabled(disabled)

	return b
}

// GetLabel implements tview.FormItem.
func (b *FormModalButton) GetLabel() string {
	return b.label.GetText(true)
}
