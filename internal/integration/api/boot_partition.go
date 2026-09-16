// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// BootPartitionSuite verifies the boot partition detection.
type BootPartitionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *BootPartitionSuite) SuiteName() string {
	return "api.BootPartitionSuite"
}

// SetupTest ...
func (suite *BootPartitionSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)

	if suite.Cluster != nil && suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping boot partition test since provisioner is docker")
	}
}

// TearDownTest ...
func (suite *BootPartitionSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestBootPartitionDiscovered verifies that the boot partition (if known) is one of the discovered volumes.
//
// The boot partition is known for sd-boot (via the EFI variable) and for GRUB installed by Talos 1.15+
// (via the kernel argument), but not e.g. when booting from an ISO or via PXE.
func (suite *BootPartitionSuite) TestBootPartitionDiscovered() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	bootPartition, err := safe.StateGetByID[*runtimeres.BootPartitionStatus](ctx, suite.Client.COSI, runtimeres.BootPartitionStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			suite.T().Skipf("boot partition is not known on node %s", node)
		}

		suite.Require().NoError(err)
	}

	partitionUUID := bootPartition.TypedSpec().PartitionUUID
	suite.Assert().NotEmpty(partitionUUID)

	discoveredVolumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	var found bool

	for dv := range discoveredVolumes.All() {
		if strings.EqualFold(dv.TypedSpec().PartitionUUID, partitionUUID) {
			suite.T().Logf("boot partition %s is discovered as %s (%s)", partitionUUID, dv.Metadata().ID(), dv.TypedSpec().PartitionLabel)

			found = true

			break
		}
	}

	suite.Assert().True(found, "boot partition %s is not among the discovered volumes on node %s", partitionUUID, node)
}

func init() {
	allSuites = append(allSuites, &BootPartitionSuite{})
}
