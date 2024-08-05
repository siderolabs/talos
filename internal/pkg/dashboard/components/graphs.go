// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gizak/termui/v3/widgets"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// BaseGraph represents the widget with some usage graph.
type BaseGraph struct {
	widgets.Plot
}

// NewBaseGraph initializes BaseGraph.
func NewBaseGraph(title string, labels []string) *BaseGraph {
	widget := &BaseGraph{
		Plot: *widgets.NewPlot(),
	}

	widget.Border = false
	widget.Title = title
	widget.DataLabels = labels
	widget.ShowAxes = false
	// TODO: looks to be a bug as it requires at least 2 points
	widget.Data = xslices.Map(labels, func(label string) []float64 { return []float64{0, 0} })

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *BaseGraph) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		for i := range widget.Data {
			widget.Data[i] = []float64{0, 0}
		}

		return
	}

	width := widget.Inner.Dx()

	for i, name := range widget.DataLabels {
		series := nodeData.Series[name]

		width = min(width, len(series))

		widget.Data[i] = widget.leftPadSeries(series[len(series)-width:], 2)
	}
}

func (widget *BaseGraph) leftPadSeries(series []float64, size int) []float64 {
	if len(series) >= size {
		return series
	}

	padded := make([]float64, size)
	copy(padded[size-len(series):], series)

	return padded
}

// NewCPUGraph creates CPU usage graph.
func NewCPUGraph() *BaseGraph {
	return NewBaseGraph("CPU USER/SYSTEM", []string{"user", "system"})
}

// NewMemGraph creates mem usage graph.
func NewMemGraph() *BaseGraph {
	return NewBaseGraph("MEM USED", []string{"mem"})
}

// NewLoadAvgGraph creates loadavg graph.
func NewLoadAvgGraph() *BaseGraph {
	return NewBaseGraph("LOAD AVG 60sec", []string{"loadavg"})
}
