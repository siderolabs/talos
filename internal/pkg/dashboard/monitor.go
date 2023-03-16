// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
)

// MonitorGrid represents the monitoring grid with a process table and various metrics.
type MonitorGrid struct {
	tview.Grid

	app *tview.Application

	apiDataListeners []APIDataListener

	processTableInner *components.ProcessTable
	processTable      *components.TermUIWrapper
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

	graphGrid.AddItem(components.NewTermUIWrapper(cpuGraph), 0, 0, 1, 1, 0, 0, false)
	graphGrid.AddItem(components.NewTermUIWrapper(memGraph), 0, 1, 1, 1, 0, 0, false)
	graphGrid.AddItem(components.NewTermUIWrapper(loadAvgGraph), 0, 2, 1, 1, 0, 0, false)

	bottomGrid := tview.NewGrid().SetRows(0, 0).SetColumns(-1, -3)

	netSparkline := components.NewNetSparkline()
	diskSparkline := components.NewDiskSparkline()

	widget.initProcessTable()

	bottomGrid.AddItem(components.NewTermUIWrapper(netSparkline), 0, 0, 1, 1, 0, 0, false)
	bottomGrid.AddItem(components.NewTermUIWrapper(diskSparkline), 1, 0, 1, 1, 0, 0, false)
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
		widget.processTableInner,
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
		widget.processTableInner.ScrollTop()
		widget.app.SetFocus(widget.processTable)
	}
}

func (widget *MonitorGrid) initProcessTable() {
	widget.processTableInner = components.NewProcessTable()

	widget.processTable = components.NewTermUIWrapper(widget.processTableInner)
	widget.processTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyUp, event.Rune() == 'k':
			widget.processTableInner.ScrollUp()
		case event.Key() == tcell.KeyDown, event.Rune() == 'j':
			widget.processTableInner.ScrollDown()
		case event.Key() == tcell.KeyCtrlU:
			widget.processTableInner.ScrollHalfPageUp()
		case event.Key() == tcell.KeyCtrlD:
			widget.processTableInner.ScrollHalfPageDown()
		case event.Key() == tcell.KeyCtrlB, event.Key() == tcell.KeyPgUp:
			widget.processTableInner.ScrollPageUp()
		case event.Key() == tcell.KeyCtrlF, event.Key() == tcell.KeyPgDn:
			widget.processTableInner.ScrollPageDown()
		}

		return event
	})
}
