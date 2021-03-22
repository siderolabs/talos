// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"

	ui "github.com/gizak/termui/v3"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/components"
	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// DataWidget is a widget which consumes Data to draw itself.
type DataWidget interface {
	Update(node string, data *data.Data)
}

// UI represents the grid, widgets and main loop.
type UI struct {
	infoGrid *ui.Grid
	grid     *ui.Grid

	sysGauges   *components.SystemGauges
	cpuInfo     *components.CPUInfo
	loadAvgInfo *components.LoadAvgInfo
	procsInfo   *components.ProcsInfo
	memInfo     *components.MemInfo

	cpuGraph     *components.BaseGraph
	memGraph     *components.BaseGraph
	loadAvgGraph *components.BaseGraph

	netSparkline  *components.BaseSparklineGroup
	diskSparkline *components.BaseSparklineGroup
	procTable     *components.ProcessTable

	topLine *components.TopLine
	tabs    *components.NodeTabs

	drawable    []ui.Drawable
	dataWidgets []DataWidget

	data *data.Data
}

// Main is the UI entrypoint.
//
//nolint:gocyclo
func (u *UI) Main(ctx context.Context, dataCh <-chan *data.Data) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	ui.Theme.Block.Title.Modifier = ui.ModifierBold

	u.topLine = components.NewTopLine()
	u.tabs = components.NewNodeTabs()

	u.sysGauges = components.NewSystemGauges()
	u.cpuInfo = components.NewCPUInfo()
	u.loadAvgInfo = components.NewLoadAvgInfo()
	u.procsInfo = components.NewProcsInfo()
	u.memInfo = components.NewMemInfo()

	u.cpuGraph = components.NewCPUGraph()
	u.memGraph = components.NewMemGraph()
	u.loadAvgGraph = components.NewLoadAvgGraph()

	u.netSparkline = components.NewNetSparkline()
	u.diskSparkline = components.NewDiskSparkline()
	u.procTable = components.NewProcessTable()

	u.infoGrid = ui.NewGrid()
	u.infoGrid.Set(
		ui.NewRow(1,
			ui.NewCol(1.0/7, u.sysGauges),
			ui.NewCol(2.0/7, u.cpuInfo),
			ui.NewCol(1.0/7, u.loadAvgInfo),
			ui.NewCol(1.0/7, u.procsInfo),
			ui.NewCol(2.0/7, u.memInfo),
		),
	)

	u.grid = ui.NewGrid()
	u.grid.Set(
		ui.NewRow(1.0/3,
			ui.NewCol(1.0/3, u.cpuGraph),
			ui.NewCol(1.0/3, u.memGraph),
			ui.NewCol(1.0/3, u.loadAvgGraph),
		),
		ui.NewRow(2.0/3,
			ui.NewCol(1.0/4,
				ui.NewRow(1.0/2, u.netSparkline),
				ui.NewRow(1.0/2, u.diskSparkline),
			),
			ui.NewCol(3.0/4, u.procTable),
		),
	)

	termWidth, termHeight := ui.TerminalDimensions()
	u.Resize(termWidth, termHeight)

	u.dataWidgets = []DataWidget{
		u.topLine,
		u.cpuInfo,
		u.loadAvgInfo,
		u.procsInfo,
		u.memInfo,
		u.sysGauges,
		u.cpuGraph,
		u.memGraph,
		u.loadAvgGraph,
		u.netSparkline,
		u.diskSparkline,
		u.procTable,
	}

	u.drawable = []ui.Drawable{u.topLine, u.infoGrid, u.grid, u.tabs}
	ui.Render(u.drawable...)

	uiEvents := ui.PollEvents()

	var ok bool

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			case "<Resize>":
				payload := e.Payload.(ui.Resize) //nolint:errcheck,forcetypeassert

				u.Resize(payload.Width, payload.Height)
				ui.Clear()
				ui.Render(u.drawable...)
			case "h", "<Left>":
				u.tabs.FocusLeft()
				u.procTable.ScrollTop()
				u.UpdateData()
			case "l", "<Right>":
				u.tabs.FocusRight()
				u.procTable.ScrollTop()
				u.UpdateData()
			case "j", "<Down>":
				u.procTable.ScrollDown()
				ui.Render(u.procTable)
			case "k", "<Up>":
				u.procTable.ScrollUp()
				ui.Render(u.procTable)
			case "<C-d>":
				u.procTable.ScrollHalfPageDown()
				ui.Render(u.procTable)
			case "<C-u>":
				u.procTable.ScrollHalfPageUp()
				ui.Render(u.procTable)
			case "<C-f>":
				u.procTable.ScrollPageDown()
				ui.Render(u.procTable)
			case "<C-b>":
				u.procTable.ScrollPageUp()
				ui.Render(u.procTable)
			}
		case u.data, ok = <-dataCh:
			if !ok {
				return nil
			}

			u.UpdateData()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Resize handles the resize events.
func (u *UI) Resize(width, height int) {
	u.topLine.SetRect(0, 0, width, 1)
	u.infoGrid.SetRect(0, 2, width, 9)
	u.grid.SetRect(0, 9, width, height-1)
	u.tabs.SetRect(0, height-1, width, height)
}

// UpdateData re-renders the widgets with new data.
func (u *UI) UpdateData() {
	if u.data == nil {
		return
	}

	node := ""
	u.tabs.Update(node, u.data)

	if len(u.tabs.TabNames) > 0 {
		node = u.tabs.TabNames[u.tabs.ActiveTabIndex]
	}

	for _, widget := range u.dataWidgets {
		widget.Update(node, u.data)
	}

	ui.Render(u.drawable...)
}
