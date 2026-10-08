// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type blankDiskValidationMode struct{}

func (blankDiskValidationMode) String() string        { return "" }
func (blankDiskValidationMode) RequiresInstall() bool { return false }
func (blankDiskValidationMode) InContainer() bool     { return false }

// TestBlankDiskAtomicPoolRemoval validates the same strategic deletion selectors used by the
// live suite without requiring a node. Only the combined result is valid while a VM references
// its pool, and named deletion must not disturb other documents.
func TestBlankDiskAtomicPoolRemoval(t *testing.T) {
	volume := blockcfg.NewUserVolumeConfigV1Alpha1()
	volume.MetaName = "backing"
	volume.VolumeType = new(block.VolumeTypeDirectory)

	pool := storagecfg.NewStoragePoolV1Alpha1()
	pool.MetaName = "target-pool"
	pool.VolumeConfig.VolumeName = volume.MetaName

	vm := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	vm.MetaName = "target-vm"
	vm.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	vm.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	vm.CPUConfig.CPUCount = 1
	vm.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	vm.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:   "data",
			DiskPool:   pool.MetaName,
			DiskSize:   meta.MustByteSize("64MiB"),
			DiskFormat: hypervisorhelpers.VirtualMachineDiskFormatQCOW2,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
			},
		},
	}

	otherVolume := blockcfg.NewUserVolumeConfigV1Alpha1()
	otherVolume.MetaName = "other-backing"
	otherVolume.VolumeType = new(block.VolumeTypeDirectory)

	otherPool := storagecfg.NewStoragePoolV1Alpha1()
	otherPool.MetaName = "other-pool"
	otherPool.VolumeConfig.VolumeName = otherVolume.MetaName

	initial, err := container.New(volume, pool, vm, otherVolume, otherPool)
	require.NoError(t, err)
	_, err = initial.ValidateAsClient(blankDiskValidationMode{}, validation.WithLocal())
	require.NoError(t, err)

	deleteVM := map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       hypervisorcfg.VirtualMachineConfigKind,
		"name":       vm.MetaName,
		"$patch":     "delete",
	}
	deletePool := map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       storagecfg.StoragePoolKind,
		"name":       pool.MetaName,
		"$patch":     "delete",
	}

	patches := make([]configpatcher.Patch, 0, 2)

	for _, selector := range []map[string]any{deleteVM, deletePool} {
		encoded, marshalErr := yaml.Marshal(selector)
		require.NoError(t, marshalErr)

		patch, loadErr := configpatcher.LoadPatch(encoded)
		require.NoError(t, loadErr)

		patches = append(patches, patch)
	}

	poolOnly, err := configpatcher.Apply(configpatcher.WithConfig(initial), patches[1:])
	require.NoError(t, err)
	poolOnlyConfig, err := poolOnly.Config()
	require.NoError(t, err)
	_, err = poolOnlyConfig.ValidateAsClient(blankDiskValidationMode{}, validation.WithLocal())
	require.ErrorContains(t, err, "no StoragePool document declares storage pool \"target-pool\"")

	combined, err := configpatcher.Apply(configpatcher.WithConfig(initial), patches)
	require.NoError(t, err)
	combinedConfig, err := combined.Config()
	require.NoError(t, err)
	_, err = combinedConfig.ValidateAsClient(blankDiskValidationMode{}, validation.WithLocal())
	require.NoError(t, err)

	remaining := make(map[string]bool)

	for _, doc := range combinedConfig.Documents() {
		named, ok := doc.(config.NamedDocument)
		require.True(t, ok, "unexpected unnamed document %s", doc.Kind())
		remaining[doc.Kind()+"/"+named.Name()] = true
	}

	require.Equal(t, map[string]bool{
		blockcfg.UserVolumeConfigKind + "/backing":       true,
		blockcfg.UserVolumeConfigKind + "/other-backing": true,
		storagecfg.StoragePoolKind + "/other-pool":       true,
	}, remaining)
}
