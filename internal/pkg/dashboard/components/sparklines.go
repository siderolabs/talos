// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// BaseSparklineGroup represents the widget with some sparklines.
type BaseSparklineGroup struct {
	tview.Primitive

	sparklines []*tvxwidgets.Sparkline
	dataLabels []string
}

// NewBaseSparklineGroup initializes BaseSparklineGroup.
func NewBaseSparklineGroup(title string, labels, dataLabels []string) *BaseSparklineGroup {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	root := tview.NewFrame(flex).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(title, true, tview.AlignLeft, tcell.ColorDefault)

	colors := []tcell.Color{tcell.ColorRed, tcell.ColorGreen}

	sparklines := make([]*tvxwidgets.Sparkline, len(labels))

	for i := range labels {
		sparklines[i] = tvxwidgets.NewSparkline()
		sparklines[i].SetBorder(false)
		sparklines[i].SetDataTitle(labels[i])
		sparklines[i].SetTitleColor(tcell.ColorDefault)
		sparklines[i].SetLineColor(colors[i%len(colors)])

		flex.AddItem(sparklines[i], 0, 1, false)
	}

	return &BaseSparklineGroup{
		Primitive:  root,
		sparklines: sparklines,
		dataLabels: dataLabels,
	}
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *BaseSparklineGroup) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		for i := range widget.sparklines {
			widget.sparklines[i].SetData([]float64{0})
		}

		return
	}

	_, _, width, _ := widget.GetRect() //nolint:dogsled

	for i, name := range widget.dataLabels {
		series := nodeData.Series[name]

		if len(series) < width {
			width = len(series)
		}

		widget.sparklines[i].SetData(series[len(series)-width:])
	}
}

// NewNetSparkline creates network sparkline.
func NewNetSparkline() *BaseSparklineGroup {
	return NewBaseSparklineGroup(" [::b]NET", []string{"RX", "TX"}, []string{"netrxbytes", "nettxbytes"})
}

// NewDiskSparkline creates disk sparkline.
func NewDiskSparkline() *BaseSparklineGroup {
	return NewBaseSparklineGroup(" [::b]DISK", []string{"READ", "WRITE"}, []string{"diskrdsectors", "diskwrsectors"})
}
