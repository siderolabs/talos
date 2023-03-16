// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components_test

import (
	"testing"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestUpdate(t *testing.T) {
	testProcessTable := components.NewProcessTable()

	testData := &apidata.Data{
		Nodes: map[string]*apidata.Node{
			"node1": {
				Processes: &machine.Process{
					Processes: []*machine.ProcessInfo{},
				},
				ProcsDiff: map[int32]*machine.ProcessInfo{
					1: {},
				},
				Series: map[string][]float64{},
			},
			"node2": {
				ProcsDiff: map[int32]*machine.ProcessInfo{
					1: {},
				},
			},
		},
	}
	testProcessTable.OnAPIDataChange("node1", testData)
	// Node2 does not have processes, without the check it panics
	testProcessTable.OnAPIDataChange("node2", testData)
}
