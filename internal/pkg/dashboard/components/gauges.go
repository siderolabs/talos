// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"math"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
)

// SystemGauges quickly show CPU/mem load.
type SystemGauges struct {
	*TermUIWrapper

	inner *systemGaugesInner
}

// NewSystemGauges creates SystemGauges.
func NewSystemGauges() *SystemGauges {
	inner := systemGaugesInner{
		Block: *ui.NewBlock(),
	}

	inner.cpuGauge = widgets.NewGauge()
	inner.cpuGauge.Border = false
	inner.cpuGauge.Title = "CPU"

	inner.memGauge = widgets.NewGauge()
	inner.memGauge.Title = "MEM"
	inner.memGauge.Border = false

	wrapper := NewTermUIWrapper(&inner)

	widget := &SystemGauges{
		TermUIWrapper: wrapper,
		inner:         &inner,
	}

	widget.SetBorderPadding(1, 0, 0, 0)

	return widget
}

// Update implements DataWidget interface.
func (widget *SystemGauges) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.inner.cpuGauge.Label = noData
		widget.inner.cpuGauge.Percent = 0
		widget.inner.memGauge.Label = noData
		widget.inner.memGauge.Percent = 0
	} else {
		memUsed := nodeData.MemUsage()

		widget.inner.memGauge.Percent = int(math.Round(memUsed * 100.0))
		widget.inner.memGauge.Label = fmt.Sprintf("%.1f%%", memUsed*100.0)

		cpuUsed := nodeData.CPUUsageByName("usage")

		widget.inner.cpuGauge.Percent = int(math.Round(cpuUsed * 100.0))
		widget.inner.cpuGauge.Label = fmt.Sprintf("%.1f%%", cpuUsed*100.0)
	}
}

type systemGaugesInner struct {
	ui.Block

	cpuGauge *widgets.Gauge
	memGauge *widgets.Gauge
}

// Draw implements io.Drawable.
func (widget *systemGaugesInner) Draw(buf *ui.Buffer) {
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
