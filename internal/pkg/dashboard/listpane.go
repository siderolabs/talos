// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// listPane is the list level of the screens: a table, a summary line and an on-demand filter input.
//
// Screens embed it. The filter methods only report what they did: rendering and focus stay with
// the screen.
type listPane struct {
	listWrapper  *tview.Grid
	table        *tview.Table
	summary      *tview.TextView
	filterInput  *tview.InputField
	filterActive bool
	filterText   string
}

// newListPane builds the list level. onSelect is called with the table row the user picked,
// onFilterChanged after the filter text changed, and onFilterDone when the input is left: with
// true if the filter is to be dropped as well.
func newListPane(onSelect func(row int), onFilterChanged func(), onFilterDone func(clearText bool)) *listPane {
	pane := &listPane{
		table:       newSelectableTable(),
		summary:     tview.NewTextView(),
		filterInput: tview.NewInputField(),
	}

	pane.table.SetSelectedFunc(func(row, _ int) {
		onSelect(row)
	})

	pane.summary.SetDynamicColors(true)
	pane.summary.SetBorderPadding(0, 0, 1, 1)

	pane.filterInput.SetLabel("filter: ")
	pane.filterInput.SetLabelColor(tcell.ColorYellow)
	pane.filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	pane.filterInput.SetChangedFunc(func(text string) {
		pane.filterText = text

		onFilterChanged()
	})
	pane.filterInput.SetDoneFunc(func(key tcell.Key) {
		onFilterDone(key == tcell.KeyEscape)
	})

	pane.listWrapper = tview.NewGrid()
	pane.listWrapper.SetRows(0, 1).SetColumns(0)
	pane.listWrapper.AddItem(pane.table, 0, 0, 1, 1, 0, 0, true)
	pane.listWrapper.AddItem(pane.summary, 1, 0, 1, 1, 0, 0, false)

	return pane
}

// showFilter shows the filter input below the list and focuses it.
func (pane *listPane) showFilter(app *tview.Application) {
	if pane.filterActive {
		return
	}

	pane.filterActive = true
	pane.filterInput.SetText(pane.filterText)
	pane.listWrapper.SetRows(0, 1, 1)
	pane.listWrapper.AddItem(pane.filterInput, 2, 0, 1, 1, 0, 0, true)
	app.SetFocus(pane.filterInput)
}

// hideFilter hides the filter input, clearing the filter if clearText is true.
//
// It reports whether the input was shown and whether the list needs rendering again.
func (pane *listPane) hideFilter(clearText bool) (wasActive, rerender bool) {
	rerender = clearText && (pane.filterActive || pane.filterText != "")

	if clearText {
		pane.filterText = ""
		pane.filterInput.SetText("")
	}

	if !pane.filterActive {
		return false, rerender
	}

	pane.filterActive = false
	pane.listWrapper.RemoveItem(pane.filterInput)
	pane.listWrapper.SetRows(0, 1)

	return true, rerender
}

// newSelectableTable returns a bordered table with a fixed header row and a selectable row cursor.
func newSelectableTable() *tview.Table {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetFixed(1, 0)
	table.SetSelectable(true, false)
	table.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	return table
}

// newDetailGrid lays out the detail level: a pane of a fixed height on top of the log viewer.
func newDetailGrid(top tview.Primitive, topRows int, logs tview.Primitive) *tview.Grid {
	grid := tview.NewGrid()
	grid.SetRows(topRows, 0).SetColumns(0)
	grid.AddItem(top, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(logs, 1, 0, 1, 1, 0, 0, true)

	return grid
}
