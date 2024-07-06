// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	backgroundColor         = tcell.Color235
	textNormalColor         = tcell.ColorIvory
	selectedTextColor       = tview.Styles.PrimaryTextColor
	selectedBackgroundColor = tview.Styles.ContrastBackgroundColor
)

// NewTable creates new table.
func NewTable() *Table {
	t := &Table{
		Table:       tview.NewTable(),
		selectedRow: -1,
		hoveredRow:  -1,
		rows:        [][]any{},
	}

	hasFocus := false

	t.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		changed := hasFocus != t.HasFocus()
		if !changed {
			return x, y, width, height
		}

		hasFocus = t.HasFocus()

		if hasFocus {
			if t.selectedRow != -1 {
				t.HoverRow(t.selectedRow)
			} else {
				t.HoverRow(1)
			}
		} else {
			t.HoverRow(-1)
		}

		return x, y, width, height
	})

	t.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		//nolint:exhaustive
		switch e.Key() {
		case tcell.KeyUp:
			if t.hoveredRow > 0 {
				t.HoverRow(t.hoveredRow - 1)
			}
		case tcell.KeyDown:
			if t.hoveredRow < t.GetRowCount() {
				t.HoverRow(t.hoveredRow + 1)
			}
		case tcell.KeyEnter:
			if t.hoveredRow != -1 {
				t.SelectRow(t.hoveredRow)
			}
		}

		return e
	})

	return t
}

// Table list of choices represented in table format.
type Table struct {
	*tview.Table
	selectedRow   int
	hoveredRow    int
	onRowSelected func(row int)
	rows          [][]any
}

// SetHeader sets table header.
func (t *Table) SetHeader(keys ...any) {
	t.AddRow(keys...)
}

// AddRow adds a new row to the table.
func (t *Table) AddRow(columns ...any) {
	row := t.GetRowCount()
	col := backgroundColor
	textColor := tview.Styles.PrimaryTextColor

	if row == 0 {
		col = tcell.ColorSilver
		textColor = tview.Styles.InverseTextColor
	} else {
		t.rows = append(t.rows, columns)
	}

	cell := tview.NewTableCell(" ").
		SetAlign(tview.AlignCenter).
		SetBackgroundColor(col).
		SetTextColor(textColor)

	if row > 0 {
		cell.SetClickedFunc(func() bool {
			t.HoverRow(row)
			t.SelectRow(row)

			return true
		})
	}

	t.SetCell(row, 0, cell)

	for i, text := range columns {
		cell = tview.NewTableCell(text.(string))
		cell.SetExpansion(1)
		cell.SetTextColor(textColor)

		if i == len(columns)-1 {
			cell.SetAlign(tview.AlignRight)
		}

		cell.SetBackgroundColor(col)
		cell.SetSelectable(true)
		t.SetCell(row, i+1, cell)

		if row > 0 {
			cell.SetClickedFunc(func() bool {
				return t.HoverRow(row)
			})
		}
	}
}

// SelectRow selects the row in the table.
func (t *Table) SelectRow(row int) bool {
	// don't select the header
	if row < 2 {
		row = 1
	}

	if row < t.GetRowCount() {
		if t.selectedRow != -1 {
			t.GetCell(t.selectedRow, 0).SetText(" ")
		}

		t.GetCell(row, 0).SetText("►")
		t.selectedRow = row

		if t.onRowSelected != nil {
			t.onRowSelected(row)
		}

		return true
	}

	return false
}

// HoverRow highlights the row in the table.
func (t *Table) HoverRow(row int) bool {
	updateRowStyle := func(r int, foregroundColor, backgroundColor tcell.Color) {
		for i := range t.GetColumnCount() {
			t.GetCell(r, i).SetBackgroundColor(backgroundColor).SetTextColor(foregroundColor)
		}
	}

	// don't select the header
	if row == 0 {
		row = 1
	}

	if row < t.GetRowCount() {
		if t.hoveredRow != -1 {
			updateRowStyle(t.hoveredRow, textNormalColor, backgroundColor)
		}

		if row != -1 {
			updateRowStyle(row, selectedTextColor, selectedBackgroundColor)
		}

		t.hoveredRow = row

		return true
	}

	return false
}

// GetHeight implements Multiline interface.
func (t *Table) GetHeight() int {
	return t.GetRowCount()
}

// SetRowSelectedFunc called when selected row is updated.
func (t *Table) SetRowSelectedFunc(callback func(row int)) {
	t.onRowSelected = callback
}

// GetValue returns value in row/column.
func (t *Table) GetValue(row, column int) any {
	if row < len(t.rows) && column < len(t.rows[row]) {
		return t.rows[row][column]
	}

	return ""
}
