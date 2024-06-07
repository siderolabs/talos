// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumesSuite ...
type VolumesSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *VolumesSuite) SuiteName() string {
	return "api.VolumesSuite"
}

// SetupTest ...
func (suite *VolumesSuite) SetupTest() {
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *VolumesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestDiscoveredVolumes verifies that standard Talos partitions are discovered.
func (suite *VolumesSuite) TestDiscoveredVolumes() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			suite.testDiscoveredVolumes(node)
		})
	}
}

func (suite *VolumesSuite) testDiscoveredVolumes(node string) {
	ctx := client.WithNode(suite.ctx, node)

	volumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	expectedVolumes := map[string]struct {
		Name string
	}{
		"META": {
			Name: "talosmeta",
		},
		"STATE": {
			Name: "xfs",
		},
		"EPHEMERAL": {
			Name: "xfs",
		},
	}

	for iterator := volumes.Iterator(); iterator.Next(); {
		dv := iterator.Value()

		suite.T().Logf("volume: %s %s %s %s", dv.Metadata().ID(), dv.TypedSpec().Name, dv.TypedSpec().PartitionLabel, dv.TypedSpec().Label)

		partitionLabel := dv.TypedSpec().PartitionLabel
		filesystemLabel := dv.TypedSpec().Label

		// this is encrypted partition, skip it, we should see another device with the actual filesystem
		if dv.TypedSpec().Name == "luks" {
			continue
		}

		// match either by partition or filesystem label
		id := partitionLabel

		expected, ok := expectedVolumes[id]
		if !ok {
			id = filesystemLabel

			expected, ok = expectedVolumes[id]

			if !ok {
				continue
			}
		}

		suite.Assert().Equal(expected.Name, dv.TypedSpec().Name, "node: ", node)

		delete(expectedVolumes, id)
	}

	suite.Assert().Empty(expectedVolumes, "node: ", node)

	if suite.T().Failed() {
		suite.DumpLogs(suite.ctx, node, "controller-runtime", "block.")
	}
}

// TestSystemDisk verifies that Talos system disk is discovered.
func (suite *VolumesSuite) TestSystemDisk() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, suite.Client.COSI, block.SystemDiskID)
			suite.Require().NoError(err)

			suite.Assert().NotEmpty(systemDisk.TypedSpec().DiskID)

			suite.T().Logf("system disk: %s", systemDisk.TypedSpec().DiskID)
		})
	}
}

// TestDisks verifies that Talos discovers disks.
func (suite *VolumesSuite) TestDisks() {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			ctx := client.WithNode(suite.ctx, node)

			disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
			suite.Require().NoError(err)

			// there should be at least two disks - loop0 for Talos squashfs and a system disk
			suite.Assert().Greater(disks.Len(), 1)

			var diskNames []string

			for iter := disks.Iterator(); iter.Next(); {
				disk := iter.Value()

				if disk.TypedSpec().Readonly {
					continue
				}

				suite.Assert().NotEmpty(disk.TypedSpec().Size, "disk: %s", disk.Metadata().ID())
				suite.Assert().NotEmpty(disk.TypedSpec().IOSize, "disk: %s", disk.Metadata().ID())
				suite.Assert().NotEmpty(disk.TypedSpec().SectorSize, "disk: %s", disk.Metadata().ID())

				if suite.Cluster != nil {
					// running on our own provider, transport should be always detected
					suite.Assert().NotEmpty(disk.TypedSpec().Transport, "disk: %s", disk.Metadata().ID())
				}

				diskNames = append(diskNames, disk.Metadata().ID())
			}

			suite.T().Logf("disks: %v", diskNames)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(VolumesSuite))
}
