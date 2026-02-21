// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// BaseGraph represents the widget with some usage graph.
type BaseGraph struct {
	tview.Primitive

	plot   *tvxwidgets.Plot
	labels []string
}

// NewBaseGraph initializes BaseGraph.
func NewBaseGraph(title string, labels []string) *BaseGraph {
	widget := &BaseGraph{
		plot:   tvxwidgets.NewPlot(),
		labels: labels,
	}

	root := tview.NewFrame(widget.plot).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(title, true, tview.AlignCenter, tcell.ColorDefault)

	widget.plot.SetBorder(false)
	widget.plot.SetLineColor([]tcell.Color{
		tcell.ColorRed,
		tcell.ColorGreen,
	})
	widget.plot.SetTitle(title)
	widget.plot.SetDrawAxes(false)
	widget.plot.SetMarker(tvxwidgets.PlotMarkerBraille)

	widget.Primitive = root

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *BaseGraph) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		plotData := make([][]float64, len(widget.labels))

		for i := range widget.labels {
			plotData[i] = []float64{0}
		}

		widget.plot.SetData(plotData)

		return
	}

	_, _, width, _ := widget.plot.GetPlotRect() //nolint:dogsled

	plotData := make([][]float64, len(widget.labels))

	for i, name := range widget.labels {
		series := nodeData.Series[name]

		maxPoints := min(width, len(series))

		plotData[i] = slices.Clone(series[len(series)-maxPoints:])
	}

	widget.plot.SetData(plotData)
}

// NewCPUGraph creates CPU usage graph.
func NewCPUGraph() *BaseGraph {
	return NewBaseGraph("[::b]CPU USER/SYSTEM", []string{"user", "system"})
}

// NewMemGraph creates mem usage graph.
func NewMemGraph() *BaseGraph {
	return NewBaseGraph("[::b]MEM USED", []string{"mem"})
}

// NewLoadAvgGraph creates loadavg graph.
func NewLoadAvgGraph() *BaseGraph {
	return NewBaseGraph("[::b]LOAD AVG 60sec", []string{"loadavg"})
}
