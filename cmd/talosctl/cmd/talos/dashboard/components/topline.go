// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"time"

	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// TopLine represents the top bar with host info.
type TopLine struct {
	widgets.Paragraph
}

// NewTopLine initializes TopLine.
func NewTopLine() *TopLine {
	topline := &TopLine{
		Paragraph: *widgets.NewParagraph(),
	}

	topline.Border = false
	topline.Text = noData

	return topline
}

// Update implements the DataWidget interface.
func (widget *TopLine) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Text = "n/a"
	} else {
		widget.Text = fmt.Sprintf("[%s](fg:yellow,mod:bold) (%s-%s): uptime %s, %d cores, %d procs",
			nodeData.Hostname.GetHostname(),
			nodeData.Version.GetVersion().GetTag(),
			nodeData.Version.GetVersion().GetSha(),
			time.Since(time.Unix(int64(nodeData.SystemStat.GetBootTime()), 0)).Round(time.Second),
			len(nodeData.CPUsInfo.GetCpuInfo()),
			len(nodeData.Processes.GetProcesses()),
		)
	}
}
