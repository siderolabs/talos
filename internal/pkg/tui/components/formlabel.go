// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
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
