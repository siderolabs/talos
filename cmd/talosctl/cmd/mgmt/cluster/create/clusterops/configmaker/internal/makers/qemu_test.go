// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"testing"

	sideronet "github.com/siderolabs/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/provision"
)

func TestQemuMaker_MachineConfig(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil)
}

func TestQemuMaker_RegistryAuth(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	qOps.DownloadHTTPAuth = map[string]clusterops.HTTPAuth{
		"example.com": {
			Username: "username",
			Password: "password",
		},
	}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	registryAuthConfig := cri.NewRegistryAuthConfigV1Alpha1("example.com")
	registryAuthConfig.RegistryUsername = "username"
	registryAuthConfig.RegistryPassword = "password"

	ctr, err := container.New(registryAuthConfig)
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil, configpatcher.NewStrategicMergePatch(ctr))
}

func TestQemuMaker_Disks(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	disks := flags.Disks{}
	err := disks.Set("virtio:10GiB,nvme:20GiB,virtio:30GiB")
	require.NoError(t, err)

	qOps.Disks = disks
	cOps.Controlplanes = 1
	cOps.Workers = 1

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	req, err := m.GetClusterConfigs()
	require.NoError(t, err)

	controlplaneDisks := req.ClusterRequest.Nodes[0].Disks
	workerDisks := req.ClusterRequest.Nodes[1].Disks

	assert.Equal(t, 1, len(controlplaneDisks))
	assert.Equal(t, 3, len(workerDisks))

	assert.Equal(t, []*provision.Disk{
		{
			Size:            disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
	}, controlplaneDisks)

	assert.Equal(t, []*provision.Disk{
		{
			Size:            disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
		{
			Size:            disks.Requests()[1].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "nvme",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
		{
			Size:            disks.Requests()[2].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
	}, workerDisks)
}

func TestQemuMaker_DiskEncryption_StatePartition(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = []string{"uuid"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	blockCfg := block.NewVolumeConfigV1Alpha1()
	blockCfg.MetaName = constants.StatePartitionLabel
	blockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:   0,
				KeyNodeID: &block.EncryptionKeyNodeID{},
			},
		},
	}

	ctr, err := container.New(blockCfg)
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil, configpatcher.NewStrategicMergePatch(ctr))
}

func TestQemuMaker_DiskEncryption_EphemeralPartition(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptEphemeralPartition = true
	qOps.DiskEncryptionKeyTypes = []string{"uuid"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	blockCfg := block.NewVolumeConfigV1Alpha1()
	blockCfg.MetaName = constants.EphemeralPartitionLabel
	blockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:        0,
				KeyNodeID:      &block.EncryptionKeyNodeID{},
				KeyLockToSTATE: new(true),
			},
		},
	}

	ctr, err := container.New(blockCfg)
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil, configpatcher.NewStrategicMergePatch(ctr))
}

func TestQemuMaker_DiskEncryption_BothPartitions(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.EncryptEphemeralPartition = true
	qOps.DiskEncryptionKeyTypes = []string{"uuid"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	// STATE partition patch (no KeyLockToSTATE)
	stateBlockCfg := block.NewVolumeConfigV1Alpha1()
	stateBlockCfg.MetaName = constants.StatePartitionLabel
	stateBlockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:   0,
				KeyNodeID: &block.EncryptionKeyNodeID{},
			},
		},
	}

	// EPHEMERAL partition patch (with KeyLockToSTATE)
	ephemeralBlockCfg := block.NewVolumeConfigV1Alpha1()
	ephemeralBlockCfg.MetaName = constants.EphemeralPartitionLabel
	ephemeralBlockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:        0,
				KeyNodeID:      &block.EncryptionKeyNodeID{},
				KeyLockToSTATE: new(true),
			},
		},
	}

	stateCtr, err := container.New(stateBlockCfg)
	require.NoError(t, err)

	ephemeralCtr, err := container.New(ephemeralBlockCfg)
	require.NoError(t, err)

	assertConfigDefaultness(
		t, cOps, *m.Maker, nil,
		configpatcher.NewStrategicMergePatch(stateCtr),
		configpatcher.NewStrategicMergePatch(ephemeralCtr),
	)
}

func TestQemuMaker_DiskEncryption_KMSKeyType(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = []string{"kms"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	// Compute the expected bridge IP as done in getEncryptionKeys
	bridgeIP, err := sideronet.NthIPInNetwork(m.Cidrs[0], 1)
	require.NoError(t, err)

	blockCfg := block.NewVolumeConfigV1Alpha1()
	blockCfg.MetaName = constants.StatePartitionLabel
	blockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot: 0,
				KeyKMS: &block.EncryptionKeyKMS{
					KMSEndpoint: "grpc://" + nethelpers.JoinHostPort(bridgeIP.String(), 4050),
				},
			},
		},
	}

	ctr, err := container.New(blockCfg)
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil, configpatcher.NewStrategicMergePatch(ctr))
}

func TestQemuMaker_DiskEncryption_MultipleKeyTypes(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = []string{"uuid", "tpm"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	blockCfg := block.NewVolumeConfigV1Alpha1()
	blockCfg.MetaName = constants.StatePartitionLabel
	blockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:   0,
				KeyNodeID: &block.EncryptionKeyNodeID{},
			},
			{
				KeySlot: 1,
				KeyTPM: &block.EncryptionKeyTPM{
					TPMCheckSecurebootStatusOnEnroll: new(true),
				},
			},
		},
	}

	ctr, err := container.New(blockCfg)
	require.NoError(t, err)

	assertConfigDefaultness(t, cOps, *m.Maker, nil, configpatcher.NewStrategicMergePatch(ctr))
}

func TestQemuMaker_DiskEncryption_LegacyVersion(t *testing.T) {
	cOps := clusterops.GetCommon()
	cOps.TalosVersion = "v1.6.0"

	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = []string{"tpm"}

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})
	require.NoError(t, err)

	// For legacy version, the encryption config is applied to machine.systemDiskEncryption
	// Verify the generated cluster config doesn't fail and includes the legacy encryption path
	clusterCfgs, err := m.GetClusterConfigs()
	require.NoError(t, err)

	cfgBytes, err := clusterCfgs.ClusterRequest.Nodes[0].Config.EncodeBytes()
	require.NoError(t, err)

	cfgStr := string(cfgBytes)
	// Legacy versions use systemDiskEncryption, not VolumeConfig
	assert.Contains(t, cfgStr, "systemDiskEncryption")
	assert.Contains(t, cfgStr, "provider: luks2")
	assert.Contains(t, cfgStr, "tpm: {}")
}

func TestQemuMaker_DiskEncryption_ErrorUnknownKeyType(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = []string{"bogus"}

	_, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key type")
}

func TestQemuMaker_DiskEncryption_ErrorNoKeyTypes(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.EncryptStatePartition = true
	qOps.DiskEncryptionKeyTypes = nil

	_, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no disk encryption key types enabled")
}
