// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import "github.com/rivo/tview"

// NewGroup creates new Group.
func NewGroup(app *tview.Application) *Group {
	return &Group{
		app:      app,
		current:  -1,
		elements: []tview.Primitive{},
	}
}

// Group is an attempt to straighten out built-in tview TAB focus sequence.
type Group struct {
	app      *tview.Application
	elements []tview.Primitive
	current  int
	focus    tview.Primitive
}

// AddElement to the group.
func (eg *Group) AddElement(element tview.Primitive) tview.Primitive {
	eg.elements = append(eg.elements, element)

	return element
}

// FocusFirst sets focus to the first element.
func (eg *Group) FocusFirst() {
	if len(eg.elements) == 0 {
		return
	}

	eg.current = 0
	eg.focus = eg.elements[eg.current]
	eg.app.SetFocus(eg.focus)
}

// NextFocus switch focus to the next element.
func (eg *Group) NextFocus() {
	eg.detectFocus()
	eg.current = (eg.current + 1) % (len(eg.elements))
	eg.focus = eg.elements[eg.current]
	eg.app.SetFocus(eg.focus)
}

// PrevFocus switch focus to the prev element.
func (eg *Group) PrevFocus() {
	eg.detectFocus()

	eg.current--

	if eg.current < 0 {
		eg.current = len(eg.elements) - 1
	}

	eg.focus = eg.elements[eg.current]
	eg.app.SetFocus(eg.focus)
}

func (eg *Group) detectFocus() {
	focused := eg.app.GetFocus()
	if eg.focus == nil || focused != eg.focus {
		for i, e := range eg.elements {
			if e == focused {
				eg.current = i

				break
			}
		}
	}
}
