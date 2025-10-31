// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	intmeta "github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/yamlutils"
)

type VolumeConfigSuite struct {
	ctest.DefaultSuite
}

type metaProvider struct {
	meta *intmeta.Meta
}

func (m metaProvider) Meta() machineruntime.Meta {
	return m.meta
}

func TestVolumeConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VolumeConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				tmpDir := suite.T().TempDir()
				path := filepath.Join(tmpDir, "meta")

				f, err := os.Create(path)
				suite.Require().NoError(err)
				suite.Require().NoError(f.Truncate(1024 * 1024))
				suite.Require().NoError(f.Close())

				st := state.WrapCore(namespaced.NewState(inmem.Build))

				m, err := intmeta.New(t.Context(), st, intmeta.WithFixedPath(path))
				suite.Require().NoError(err)

				suite.Require().NoError(suite.Runtime().RegisterController(
					&blockctrls.VolumeConfigController{
						MetaProvider: metaProvider{meta: m},
					},
				))
			},
		},
	})
}

func (suite *VolumeConfigSuite) TestReconcileDefaults() {
	// no machine config, default config which only searches for
	ctest.AssertResource(suite, constants.MetaPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)
	})
	ctest.AssertResource(suite, constants.StatePartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)

		locator, err := r.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`volume.partition_label == "STATE" && volume.name != ""`, string(locator))

		asrt.Equal(constants.StateMountPoint, r.TypedSpec().Mount.TargetPath)
	})
	ctest.AssertNoResource[*block.VolumeConfig](suite, constants.EphemeralPartitionLabel)

	// create a dummy machine config
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	ctest.AssertResource(suite, constants.MetaPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)
		asrt.Empty(r.TypedSpec().Mount)

		locator, err := r.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)

		asrt.Equal(`volume.partition_label == "META" && volume.name in ["", "talosmeta"] && volume.size == 1048576u`, string(locator))
	})
	ctest.AssertResource(suite, constants.StatePartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)

		locator, err := r.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`volume.partition_label == "STATE"`, string(locator))

		asrt.Equal(constants.StateMountPoint, r.TypedSpec().Mount.TargetPath)
	})
	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)

		locator, err := r.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`volume.partition_label == "EPHEMERAL"`, string(locator))

		locator, err = r.TypedSpec().Provisioning.DiskSelector.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`system_disk`, string(locator))

		asrt.True(r.TypedSpec().Provisioning.PartitionSpec.Grow)
		asrt.EqualValues(0, r.TypedSpec().Provisioning.PartitionSpec.MaxSize)
		asrt.EqualValues(quirks.New("").PartitionSizes().EphemeralMinSize(), r.TypedSpec().Provisioning.PartitionSpec.MinSize)

		asrt.Equal(constants.EphemeralMountPoint, r.TypedSpec().Mount.TargetPath)
	})
	ctest.AssertResource(suite, "/var/run", func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Equal(r.TypedSpec().Type, block.VolumeTypeSymlink)
		asrt.Equal(r.TypedSpec().Symlink.SymlinkTargetPath, "/run")
		asrt.Equal(r.TypedSpec().Mount.TargetPath, "/var/run")
	})

	ctest.AssertResources(suite, []resource.ID{
		constants.LogMountPoint,
		"/var/log/audit",
		"/var/log/containers",
		"/var/log/pods",
		constants.EtcdDataVolumeID,
		"/var/lib/containerd",
		"/var/lib/kubelet",
		"/var/lib/cni",
		constants.SeccompProfilesDirectory,
		constants.KubernetesAuditLogDir,
		"/var/run/lock",
	}, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Equal(block.VolumeTypeDirectory, r.TypedSpec().Type)
	})

	ctest.AssertResources(suite,
		xslices.Map(constants.Overlays, func(target constants.SELinuxLabeledPath) resource.ID {
			return target.Path
		}),
		func(r *block.VolumeConfig, asrt *assert.Assertions) {
			asrt.Equal(block.VolumeTypeOverlay, r.TypedSpec().Type)
		})
}

func (suite *VolumeConfigSuite) TestReconcileEncryptedSTATE() {
	stateEncryption := &v1alpha1.EncryptionConfig{
		EncryptionProvider: "luks2",
		EncryptionKeys: []*v1alpha1.EncryptionKey{
			{
				KeySlot: 1,
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "supersecret",
				},
			},
			{
				KeySlot: 2,
				KeyTPM:  &v1alpha1.EncryptionKeyTPM{},
			},
		},
	}

	stateEncryptionMarshalled, err := json.Marshal(stateEncryption)
	suite.Require().NoError(err)

	stateMetaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))
	stateMetaKey.TypedSpec().Value = string(stateEncryptionMarshalled)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), stateMetaKey))

	// no machine config, default config which only searches for
	ctest.AssertResource(suite, constants.MetaPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)
	})
	ctest.AssertResource(suite, constants.StatePartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)

		asrt.NotEmpty(r.TypedSpec().Encryption)

		asrt.Equal(block.EncryptionProviderLUKS2, r.TypedSpec().Encryption.Provider)
		asrt.Len(r.TypedSpec().Encryption.Keys, 2)

		if len(r.TypedSpec().Encryption.Keys) != 2 {
			return
		}

		asrt.Equal(1, r.TypedSpec().Encryption.Keys[0].Slot)

		asrt.Equal(block.EncryptionKeyStatic, r.TypedSpec().Encryption.Keys[0].Type)
		asrt.Equal(yamlutils.StringBytes([]byte("supersecret")), r.TypedSpec().Encryption.Keys[0].StaticPassphrase)

		asrt.Equal(2, r.TypedSpec().Encryption.Keys[1].Slot)
		asrt.Equal(block.EncryptionKeyTPM, r.TypedSpec().Encryption.Keys[1].Type)
	})
	ctest.AssertNoResource[*block.VolumeConfig](suite, constants.EphemeralPartitionLabel)

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineSystemDiskEncryption: &v1alpha1.SystemDiskEncryptionConfig{
						StatePartition: stateEncryption,
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	ctest.AssertResource(suite, constants.MetaPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Provisioning)
	})
	ctest.AssertResource(suite, constants.StatePartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)
		asrt.NotEmpty(r.TypedSpec().Encryption)
	})
	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)
		asrt.Empty(r.TypedSpec().Encryption)
	})
}

func (suite *VolumeConfigSuite) TestReconcileExtraEPHEMERALConfig() {
	ctest.AssertNoResource[*block.VolumeConfig](suite, constants.EphemeralPartitionLabel)

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	ctr, err := container.New(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
			},
		},
		&blockcfg.VolumeConfigV1Alpha1{
			MetaName: constants.EphemeralPartitionLabel,
			ProvisioningSpec: blockcfg.ProvisioningSpec{
				DiskSelectorSpec: blockcfg.DiskSelector{
					Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
				},
				ProvisioningGrow:    pointer.To(false),
				ProvisioningMaxSize: blockcfg.MustSize("2.5TiB"),
			},
			EncryptionSpec: blockcfg.EncryptionSpec{
				EncryptionProvider: block.EncryptionProviderLUKS2,
				EncryptionKeys: []blockcfg.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &blockcfg.EncryptionKeyTPM{},
					},
				},
			},
		},
	)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)
		asrt.NotEmpty(r.TypedSpec().Encryption)

		locator, err := r.TypedSpec().Provisioning.DiskSelector.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`disk.transport == "nvme"`, string(locator))

		asrt.False(r.TypedSpec().Provisioning.PartitionSpec.Grow)
		asrt.EqualValues(2.5*1024*1024*1024*1024, r.TypedSpec().Provisioning.PartitionSpec.MaxSize)
		asrt.EqualValues(quirks.New("").PartitionSizes().EphemeralMinSize(), r.TypedSpec().Provisioning.PartitionSpec.MinSize)

		asrt.Equal(block.EncryptionProviderLUKS2, r.TypedSpec().Encryption.Provider)
	})
}

func (suite *VolumeConfigSuite) TestReconcileUserRawVolumes() {
	rv1 := blockcfg.NewRawVolumeConfigV1Alpha1()
	rv1.MetaName = "data1"
	suite.Require().NoError(rv1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
	rv1.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("10GiB")
	rv1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("100GiB")

	rv2 := blockcfg.NewRawVolumeConfigV1Alpha1()
	rv2.MetaName = "data2"
	suite.Require().NoError(rv2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	rv2.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("1TiB")
	rv2.EncryptionSpec = blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &blockcfg.EncryptionKeyTPM{},
			},
			{
				KeySlot:   1,
				KeyStatic: &blockcfg.EncryptionKeyStatic{KeyData: "secret"},
			},
		},
	}

	ctr, err := container.New(rv1, rv2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	rawVolumes := []string{
		constants.RawVolumePrefix + "data1",
		constants.RawVolumePrefix + "data2",
	}

	ctest.AssertResources(suite, rawVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Contains(vc.Metadata().Labels().Raw(), block.RawVolumeLabel)

		asrt.Equal(block.VolumeTypePartition, vc.TypedSpec().Type)
		asrt.Contains(rawVolumes, vc.TypedSpec().Provisioning.PartitionSpec.Label)

		locator, err := vc.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)

		asrt.Contains(string(locator), vc.TypedSpec().Provisioning.PartitionSpec.Label)
		asrt.Equal(block.FilesystemTypeNone, vc.TypedSpec().Provisioning.FilesystemSpec.Type)

		asrt.Empty(vc.TypedSpec().Mount)
	})

	for _, volumeID := range rawVolumes {
		ctest.AssertNoResource[*block.VolumeMountRequest](suite, volumeID)
	}

	// drop the first volume
	ctr, err = container.New(rv2)
	suite.Require().NoError(err)

	newCfg := config.NewMachineConfig(ctr)
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(newCfg)

	// now the resources should be removed
	ctest.AssertNoResource[*block.VolumeConfig](suite, rawVolumes[0])
}

func (suite *VolumeConfigSuite) TestReconcileUserSwapVolumes() {
	userVolumeNames := []string{
		"data-part1",
		"data-part2",
		"data-dir1",
		"data-disk1",
	}

	uvPart1 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uvPart1.MetaName = userVolumeNames[0]
	suite.Require().NoError(uvPart1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
	uvPart1.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("10GiB")
	uvPart1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("100GiB")
	uvPart1.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	uvPart2 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uvPart2.MetaName = userVolumeNames[1]
	uvPart2.VolumeType = pointer.To(block.VolumeTypePartition)
	suite.Require().NoError(uvPart2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	uvPart2.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("1TiB")
	uvPart2.EncryptionSpec = blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &blockcfg.EncryptionKeyTPM{},
			},
			{
				KeySlot:   1,
				KeyStatic: &blockcfg.EncryptionKeyStatic{KeyData: "secret"},
			},
		},
	}

	uvDir1 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uvDir1.MetaName = userVolumeNames[2]
	uvDir1.VolumeType = pointer.To(block.VolumeTypeDirectory)

	uvDisk1 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uvDisk1.MetaName = userVolumeNames[3]
	suite.Require().NoError(uvDisk1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	uvDisk1.EncryptionSpec = blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &blockcfg.EncryptionKeyTPM{},
			},
			{
				KeySlot:   1,
				KeyStatic: &blockcfg.EncryptionKeyStatic{KeyData: "secret"},
			},
		},
	}

	sv1 := blockcfg.NewSwapVolumeConfigV1Alpha1()
	sv1.MetaName = "swap"
	suite.Require().NoError(sv1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))
	sv1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("2GiB")

	ctr, err := container.New(uvPart1, uvPart2, uvDir1, uvDisk1, sv1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	userVolumes := xslices.Map(userVolumeNames, func(in string) string { return constants.UserVolumePrefix + in })

	ctest.AssertResources(suite, userVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Contains(vc.Metadata().Labels().Raw(), block.UserVolumeLabel)

		switch vc.Metadata().ID() {
		case userVolumes[0], userVolumes[1], userVolumes[3]:
			asrt.Equal(block.VolumeTypePartition, vc.TypedSpec().Type)

			asrt.Contains(userVolumes, vc.TypedSpec().Provisioning.PartitionSpec.Label)

			locator, err := vc.TypedSpec().Locator.Match.MarshalText()
			asrt.NoError(err)

			asrt.Contains(string(locator), vc.TypedSpec().Provisioning.PartitionSpec.Label)

		case userVolumes[2]:
			asrt.Equal(block.VolumeTypeDirectory, vc.TypedSpec().Type)
		}

		asrt.Contains(userVolumeNames, vc.TypedSpec().Mount.TargetPath)
		asrt.Equal(constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)

		switch vc.Metadata().ID() {
		case userVolumes[0]:
			asrt.EqualValues(10*1024*1024*1024, vc.TypedSpec().Provisioning.PartitionSpec.MinSize)
		case userVolumes[1]:
			asrt.EqualValues(100*1024*1024, vc.TypedSpec().Provisioning.PartitionSpec.MinSize)
		}
	})

	ctest.AssertResources(suite, userVolumes, func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Contains(vmr.Metadata().Labels().Raw(), block.UserVolumeLabel)
	})

	swapVolumes := []string{
		constants.SwapVolumePrefix + "swap",
	}

	ctest.AssertResources(suite, swapVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Contains(vc.Metadata().Labels().Raw(), block.SwapVolumeLabel)

		asrt.Equal(block.VolumeTypePartition, vc.TypedSpec().Type)
		asrt.Contains(swapVolumes, vc.TypedSpec().Provisioning.PartitionSpec.Label)

		locator, err := vc.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)

		asrt.Contains(string(locator), vc.TypedSpec().Provisioning.PartitionSpec.Label)

		asrt.Equal(block.FilesystemTypeSwap, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
	})

	// simulate other controllers working - add finalizers for volume config & mount request
	for _, volumeID := range userVolumes {
		suite.AddFinalizer(block.NewVolumeConfig(block.NamespaceName, volumeID).Metadata(), "test")
		suite.AddFinalizer(block.NewVolumeMountRequest(block.NamespaceName, volumeID).Metadata(), "test")
	}

	// keep only the first volume
	ctr, err = container.New(uvPart1)
	suite.Require().NoError(err)

	newCfg := config.NewMachineConfig(ctr)
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(newCfg)

	// controller should tear down removed resources
	ctest.AssertResources(suite, userVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		if vc.Metadata().ID() == userVolumes[0] {
			asrt.Equal(resource.PhaseRunning, vc.Metadata().Phase())
		} else {
			asrt.Equal(resource.PhaseTearingDown, vc.Metadata().Phase())
		}
	})

	ctest.AssertResources(suite, userVolumes, func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
		if vmr.Metadata().ID() == userVolumes[0] {
			asrt.Equal(resource.PhaseRunning, vmr.Metadata().Phase())
		} else {
			asrt.Equal(resource.PhaseTearingDown, vmr.Metadata().Phase())
		}
	})

	// remove finalizers
	for _, userVolume := range userVolumes[1:] {
		suite.RemoveFinalizer(block.NewVolumeConfig(block.NamespaceName, userVolume).Metadata(), "test")
		suite.RemoveFinalizer(block.NewVolumeMountRequest(block.NamespaceName, userVolume).Metadata(), "test")
	}

	// now the resources should be removed
	for _, userVolume := range userVolumes[1:] {
		ctest.AssertNoResource[*block.VolumeConfig](suite, userVolume)
		ctest.AssertNoResource[*block.VolumeMountRequest](suite, userVolume)
	}
}
