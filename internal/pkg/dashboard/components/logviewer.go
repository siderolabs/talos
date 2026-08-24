// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LogViewer represents the logs widget.
type LogViewer struct {
	tview.Grid

	app *tview.Application

	logs tview.TextView

	entries []logEntry

	filterInput  *tview.InputField
	filterActive bool
	filterText   string
}

// logEntry holds a single raw log line, kept so the view can be re-filtered.
type logEntry struct {
	text    string
	isError bool
}

// NewLogViewer initializes LogViewer.
func NewLogViewer(app *tview.Application) *LogViewer {
	widget := &LogViewer{
		Grid: *tview.NewGrid(),
		app:  app,
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

			if event.Rune() == '/' {
				widget.activateSearch()

				return nil
			}

			return event
		})

	widget.filterInput = tview.NewInputField()
	widget.filterInput.SetLabel("filter: ")
	widget.filterInput.SetLabelColor(tcell.ColorYellow)
	widget.filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	widget.filterInput.SetChangedFunc(func(text string) {
		widget.filterText = text
		widget.renderLogs()
	})
	widget.filterInput.SetDoneFunc(func(key tcell.Key) {
		widget.deactivateSearch(key == tcell.KeyEscape)
	})

	widget.SetRows(1, 0).SetColumns(0)

	widget.AddItem(NewHorizontalLine("Logs (/: filter)"), 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(&widget.logs, 1, 0, 1, 1, 0, 0, true)

	return widget
}

// activateSearch shows the search input below the log view.
func (widget *LogViewer) activateSearch() {
	if widget.filterActive {
		widget.app.SetFocus(widget.filterInput)

		return
	}

	widget.filterActive = true
	widget.filterInput.SetText(widget.filterText)
	widget.SetRows(1, 0, 1)
	widget.AddItem(widget.filterInput, 2, 0, 1, 1, 0, 0, true)
	widget.app.SetFocus(widget.filterInput)
}

// deactivateSearch hides the search input. If clearText is true, the filter is also cleared.
func (widget *LogViewer) deactivateSearch(clearText bool) {
	if !widget.filterActive {
		if clearText && widget.filterText != "" {
			widget.filterText = ""
			widget.filterInput.SetText("")
			widget.renderLogs()
		}

		return
	}

	hadFilter := widget.filterText != ""

	if clearText {
		widget.filterText = ""
		widget.filterInput.SetText("")
	}

	widget.filterActive = false
	widget.RemoveItem(widget.filterInput)
	widget.SetRows(1, 0)
	widget.app.SetFocus(&widget.logs)

	if clearText && hadFilter {
		widget.renderLogs()
	}
}

// WriteLog writes the log line to the widget.
func (widget *LogViewer) WriteLog(logLine, logError string) {
	entry := logEntry{text: logLine, isError: logError != ""}
	if entry.isError {
		entry.text = logError
	}

	// drop the noData placeholder before the first line is appended
	if len(widget.entries) == 0 {
		widget.logs.Clear()
	}

	widget.entries = append(widget.entries, entry)
	if len(widget.entries) > maxLogLines {
		widget.entries = widget.entries[len(widget.entries)-maxLogLines:]
	}

	if formatted, ok := formatLogEntry(entry, widget.filterText); ok {
		widget.logs.Write([]byte(formatted)) //nolint:errcheck
	}
}

// renderLogs rebuilds the log view from the buffered entries, applying the current filter.
func (widget *LogViewer) renderLogs() {
	widget.logs.Clear()

	for _, entry := range widget.entries {
		if formatted, ok := formatLogEntry(entry, widget.filterText); ok {
			widget.logs.Write([]byte(formatted)) //nolint:errcheck
		}
	}

	widget.logs.ScrollToEnd()
}

// lowerWithOffsets lowercases text and returns, for each byte index of the result,
// the byte index in text it originated from. The returned slice has one extra
// trailing entry equal to len(text), so the end of a match is always addressable.
func lowerWithOffsets(text string) (string, []int) {
	var sb strings.Builder

	sb.Grow(len(text))

	offsets := make([]int, 0, len(text)+1)

	for i, r := range text {
		before := sb.Len()

		sb.WriteRune(unicode.ToLower(r))

		for range sb.Len() - before {
			offsets = append(offsets, i)
		}
	}

	return sb.String(), append(offsets, len(text))
}

// formatLogEntry renders a single log entry as tview markup, highlighting filter
// matches. The second return value is false when filter is non-empty and the
// entry doesn't match, meaning the line should not be displayed.
func formatLogEntry(entry logEntry, filter string) (string, bool) {
	if filter == "" {
		if entry.isError {
			return "[red]" + tview.Escape(entry.text) + "[-]\n", true
		}

		return tview.Escape(entry.text) + "\n", true
	}

	lowText, offsets := lowerWithOffsets(entry.text)
	lowFilter := strings.ToLower(filter)

	if !strings.Contains(lowText, lowFilter) {
		return "", false
	}

	baseColor := ""
	if entry.isError {
		baseColor = "[red]"
	}

	var sb strings.Builder

	sb.WriteString(baseColor)

	// search in the lowercased text, but always slice the original text through
	// offsets: lowercasing may change byte lengths, so a lowercased index is never
	// a valid index into entry.text.
	searchStart := 0

	for {
		rel := strings.Index(lowText[searchStart:], lowFilter)
		if rel < 0 {
			sb.WriteString(tview.Escape(entry.text[offsets[searchStart]:]))

			break
		}

		matchLow := searchStart + rel
		matchStart, matchEnd := offsets[matchLow], offsets[matchLow+len(lowFilter)]

		sb.WriteString(tview.Escape(entry.text[offsets[searchStart]:matchStart]))
		sb.WriteString("[yellow]")
		sb.WriteString(tview.Escape(entry.text[matchStart:matchEnd]))
		sb.WriteString("[-]")
		sb.WriteString(baseColor)

		searchStart = matchLow + len(lowFilter)
	}

	if baseColor != "" {
		sb.WriteString("[-]")
	}

	sb.WriteString("\n")

	return sb.String(), true
}
