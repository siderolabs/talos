// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// WipeSuite ...
type WipeSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *WipeSuite) SuiteName() string {
	return "api.WipeSuite"
}

// SetupTest ...
func (suite *WipeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}
}

// TearDownTest ...
func (suite *WipeSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestWipeBlockDeviceInvalid verifies that invalid wipe requests are rejected.
func (suite *WipeSuite) TestWipeBlockDeviceInvalid() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for disk := range disks.All() {
		if disk.TypedSpec().Readonly || disk.TypedSpec().CDROM {
			suite.T().Logf("invalid wipe request for %s at %s", disk.Metadata().ID(), node)

			err = suite.Client.BlockDeviceWipe(nodeCtx, &storage.BlockDeviceWipeRequest{
				Devices: []*storage.BlockDeviceWipeDescriptor{
					{
						Device: disk.Metadata().ID(),
					},
				},
			})
			suite.Require().Error(err)
			suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))
		}
	}

	err = suite.Client.BlockDeviceWipe(nodeCtx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: "nosuchdevice",
			},
		},
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.NotFound, client.StatusCode(err))

	// try to wipe a system disk
	systemDisk, err := safe.StateGetByID[*block.SystemDisk](nodeCtx, suite.Client.COSI, block.SystemDiskID)
	suite.Require().NoError(err)

	suite.T().Logf("invalid wipe request for %s at %s", systemDisk.TypedSpec().DiskID, node)
	err = suite.Client.BlockDeviceWipe(nodeCtx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: systemDisk.TypedSpec().DiskID,
			},
		},
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))
}

// TestWipeFilesystem verifies that the filesystem can be wiped.
func (suite *WipeSuite) TestWipeFilesystem() {
	if suite.SelinuxEnforcing {
		suite.T().Skip("skipping tests with nsenter in SELinux enforcing mode")
	}

	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeName := k8sNode.Name

	suite.T().Logf("creating filesystem on %s/%s", node, nodeName)

	userDisks := suite.UserDisks(suite.ctx, node)

	if len(userDisks) < 1 {
		suite.T().Skipf("skipping test, not enough user disks available on node %s/%s: %q", node, nodeName, userDisks)
	}

	userDisk := userDisks[0]

	podDef, err := suite.NewPrivilegedPod("fs-format")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	_, _, err = podDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mkfs.xfs %s", userDisk),
	)
	suite.Require().NoError(err)

	// now Talos should report the disk as xfs formatted
	deviceName := filepath.Base(userDisk)

	nodeCtx := client.WithNode(suite.ctx, node)

	// wait for Talos to discover xfs
	_, err = suite.Client.COSI.WatchFor(nodeCtx,
		block.NewDiscoveredVolume(block.NamespaceName, deviceName).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return r.(*block.DiscoveredVolume).TypedSpec().Name == "xfs", nil
		}),
	)
	suite.Require().NoError(err)

	suite.T().Logf("xfs filesystem created on %s/%s", node, nodeName)

	// wipe the filesystem
	err = suite.Client.BlockDeviceWipe(nodeCtx, &storage.BlockDeviceWipeRequest{
		Devices: []*storage.BlockDeviceWipeDescriptor{
			{
				Device: deviceName,
			},
		},
	})
	suite.Require().NoError(err)

	// wait for Talos to discover that the disk is wiped
	_, err = suite.Client.COSI.WatchFor(nodeCtx,
		block.NewDiscoveredVolume(block.NamespaceName, deviceName).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return r.(*block.DiscoveredVolume).TypedSpec().Name == "", nil
		}),
	)
	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(WipeSuite))
}
