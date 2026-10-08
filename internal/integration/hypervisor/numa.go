// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// NUMASuite verifies a cluster created with --numa-nodes.
type NUMASuite struct {
	base.APISuite
}

// SuiteName implements base.NamedSuite.
func (suite *NUMASuite) SuiteName() string {
	return "hypervisor.NUMASuite"
}

// SetupTest checks the NUMA fixture prerequisites.
func (suite *NUMASuite) SetupTest() {
	if suite.NUMANodes == 0 {
		suite.T().Skip("NUMA fixture was not requested")
	}

	suite.Require().Positive(suite.NUMANodes)
	suite.Require().True(suite.Capabilities().RunsTalosKernel, "NUMA fixture requires the Talos kernel")
}

// TestTopology verifies the published inventory against the guest kernel.
func (suite *NUMASuite) TestTopology() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 30*time.Second)
	defer cancel()

	machines := suite.DiscoverNodeInternalIPs(ctx)
	suite.Require().NotEmpty(machines)

	for _, machine := range machines {
		ctx := client.WithNode(ctx, machine)
		present := suite.readNUMAIDs(ctx, "/sys/devices/system/cpu/present")
		online := suite.readNUMAIDs(ctx, "/sys/devices/system/cpu/online")
		nodes := suite.readNUMAIDs(ctx, "/sys/devices/system/node/online")
		suite.Require().Len(nodes, suite.NUMANodes, "machine %s did not boot the requested NUMA topology", machine)
		suite.Require().Equal(present, online, "fixture CPUs must all be online")

		expected := hardware.NUMATopologySpec{PresentCPUs: present, OnlineCPUs: online}
		offset := 0

		for i, id := range nodes {
			suite.Require().EqualValues(i, id)

			cpus := suite.readNUMAIDs(ctx, fmt.Sprintf("/sys/devices/system/node/node%d/cpulist", id))

			count := len(present) / suite.NUMANodes
			if i < len(present)%suite.NUMANodes {
				count++
			}

			suite.Require().Positive(count)
			suite.Require().Equal(present[offset:offset+count], cpus, "machine %s node %d CPU assignment", machine, id)
			offset += count

			var (
				memoryNode uint32
				memoryKiB  uint64
			)

			// Kernel reservations make usable node memory smaller than the QEMU backend.
			meminfo := suite.readNUMAFile(ctx, fmt.Sprintf("/sys/devices/system/node/node%d/meminfo", id))
			fields, err := fmt.Sscanf(meminfo, "Node %d MemTotal: %d kB", &memoryNode, &memoryKiB)
			suite.Require().NoError(err)
			suite.Require().Equal(2, fields)
			suite.Require().Equal(id, memoryNode)
			suite.Require().Positive(memoryKiB)

			expected.Nodes = append(expected.Nodes, hardware.NUMANodeSpec{ID: id, CPUs: cpus, MemoryTotalBytes: memoryKiB * 1024})
		}

		rtestutils.AssertResource[*hardware.NUMATopology](ctx, suite.T(), suite.Client.COSI, hardware.NUMATopologyID,
			func(topology *hardware.NUMATopology, asrt *assert.Assertions) {
				asrt.Equal(expected, *topology.TypedSpec(), "machine %s NUMA inventory", machine)
			})
	}
}

func (suite *NUMASuite) readNUMAFile(ctx context.Context, path string) string {
	reader, err := suite.Client.Read(ctx, path)
	suite.Require().NoError(err)

	data, err := io.ReadAll(reader)
	suite.Require().NoError(reader.Close())
	suite.Require().NoError(err)

	return strings.TrimSpace(string(data))
}

func (suite *NUMASuite) readNUMAIDs(ctx context.Context, path string) []uint32 {
	ids, err := cpuset.Parse(suite.readNUMAFile(ctx, path))
	suite.Require().NoError(err)

	return xslices.Map(ids.List(), func(id int) uint32 { return uint32(id) })
}

func init() {
	allSuites = append(allSuites, new(NUMASuite))
}
