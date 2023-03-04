// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"sync"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
)

// SummaryGrid represents the summary grid with the basic node information and the logs.
type SummaryGrid struct {
	tview.Grid

	app *tview.Application

	dataWidgets []DataWidget

	lock       sync.Mutex
	active     bool
	node       string
	logViewers map[string]*components.LogViewer
}

// NewSummaryGrid initializes SummaryGrid.
func NewSummaryGrid(app *tview.Application) *SummaryGrid {
	widget := &SummaryGrid{
		app:        app,
		Grid:       *tview.NewGrid(),
		logViewers: make(map[string]*components.LogViewer),
	}

	widget.SetRows(8, 0).
		SetColumns(0, 0, 0)

	talosInfo := components.NewTalosInfo()
	widget.AddItem(talosInfo, 0, 0, 1, 1, 0, 0, false)

	kubernetesInfo := components.NewKubernetesInfo()
	widget.AddItem(kubernetesInfo, 0, 1, 1, 1, 0, 0, false)

	networkInfo := components.NewNetworkInfo()
	widget.AddItem(networkInfo, 0, 2, 1, 1, 0, 0, false)

	widget.dataWidgets = []DataWidget{
		talosInfo,
		networkInfo,
		kubernetesInfo,
	}

	return widget
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *SummaryGrid) OnNodeSelect(node string) {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.node = node

	widget.updateLogViewer()
}

// Update implements the DataWidget interface.
func (widget *SummaryGrid) Update(node string, data *data.Data) {
	for _, dataWidget := range widget.dataWidgets {
		dataWidget.Update(node, data)
	}
}

// UpdateLog implements the LogWidget interface.
func (widget *SummaryGrid) UpdateLog(node string, logLine string) {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.logViewer(node).WriteLog(logLine)
}

func (widget *SummaryGrid) updateLogViewer() {
	if !widget.active {
		return
	}

	widget.logViewer(widget.node)

	for currNode, logViewer := range widget.logViewers {
		if currNode == widget.node {
			widget.AddItem(logViewer, 1, 0, 1, 3, 0, 0, false)

			widget.app.SetFocus(logViewer)

			return
		}

		widget.RemoveItem(logViewer)
	}
}

func (widget *SummaryGrid) logViewer(node string) *components.LogViewer {
	logViewer, ok := widget.logViewers[node]
	if !ok {
		logViewer = components.NewLogViewer()

		widget.logViewers[node] = logViewer
	}

	return logViewer
}

// OnScreenSelect implements the screenSelectListener interface.
func (widget *SummaryGrid) onScreenSelect(active bool) {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.active = active

	widget.updateLogViewer()
}
