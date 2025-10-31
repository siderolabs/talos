// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type UserVolumeConfigSuite struct {
	ctest.DefaultSuite
}

func TestUserVolumeConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &UserVolumeConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.UserVolumeConfigController{}))
			},
		},
	})
}

func (suite *UserVolumeConfigSuite) TestReconcileUserVolumesSwapVolumes() {
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
	uvPart1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("100GiB")
	uvPart1.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	uvPart2 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uvPart2.MetaName = userVolumeNames[1]
	uvPart2.VolumeType = pointer.To(block.VolumeTypePartition)
	suite.Require().NoError(uvPart2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	uvPart2.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("1TiB")
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
	sv1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("2GiB")

	ctr, err := container.New(uvPart1, uvPart2, uvDir1, uvDisk1, sv1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	userVolumes := xslices.Map(userVolumeNames, func(in string) string { return constants.UserVolumePrefix + in })

	ctest.AssertResources(suite, userVolumeNames, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
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

func (suite *UserVolumeConfigSuite) TestReconcileRawVolumes() {
	rv1 := blockcfg.NewRawVolumeConfigV1Alpha1()
	rv1.MetaName = "data1"
	suite.Require().NoError(rv1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
	rv1.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("10GiB")
	rv1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("100GiB")

	rv2 := blockcfg.NewRawVolumeConfigV1Alpha1()
	rv2.MetaName = "data2"
	suite.Require().NoError(rv2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	rv2.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("1TiB")
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
