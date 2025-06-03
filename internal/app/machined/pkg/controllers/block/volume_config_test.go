// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/yamlutils"
)

type VolumeConfigSuite struct {
	ctest.DefaultSuite
}

func TestVolumeConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VolumeConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeConfigController{}))
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
		asrt.EqualValues(partition.EphemeralMinSize, r.TypedSpec().Provisioning.PartitionSpec.MinSize)

		asrt.Equal(constants.EphemeralMountPoint, r.TypedSpec().Mount.TargetPath)
	})

	ctest.AssertResources(suite, []resource.ID{
		"/var/log",
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
				ProvisioningMaxSize: blockcfg.MustByteSize("2.5TiB"),
			},
		},
	)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	ctest.AssertResource(suite, constants.EphemeralPartitionLabel, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(r.TypedSpec().Provisioning)
		asrt.Empty(r.TypedSpec().Encryption)

		locator, err := r.TypedSpec().Provisioning.DiskSelector.Match.MarshalText()
		asrt.NoError(err)
		asrt.Equal(`disk.transport == "nvme"`, string(locator))

		asrt.False(r.TypedSpec().Provisioning.PartitionSpec.Grow)
		asrt.EqualValues(2.5*1024*1024*1024*1024, r.TypedSpec().Provisioning.PartitionSpec.MaxSize)
		asrt.EqualValues(partition.EphemeralMinSize, r.TypedSpec().Provisioning.PartitionSpec.MinSize)
	})
}
