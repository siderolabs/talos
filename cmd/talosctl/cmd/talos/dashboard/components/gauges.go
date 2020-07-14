// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"math"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// SystemGauges quickly show CPU/mem load.
type SystemGauges struct {
	ui.Block

	cpuGauge *widgets.Gauge
	memGauge *widgets.Gauge
}

// NewSystemGauges creates SystemGauges.
func NewSystemGauges() *SystemGauges {
	widget := &SystemGauges{
		Block: *ui.NewBlock(),
	}

	widget.cpuGauge = widgets.NewGauge()
	widget.cpuGauge.Border = false
	widget.cpuGauge.Title = "CPU"
	widget.memGauge = widgets.NewGauge()
	widget.memGauge.Title = "MEM"
	widget.memGauge.Border = false

	return widget
}

// Update implements DataWidget interface.
func (widget *SystemGauges) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.cpuGauge.Label = noData
		widget.cpuGauge.Percent = 0
		widget.memGauge.Label = noData
		widget.memGauge.Percent = 0
	} else {
		memUsed := nodeData.MemUsage()

		widget.memGauge.Percent = int(math.Round(memUsed * 100.0))
		widget.memGauge.Label = fmt.Sprintf("%.1f%%", memUsed*100.0)

		cpuUsed := nodeData.CPUUsageByName("usage")

		widget.cpuGauge.Percent = int(math.Round(cpuUsed * 100.0))
		widget.cpuGauge.Label = fmt.Sprintf("%.1f%%", cpuUsed*100.0)
	}
}

// Draw implements io.Drawable.
func (widget *SystemGauges) Draw(buf *ui.Buffer) {
	width := widget.Dx()
	height := widget.Dy()

	y := 0
	itemHeight := 2

	for _, item := range []ui.Drawable{widget.cpuGauge, widget.memGauge} {
		item.SetRect(widget.Min.X, widget.Min.Y+y, widget.Min.X+width, widget.Min.Y+y+itemHeight+1)
		item.Draw(buf)

		y += itemHeight

		if y > height {
			break
		}
	}
}
