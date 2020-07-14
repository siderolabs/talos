// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
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
	widget.Data = make([][]float64, len(labels))

	// TODO: looks to be a bug as it requires at least 2 points
	for i := range widget.Data {
		widget.Data[i] = []float64{0, 0}
	}

	return widget
}

// Update implements the DataWidget interface.
func (widget *BaseGraph) Update(node string, data *data.Data) {
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

		if len(series) < width {
			width = len(series)
		}

		widget.Data[i] = series[len(series)-width:]
	}
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
