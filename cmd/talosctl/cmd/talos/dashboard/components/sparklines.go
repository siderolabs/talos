// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// BaseSparklineGroup represents the widget with some sparklines.
type BaseSparklineGroup struct {
	widgets.SparklineGroup

	dataLabels []string
}

// NewBaseSparklineGroup initializes BaseSparklineGroup.
func NewBaseSparklineGroup(title string, labels, dataLabels []string) *BaseSparklineGroup {
	sparklines := make([]*widgets.Sparkline, len(labels))

	for i := range sparklines {
		sparklines[i] = widgets.NewSparkline()
		sparklines[i].Title = labels[i]
		sparklines[i].Data = []float64{0, 0}
		sparklines[i].LineColor = ui.Theme.Plot.Lines[i]
	}

	widget := &BaseSparklineGroup{
		SparklineGroup: *widgets.NewSparklineGroup(sparklines...),
		dataLabels:     dataLabels,
	}

	widget.Border = false
	widget.Title = title

	return widget
}

// Update implements the DataWidget interface.
func (widget *BaseSparklineGroup) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		for i := range widget.Sparklines {
			widget.Sparklines[i].Data = []float64{0, 0}
		}

		return
	}

	width := widget.Inner.Dx()

	for i, name := range widget.dataLabels {
		series := nodeData.Series[name]

		if len(series) < width {
			width = len(series)
		}

		widget.Sparklines[i].Data = series[len(series)-width:]
	}
}

// NewNetSparkline creates network sparkline.
func NewNetSparkline() *BaseSparklineGroup {
	return NewBaseSparklineGroup("NET", []string{"RX", "TX"}, []string{"netrxbytes", "nettxbytes"})
}

// NewDiskSparkline creates disk sparkline.
func NewDiskSparkline() *BaseSparklineGroup {
	return NewBaseSparklineGroup("DISK", []string{"READ", "WRITE"}, []string{"diskrdsectors", "diskwrsectors"})
}
