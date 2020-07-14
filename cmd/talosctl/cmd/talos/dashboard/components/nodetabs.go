// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"sort"

	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// NodeTabs represents the bottom bar with node list.
type NodeTabs struct {
	widgets.TabPane
}

// NewNodeTabs initializes NodeTabs.
func NewNodeTabs() *NodeTabs {
	tabs := &NodeTabs{
		TabPane: *widgets.NewTabPane(),
	}

	return tabs
}

// Update implements the DataWidget interface.
func (widget *NodeTabs) Update(node string, data *data.Data) {
	nodes := make([]string, 0, len(data.Nodes))

	for node := range data.Nodes {
		nodes = append(nodes, node)
	}

	sort.Strings(nodes)

	widget.TabNames = nodes
	if widget.ActiveTabIndex > len(nodes) {
		widget.ActiveTabIndex = 0
	}
}
