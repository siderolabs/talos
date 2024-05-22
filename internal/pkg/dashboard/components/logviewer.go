// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LogViewer represents the logs widget.
type LogViewer struct {
	tview.Grid
	logs tview.TextView
}

// NewLogViewer initializes LogViewer.
func NewLogViewer() *LogViewer {
	widget := &LogViewer{
		Grid: *tview.NewGrid(),
		logs: *tview.NewTextView(),
	}

	widget.logs.ScrollToEnd().
		SetDynamicColors(true).
		SetMaxLines(maxLogLines).
		SetText(noData).
		SetBorderPadding(0, 0, 1, 1).
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			_, _, _, pageSize := widget.logs.GetInnerRect()
			lineOffset, columnOffset := widget.logs.GetScrollOffset()

			//nolint:exhaustive
			switch event.Key() {
			case tcell.KeyCtrlD:
				widget.logs.ScrollTo(lineOffset+(pageSize/2), columnOffset)

				return nil
			case tcell.KeyCtrlU:
				widget.logs.ScrollTo(lineOffset-(pageSize/2), columnOffset)

				return nil
			}

			return event
		})

	widget.SetRows(1, 0).SetColumns(0)

	widget.AddItem(NewHorizontalLine("Logs"), 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(&widget.logs, 1, 0, 1, 1, 0, 0, true)

	return widget
}

// WriteLog writes the log line to the widget.
func (widget *LogViewer) WriteLog(logLine, logError string) {
	if logError != "" {
		logLine = "[red]" + tview.Escape(logError) + "[-]\n"
	} else {
		logLine = tview.Escape(logLine) + "\n"
	}

	widget.logs.Write([]byte(logLine)) //nolint:errcheck
}
