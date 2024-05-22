// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"slices"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Diagnostics represents the diagnostics widget.
type Diagnostics struct {
	tview.Grid

	hline *HorizontalLine
	info  *tview.TextView

	selectedNode    string
	perNodeWarnings map[string][]*runtime.Diagnostic
}

// NewDiagnostics initializes Diagnostics.
func NewDiagnostics() *Diagnostics {
	widget := &Diagnostics{
		Grid:            *tview.NewGrid(),
		info:            tview.NewTextView(),
		hline:           NewHorizontalLine("Diagnostics"),
		perNodeWarnings: make(map[string][]*runtime.Diagnostic),
	}

	widget.info.
		SetDynamicColors(true).
		SetBorderPadding(0, 0, 1, 1)

	widget.SetRows(1, 0).SetColumns(0)

	widget.AddItem(widget.hline, 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(widget.info, 1, 0, 1, 1, 0, 0, false)

	return widget
}

// GetCurrentHeight returns the height of the widget.
func (widget *Diagnostics) GetCurrentHeight() int {
	numWarnings := len(widget.perNodeWarnings[widget.selectedNode])
	if numWarnings == 0 {
		return 0
	}

	return 1 + numWarnings
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *Diagnostics) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *Diagnostics) OnResourceDataChange(data resourcedata.Data) {
	r, ok := data.Resource.(*runtime.Diagnostic)
	if !ok {
		return
	}

	idx := slices.IndexFunc(widget.perNodeWarnings[data.Node], func(warning *runtime.Diagnostic) bool {
		return warning.Metadata().ID() == r.Metadata().ID()
	})

	if data.Deleted {
		if idx != -1 {
			widget.perNodeWarnings[data.Node] = slices.Delete(widget.perNodeWarnings[data.Node], idx, idx+1)
		}
	} else {
		if idx == -1 {
			widget.perNodeWarnings[data.Node] = append(widget.perNodeWarnings[data.Node], r)
		} else {
			widget.perNodeWarnings[data.Node][idx] = r
		}
	}

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

// WriteLog writes the log line to the widget.
func (widget *Diagnostics) redraw() {
	widget.info.SetWrap(true)
	widget.info.Clear()

	for _, warning := range widget.perNodeWarnings[widget.selectedNode] {
		widget.info.Write([]byte(fmt.Sprintf("■ (%s) [red]%s[-]\n", //nolint:errcheck
			tview.Escape(warning.TypedSpec().DocumentationURL(warning.Metadata().ID())),
			tview.Escape(warning.TypedSpec().Message))),
		)
	}
}
