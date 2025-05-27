// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
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

func (suite *UserVolumeConfigSuite) TestReconcile() {
	uv1 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uv1.MetaName = "data1"
	suite.Require().NoError(uv1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
	uv1.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("10GiB")
	uv1.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("100GiB")
	uv1.FilesystemSpec.FilesystemType = block.FilesystemTypeXFS

	uv2 := blockcfg.NewUserVolumeConfigV1Alpha1()
	uv2.MetaName = "data2"
	suite.Require().NoError(uv2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
	uv2.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustByteSize("1TiB")
	uv2.EncryptionSpec = blockcfg.EncryptionSpec{
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

	ctr, err := container.New(uv1, uv2, sv1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	userVolumes := []string{
		constants.UserVolumePrefix + "data1",
		constants.UserVolumePrefix + "data2",
	}

	ctest.AssertResources(suite, userVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Contains(vc.Metadata().Labels().Raw(), block.UserVolumeLabel)

		asrt.Equal(block.VolumeTypePartition, vc.TypedSpec().Type)
		asrt.Contains(userVolumes, vc.TypedSpec().Provisioning.PartitionSpec.Label)

		locator, err := vc.TypedSpec().Locator.Match.MarshalText()
		asrt.NoError(err)

		asrt.Contains(string(locator), vc.TypedSpec().Provisioning.PartitionSpec.Label)

		asrt.Contains([]string{"data1", "data2"}, vc.TypedSpec().Mount.TargetPath)
		asrt.Equal(constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
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

	// drop the first volume
	ctr, err = container.New(uv2)
	suite.Require().NoError(err)

	newCfg := config.NewMachineConfig(ctr)
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(newCfg)

	// controller should tear down removed volumes
	ctest.AssertResources(suite, userVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		if vc.Metadata().ID() == userVolumes[0] {
			asrt.Equal(resource.PhaseTearingDown, vc.Metadata().Phase())
		} else {
			asrt.Equal(resource.PhaseRunning, vc.Metadata().Phase())
		}
	})

	// controller should tear down removed volume resources
	ctest.AssertResources(suite, userVolumes, func(vc *block.VolumeConfig, asrt *assert.Assertions) {
		if vc.Metadata().ID() == userVolumes[0] {
			asrt.Equal(resource.PhaseTearingDown, vc.Metadata().Phase())
		} else {
			asrt.Equal(resource.PhaseRunning, vc.Metadata().Phase())
		}
	})

	ctest.AssertResources(suite, userVolumes, func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
		if vmr.Metadata().ID() == userVolumes[0] {
			asrt.Equal(resource.PhaseTearingDown, vmr.Metadata().Phase())
		} else {
			asrt.Equal(resource.PhaseRunning, vmr.Metadata().Phase())
		}
	})

	// remove finalizers
	suite.RemoveFinalizer(block.NewVolumeConfig(block.NamespaceName, userVolumes[0]).Metadata(), "test")
	suite.RemoveFinalizer(block.NewVolumeMountRequest(block.NamespaceName, userVolumes[0]).Metadata(), "test")

	// now the resources should be removed
	ctest.AssertNoResource[*block.VolumeConfig](suite, userVolumes[0])
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, userVolumes[0])
}
