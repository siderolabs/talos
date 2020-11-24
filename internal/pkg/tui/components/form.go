// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewForm creates a new form.
func NewForm() *Form {
	return &Form{
		Flex:      tview.NewFlex().SetDirection(tview.FlexRow),
		formItems: []tview.FormItem{},
	}
}

// Form is a more flexible form component for tview lib.
type Form struct {
	*tview.Flex
	formItems   []tview.FormItem
	maxLabelLen int
}

// AddFormItem adds a new item to the form.
func (f *Form) AddFormItem(item tview.Primitive) {
	if formItem, ok := item.(tview.FormItem); ok {
		f.formItems = append(f.formItems, formItem)
		labelLen := len(formItem.GetLabel()) + 1

		if labelLen > f.maxLabelLen {
			for _, item := range f.formItems[:len(f.formItems)-1] {
				item.SetFormAttributes(
					labelLen,
					tview.Styles.PrimaryTextColor,
					f.GetBackgroundColor(),
					tview.Styles.PrimaryTextColor,
					tview.Styles.ContrastBackgroundColor,
				)
			}

			f.maxLabelLen = labelLen
		}

		formItem.SetFormAttributes(
			f.maxLabelLen,
			tview.Styles.PrimaryTextColor,
			f.GetBackgroundColor(),
			tview.Styles.PrimaryTextColor,
			tview.Styles.ContrastBackgroundColor,
		)
	} else if box, ok := item.(Box); ok {
		box.SetBackgroundColor(f.GetBackgroundColor())
	}

	height := 1
	multiline, ok := item.(Multiline)

	if ok {
		height = multiline.GetHeight()
	}

	f.AddItem(item, height+1, 1, false)
}

// Multiline interface represents elements that can occupy more than one line.
type Multiline interface {
	GetHeight() int
}

// Box interface that has just SetBackgroundColor.
type Box interface {
	SetBackgroundColor(tcell.Color) *tview.Box
}
