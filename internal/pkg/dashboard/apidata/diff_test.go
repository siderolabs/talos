// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apidata_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// TestUpdateDiffContainerCPU verifies that a container only has a CPU delta once there are two
// samples to derive it from: an entry holding a zero would be rendered as an idle 0.0% rather than
// as the unknown it is.
func TestUpdateDiffContainerCPU(t *testing.T) {
	old := &apidata.Node{
		ContainerStats: &machine.Stats{Stats: []*machine.Stat{
			{Id: "steady", CpuUsage: uint64(10 * time.Second)},
			{Id: "restarted", CpuUsage: uint64(30 * time.Second)},
		}},
	}

	node := &apidata.Node{
		ContainerStats: &machine.Stats{Stats: []*machine.Stat{
			{Id: "steady", CpuUsage: uint64(11 * time.Second)},
			{Id: "restarted", CpuUsage: uint64(time.Second)},
			{Id: "new", CpuUsage: uint64(5 * time.Second)},
		}},
	}

	node.UpdateDiff(old)

	assert.Equal(t, uint64(time.Second), node.ContainerCPUDiff["steady"])

	// A counter that went backwards is a sign containerd restarted the container: zero is the
	// reading, and the alternative would be a spike.
	restarted, ok := node.ContainerCPUDiff["restarted"]
	assert.True(t, ok)
	assert.Zero(t, restarted)

	// A container seen for the first time has nothing to subtract from.
	_, ok = node.ContainerCPUDiff["new"]
	assert.False(t, ok)
}

// TestUpdateDiffNoPreviousContainerStats verifies that the first poll of all leaves the deltas
// unset rather than reporting every container as idle.
func TestUpdateDiffNoPreviousContainerStats(t *testing.T) {
	node := &apidata.Node{
		ContainerStats: &machine.Stats{Stats: []*machine.Stat{{Id: "apid", CpuUsage: uint64(time.Second)}}},
	}

	node.UpdateDiff(&apidata.Node{})

	assert.Nil(t, node.ContainerCPUDiff)
}
