// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// SystemGauges quickly show CPU/mem load.
type SystemGauges struct {
	tview.Primitive

	cpuGauge *tvxwidgets.PercentageModeGauge
	memGauge *tvxwidgets.PercentageModeGauge
}

// NewSystemGauges creates SystemGauges.
func NewSystemGauges() *SystemGauges {
	root := tview.NewGrid().SetRows(0).SetColumns(0)
	root.SetBorderPadding(1, 2, 1, 1)

	cpuGauge := tvxwidgets.NewPercentageModeGauge()
	cpuGauge.SetBorder(false)
	cpuGauge.SetMaxValue(100)
	cpuGauge.SetPgBgColor(tview.Styles.ContrastBackgroundColor)

	cpuFrame := tview.NewFrame(cpuGauge)
	cpuFrame.SetBorders(0, 0, 0, 0, 0, 0).
		AddText("[::b]CPU", true, tview.AlignLeft, tcell.ColorDefault)

	root.AddItem(cpuFrame, 0, 0, 1, 1, 0, 0, false)

	memGauge := tvxwidgets.NewPercentageModeGauge()
	memGauge.SetBorder(false)
	memGauge.SetMaxValue(100)
	memGauge.SetPgBgColor(tview.Styles.ContrastBackgroundColor)

	memFrame := tview.NewFrame(memGauge)
	memFrame.SetBorders(0, 0, 0, 0, 0, 0).
		AddText("[::b]MEM", true, tview.AlignLeft, tcell.ColorDefault)

	root.AddItem(memFrame, 1, 0, 1, 1, 0, 0, false)

	widget := &SystemGauges{
		Primitive: root,

		cpuGauge: cpuGauge,
		memGauge: memGauge,
	}

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *SystemGauges) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.cpuGauge.SetValue(0)
		widget.memGauge.SetValue(0)
	} else {
		memUsed := nodeData.MemUsage()
		widget.memGauge.SetValue(int(math.Round(memUsed * 100.0)))

		cpuUsed := nodeData.CPUUsageByName("usage")
		widget.cpuGauge.SetValue(int(math.Round(cpuUsed * 100.0)))
	}
}
