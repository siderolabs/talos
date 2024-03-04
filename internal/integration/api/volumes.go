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
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	volumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	expectedVolumes := map[string]struct {
		Name string
	}{
		"META": {},
		"STATE": {
			Name: "xfs",
		},
		"EPHEMERAL": {
			Name: "xfs",
		},
	}

	for iterator := volumes.Iterator(); iterator.Next(); {
		dv := iterator.Value()

		partitionLabel := dv.TypedSpec().PartitionLabel
		filesystemLabel := dv.TypedSpec().Label

		// this is encrypted partition, skip it, we should see another device with actual filesystem
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

		suite.Assert().Equal(expected.Name, dv.TypedSpec().Name)

		delete(expectedVolumes, id)
	}

	suite.Assert().Empty(expectedVolumes)
}

func init() {
	allSuites = append(allSuites, new(VolumesSuite))
}
