// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
)

// MonitorGrid represents the monitoring grid with a process table and various metrics.
type MonitorGrid struct {
	tview.Grid

	app *tview.Application

	apiDataListeners []APIDataListener

	processTable *components.ProcessTable
}

// NewMonitorGrid initializes MonitorGrid.
func NewMonitorGrid(app *tview.Application) *MonitorGrid {
	widget := &MonitorGrid{
		app:  app,
		Grid: *tview.NewGrid(),
	}

	widget.SetRows(7, -1, -2).SetColumns(0)

	infoGrid := tview.NewGrid().SetRows(0).SetColumns(-1, -2, -1, -1, -2)

	sysGauges := components.NewSystemGauges()
	cpuInfo := components.NewCPUInfo()
	loadAvgInfo := components.NewLoadAvgInfo()
	procsInfo := components.NewProcsInfo()
	memInfo := components.NewMemInfo()

	infoGrid.AddItem(sysGauges, 0, 0, 1, 1, 0, 0, false)
	infoGrid.AddItem(cpuInfo, 0, 1, 1, 1, 0, 0, false)
	infoGrid.AddItem(loadAvgInfo, 0, 2, 1, 1, 0, 0, false)
	infoGrid.AddItem(procsInfo, 0, 3, 1, 1, 0, 0, false)
	infoGrid.AddItem(memInfo, 0, 4, 1, 1, 0, 0, false)

	graphGrid := tview.NewGrid().SetRows(0).SetColumns(0, 0, 0)

	cpuGraph := components.NewCPUGraph()
	memGraph := components.NewMemGraph()
	loadAvgGraph := components.NewLoadAvgGraph()

	graphGrid.AddItem(cpuGraph, 0, 0, 1, 1, 0, 0, false)
	graphGrid.AddItem(memGraph, 0, 1, 1, 1, 0, 0, false)
	graphGrid.AddItem(loadAvgGraph, 0, 2, 1, 1, 0, 0, false)

	bottomGrid := tview.NewGrid().SetRows(0, 0).SetColumns(-1, -3)

	netSparkline := components.NewNetSparkline()
	diskSparkline := components.NewDiskSparkline()

	widget.processTable = components.NewProcessTable()

	bottomGrid.AddItem(netSparkline, 0, 0, 1, 1, 0, 0, false)
	bottomGrid.AddItem(diskSparkline, 1, 0, 1, 1, 0, 0, false)
	bottomGrid.AddItem(widget.processTable, 0, 1, 2, 1, 0, 0, false)

	widget.AddItem(infoGrid, 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(graphGrid, 1, 0, 1, 1, 0, 0, false)
	widget.AddItem(bottomGrid, 2, 0, 1, 1, 0, 0, false)

	widget.apiDataListeners = []APIDataListener{
		sysGauges,
		cpuInfo,
		loadAvgInfo,
		procsInfo,
		memInfo,
		cpuGraph,
		memGraph,
		loadAvgGraph,
		netSparkline,
		diskSparkline,
		widget.processTable,
	}

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *MonitorGrid) OnAPIDataChange(node string, data *apidata.Data) {
	for _, dataWidget := range widget.apiDataListeners {
		dataWidget.OnAPIDataChange(node, data)
	}
}

// OnScreenSelect implements the screenSelectListener interface.
func (widget *MonitorGrid) onScreenSelect(active bool) {
	if active {
		widget.processTable.ScrollToBeginning()
		widget.processTable.Select(1, 0)
		widget.app.SetFocus(widget.processTable)
	}
}
