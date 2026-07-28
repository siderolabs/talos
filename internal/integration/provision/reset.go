// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ResetSystemVolumesSuite verifies that a node can be reset, assuming either
// the default layout of promotable system volumes (directories under EPHEMERAL)
// or a layout where they are promoted onto dedicated partitions.
// It does so by provisioning a cluster with the given layout, then resetting one node
// of each machine type and verifying that the cluster becomes healthy again.
//
// The api.ResetSuite covers the same ground, but only for whatever layout the cluster it runs
// against happens to have; this suite provisions the layout it tests.
type ResetSystemVolumesSuite struct {
	BaseSuite

	// dedicatedSystemVolumes places the promotable system volumes onto dedicatedSystemVolumes partitions instead of leaving
	// them as directories under EPHEMERAL.
	dedicatedSystemVolumes bool

	track int
}

// SuiteName ...
func (suite *ResetSystemVolumesSuite) SuiteName() string {
	return fmt.Sprintf("provision.ResetSystemVolumesSuite.%s-TR%d", suite.layoutName(), suite.track)
}

func (suite *ResetSystemVolumesSuite) layoutName() string {
	if suite.dedicatedSystemVolumes {
		return "Dedicated"
	}

	return "OnEphemeral"
}

// expectedVolumeType is the backing every promotable system volume is expected to have.
func (suite *ResetSystemVolumesSuite) expectedVolumeType() blockres.VolumeType {
	if suite.dedicatedSystemVolumes {
		return blockres.VolumeTypePartition
	}

	return blockres.VolumeTypeDirectory
}

// dedicatedSystemVolumesPatch promotes every promotable system volume onto a dedicated partition.
//
// The documents are built here rather than loaded from hack/test/patches/, as the provision test
// binary doesn't necessarily run with the repository tree available.
func dedicatedSystemVolumesPatch() (configpatcher.Patch, error) {
	docs := xslices.Map(configconfig.PromotableSystemVolumeNames, func(volumeID string) configconfig.Document {
		cfg := blockcfg.NewVolumeConfigV1Alpha1()
		cfg.MetaName = volumeID
		cfg.ProvisioningSpec = blockcfg.ProvisioningSpec{
			ProvisioningMinSize: blockcfg.MustByteSize("512MB"),
			ProvisioningMaxSize: blockcfg.MustSize("1GB"),
		}

		return cfg
	})

	ctr, err := container.New(docs...)
	if err != nil {
		return nil, err
	}

	return configpatcher.NewStrategicMergePatch(ctr), nil
}

// TestResetSystemVolumes resets one node of each machine type, wiping EPHEMERAL together with every
// promotable system volume.
func (suite *ResetSystemVolumesSuite) TestResetSystemVolumes() {
	options := clusterOptions{
		ClusterName: fmt.Sprintf("reset-system-volumes-%s", suite.layoutName()),

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,

		SourceKernelPath:    helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath: helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		SourceVersion:    DefaultSettings.CurrentVersion,
		SourceK8sVersion: constants.DefaultKubernetesVersion,
	}

	if suite.dedicatedSystemVolumes {
		patch, err := dedicatedSystemVolumesPatch()
		suite.Require().NoError(err)

		// the same patch for both machine types: unlike the CI config patches, this suite wants a
		// layout which is identical across the cluster
		options.ConfigPatchesControlPlane = []configpatcher.Patch{patch}
		options.ConfigPatchesWorker = []configpatcher.Patch{patch}
	}

	suite.setupCluster(options)

	for _, nodeType := range []machine.Type{machine.TypeControlPlane, machine.TypeWorker} {
		suite.Run(nodeType.String(), func() {
			node, ok := suite.nodeOfType(nodeType)
			suite.Require().Truef(ok, "no %s node in the cluster", nodeType)

			suite.assertSystemVolumeTypes(node)
			suite.resetNode(node)
		})
	}
}

// nodeOfType returns the IP of the first node of the given machine type.
func (suite *ResetSystemVolumesSuite) nodeOfType(nodeType machine.Type) (string, bool) {
	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == nodeType {
			return node.IPs[0].String(), true
		}
	}

	return "", false
}

// assertSystemVolumeTypes verifies the node came up with the layout this suite provisioned.
func (suite *ResetSystemVolumesSuite) assertSystemVolumeTypes(node string) {
	client, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	nodeCtx := talosclient.WithNode(suite.ctx, node)

	for _, volumeID := range configconfig.PromotableSystemVolumeNames {
		volumeStatus, err := safe.StateGetByID[*blockres.VolumeStatus](nodeCtx, client.COSI, volumeID)
		suite.Require().NoError(err)
		suite.Require().Equalf(suite.expectedVolumeType(), volumeStatus.TypedSpec().Type,
			"unexpected backing of volume %q on node %s", volumeID, node)
	}
}

// resetNode wipes EPHEMERAL and every promotable system volume on the node, then waits for the
// cluster to become healthy again.
func (suite *ResetSystemVolumesSuite) resetNode(node string) {
	client, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	nodeCtx := talosclient.WithNode(suite.ctx, node)

	partitionsToWipe := make([]*machineapi.ResetPartitionSpec, 0, 1+len(configconfig.PromotableSystemVolumeNames))
	partitionsToWipe = append(partitionsToWipe, &machineapi.ResetPartitionSpec{
		Label: constants.EphemeralPartitionLabel,
		Wipe:  true,
	})

	for _, volumeID := range configconfig.PromotableSystemVolumeNames {
		partitionsToWipe = append(partitionsToWipe, &machineapi.ResetPartitionSpec{
			Label: volumeID,
			Wipe:  true,
		})
	}

	bootIDBefore := suite.readBootID(nodeCtx, client)

	suite.T().Logf("resetting node %s (boot ID %s)", node, bootIDBefore)

	suite.Require().NoError(client.ResetGeneric(nodeCtx, &machineapi.ResetRequest{
		Reboot:                 true,
		Graceful:               true,
		SystemPartitionsToWipe: partitionsToWipe,
	}))

	// wait for the node to actually come back on a new boot before health-checking the cluster,
	// otherwise the check might still see the pre-reset state
	suite.Require().NoError(retry.Constant(10*time.Minute, retry.WithUnits(5*time.Second)).Retry(
		func() error {
			bootID, err := suite.tryReadBootID(nodeCtx, client)
			if err != nil {
				return retry.ExpectedError(err)
			}

			if bootID == bootIDBefore {
				return retry.ExpectedErrorf("node %s hasn't rebooted yet", node)
			}

			return nil
		},
	))

	suite.waitForClusterHealth()

	// Negative case: if KUBELET is directory-backed on EPHEMERAL, the caller must wipe the backing partition (EPHEMERAL) as well.
	// Otherwise, the request must immediately fail with an error.
	if !suite.dedicatedSystemVolumes {
		dirBackedVolume := constants.KubeletDataVolumeID
		expectedParentVolume := constants.EphemeralPartitionLabel

		// Spot-check the volume to make sure we're testing the right thing.
		volumeStatus, err := safe.StateGetByID[*blockres.VolumeStatus](nodeCtx, client.COSI, dirBackedVolume)
		suite.Require().NoError(err)
		suite.Require().Equalf(blockres.VolumeTypeDirectory, volumeStatus.TypedSpec().Type,
			"unexpected backing of volume %q on node %s", dirBackedVolume, node)

		suite.Require().ErrorContains(client.ResetGeneric(nodeCtx, &machineapi.ResetRequest{
			Reboot:   true,
			Graceful: true,
			SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
				{
					Label: constants.KubeletDataVolumeID,
					Wipe:  true,
				},
				// EPHEMERAL is intentionally missing here.
			},
		}),
			fmt.Sprintf("failed to reset: volume %q resides on volume %q, and therefore %q cannot be wiped without wiping %q",
				dirBackedVolume,
				expectedParentVolume,
				dirBackedVolume,
				expectedParentVolume,
			),
		)
	}

	suite.waitForClusterHealth()
}

func (suite *ResetSystemVolumesSuite) readBootID(nodeCtx context.Context, client *talosclient.Client) string {
	bootID, err := suite.tryReadBootID(nodeCtx, client)
	suite.Require().NoError(err)

	return bootID
}

func (suite *ResetSystemVolumesSuite) tryReadBootID(nodeCtx context.Context, client *talosclient.Client) (string, error) {
	// short timeout: a rebooting node may not answer for a long time
	reqCtx, reqCtxCancel := context.WithTimeout(nodeCtx, 10*time.Second)
	defer reqCtxCancel()

	bootID, err := safe.StateGetByID[*runtimeres.BootID](reqCtx, client.COSI, runtimeres.BootIDID)
	if err != nil {
		return "", err
	}

	return bootID.TypedSpec().BootID, nil
}

func init() {
	allSuites = append(
		allSuites,
		&ResetSystemVolumesSuite{dedicatedSystemVolumes: false, track: 3},
		&ResetSystemVolumesSuite{dedicatedSystemVolumes: true, track: 3},
	)
}
