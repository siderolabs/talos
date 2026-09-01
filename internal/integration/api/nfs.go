// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// TestExternalVolumesNFS verifies the QEMU development NFS server through real NFSv3 and NFSv4 mounts.
func (suite *VolumesSuite) TestExternalVolumesNFS() {
	if !suite.NFS {
		suite.T().Skip("skipping test without the embedded NFS server")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeCtx := client.WithNode(suite.ctx, node)
	gatewayAddrs := suite.Cluster.Info().Network.GatewayAddrs
	suite.Require().NotEmpty(gatewayAddrs, "QEMU cluster has no gateway address")

	gateway := gatewayAddrs[0]
	newConfig := func(name string, version block.NFSVersion) *blockcfg.ExternalVolumeConfigV1Alpha1 {
		cfg := blockcfg.NewExternalVolumeConfigV1Alpha1()
		cfg.MetaName = name
		cfg.FilesystemType = block.FilesystemTypeNFS
		cfg.MountSpec.MountNFS = &blockcfg.NFSMountSpec{
			NFSServer:    gateway.String(),
			NFSPath:      "/export",
			NFSVersion:   version,
			NFSPort:      2049,
			NFSTransport: new(block.NFSTransportTCP),
		}

		if version == block.NFSVersion3 {
			cfg.MountSpec.MountNFS.NFSMountPort = 2049
			cfg.MountSpec.MountNFS.NFSMountTransport = new(block.NFSTransportTCP)
		}

		return cfg
	}

	defer func() {
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.ExternalVolumeConfigKind, "nfs-v3")
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.ExternalVolumeConfigKind, "nfs-v4")
	}()

	suite.PatchMachineConfig(nodeCtx,
		newConfig("nfs-v3", block.NFSVersion3),
		newConfig("nfs-v4", block.NFSVersion4Point1),
	)

	volumeIDs := []resource.ID{
		constants.ExternalVolumePrefix + "nfs-v3",
		constants.ExternalVolumePrefix + "nfs-v4",
	}

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, volumeIDs,
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
			asrt.Equal(block.VolumeTypeExternal, status.TypedSpec().Type)

			version := "3"
			if status.Metadata().ID() == constants.ExternalVolumePrefix+"nfs-v4" {
				version = "4.1"
			}

			asrt.Contains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("vers", version))

			if status.Metadata().ID() == constants.ExternalVolumePrefix+"nfs-v3" {
				asrt.Contains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("mountport", "2049"))
				asrt.Contains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("mountproto", "tcp"))
			}
		},
	)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, volumeIDs,
		func(status *block.MountStatus, asrt *assert.Assertions) {
			asrt.Equal(block.FilesystemTypeNFS, status.TypedSpec().Filesystem)
			asrt.False(status.TypedSpec().ReadOnly)
			asrt.Equal(filepath.Join(constants.UserVolumeMountPoint, status.Metadata().ID()[len(constants.ExternalVolumePrefix):]), status.TypedSpec().Target)
			asrt.Contains(status.TypedSpec().Source, ":/export")
		},
	)

	podDef, err := suite.NewPod("external-volume-nfs-test")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(k8sNode.Name).
		WithNamespace("kube-system").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, "nfs-v3"), "/mnt/nfs-v3").
		WithHostVolumeMount(filepath.Join(constants.UserVolumeMountPoint, "nfs-v4"), "/mnt/nfs-v4")

	suite.Require().NoError(podDef.Create(suite.ctx, time.Minute))
	defer podDef.Delete(suite.ctx) //nolint:errcheck

	first := uuid.NewString()
	second := uuid.NewString()
	command := fmt.Sprintf(
		"set -eu; printf '%%s' %s > /mnt/nfs-v3/from-v3; test \"$(cat /mnt/nfs-v4/from-v3)\" = %s; "+
			"printf '%%s' %s > /mnt/nfs-v4/from-v4; test \"$(cat /mnt/nfs-v3/from-v4)\" = %s; "+
			"rm /mnt/nfs-v3/from-v3 /mnt/nfs-v4/from-v4",
		strconv.Quote(first), strconv.Quote(first), strconv.Quote(second), strconv.Quote(second),
	)

	stdout, stderr, err := podDef.Exec(suite.ctx, command)
	suite.Require().NoError(err, "NFS cross-version read/write failed: stdout=%q stderr=%q", stdout, stderr)

	// Tune the existing external volume in place. The mount controller should coordinate
	// teardown of the old NFSv3 mount and recreate it as NFSv4.1 without deleting the document.
	//
	// Patches are a strategic merge, so the NFSv3-only options the document already carries survive
	// a patch that merely omits them and are then rejected on v4: drop them with `$patch: delete`.
	suite.PatchMachineConfig(nodeCtx,
		newConfig("nfs-v3", block.NFSVersion4Point1),
		map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       blockcfg.ExternalVolumeConfigKind,
			"name":       "nfs-v3",
			"mount": map[string]any{
				"nfs": map[string]any{
					"mountPort":      map[string]string{"$patch": "delete"},
					"mountTransport": map[string]string{"$patch": "delete"},
				},
			},
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, constants.ExternalVolumePrefix+"nfs-v3",
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
			asrt.Contains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("vers", "4.1"))
			asrt.NotContains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("vers", "3"))
			asrt.NotContains(status.TypedSpec().MountSpec.Parameters, block.NewBooleanParameter("nolock"))
			asrt.NotContains(status.TypedSpec().MountSpec.Parameters, block.NewStringParameter("mountport", "2049"))
		},
	)

	mountTarget := filepath.Join(constants.UserVolumeMountPoint, "nfs-v3")
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		mountInfo := suite.ReadFile(nodeCtx, "/proc/1/mountinfo")
		asrt := assert.New(collect)

		asrt.True(mountInfoHasFilesystem(mountInfo, mountTarget, "nfs4"),
			"external volume was not recreated as NFSv4.1")
	}, time.Minute, time.Second)
}

func mountInfoHasFilesystem(mountInfo, target, filesystem string) bool {
	for line := range strings.Lines(mountInfo) {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[4] != target {
			continue
		}

		separator := slices.Index(fields, "-")
		if separator >= 0 && len(fields) > separator+1 && fields[separator+1] == filesystem {
			return true
		}
	}

	return false
}
