// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// newStoragePoolConfigDoc builds a StoragePoolConfig document on its own backing volume. Every virtual machine
// document here puts its disks in "pool1", so declaring it keeps the image-reference test about
// image references.
func newStoragePoolConfigDoc(name, volumeName string) *storagecfg.StoragePoolConfigV1Alpha1 {
	doc := storagecfg.NewStoragePoolConfigV1Alpha1()
	doc.MetaName = name
	doc.VolumeConfig.VolumeName = volumeName

	return doc
}

func newDirectoryUserVolumeDoc(name string) config.Document {
	doc := newUserVolumeDoc(name)
	doc.VolumeType = new(block.VolumeTypeDirectory)

	return doc
}

// poolDocs are the documents a virtual machine with a disk in "pool1" needs to resolve it.
func poolDocs() []config.Document {
	return []config.Document{newDirectoryUserVolumeDoc("vm-pool"), newStoragePoolConfigDoc("pool1", "vm-pool")}
}

// newVirtualMachineDoc builds a VirtualMachineConfig document whose disks are provisioned from the
// named content libraries, one disk per library.
func newVirtualMachineDoc(name string, libraryNames ...string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")

	for i, libraryName := range libraryNames {
		doc.DisksConfig = append(doc.DisksConfig, hypervisorcfg.VirtualMachineDisk{
			DiskName: "disk" + string(rune('a'+i)),
			DiskPool: "pool1",
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: libraryName,
					ImageFile:    "talos.qcow2",
				},
			},
		})
	}

	return doc
}

// newBlankDiskVirtualMachineDoc builds a VirtualMachineConfig whose single disk is blank, so it
// references no content library at all.
func newBlankDiskVirtualMachineDoc(name string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName: "system",
			DiskPool: "pool1",
			DiskSize: meta.MustByteSize("20GiB"),
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
			},
		},
	}

	return doc
}

func newCloudInitVirtualMachineDoc(name string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := newBlankDiskVirtualMachineDoc(name)
	doc.GuestConfig.CloudInitConfig = &hypervisorcfg.VirtualMachineCloudInit{
		PoolConfig:     "pool1",
		UserDataConfig: "#cloud-config\n",
	}

	return doc
}

func TestVirtualMachineImageReferencesAllowRetiredSources(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "no virtual machines",
			docs: []config.Document{
				newDirectoryUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
			},
		},
		{
			name: "no library referenced",
			docs: append(poolDocs(),
				newBlankDiskVirtualMachineDoc("vm1"),
			),
		},
		{
			name: "library declared",
			docs: append(poolDocs(),
				newDirectoryUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newVirtualMachineDoc("vm1", "images"),
			),
		},
		{
			name: "library not declared",
			docs: append(poolDocs(),
				newVirtualMachineDoc("vm1", "images"),
			),
		},
		{
			name: "one disk resolves, one does not",
			docs: append(poolDocs(),
				newDirectoryUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newVirtualMachineDoc("vm1", "images", "nowhere"),
			),
		},
		{
			name: "cloud-init needs no content library",
			docs: append(poolDocs(),
				newCloudInitVirtualMachineDoc("vm1"),
			),
		},
		{
			name: "two virtual machines, same missing library",
			docs: append(poolDocs(),
				newVirtualMachineDoc("vm1", "images"),
				newVirtualMachineDoc("vm2", "images"),
			),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(test.docs...)
			require.NoError(t, err)

			_, err = ctr.ValidateAsClient(validationMode{})

			if len(test.expectedErrs) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}
		})
	}
}

func TestVirtualMachineCloudInitPoolReferences(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		pool string
		want string
	}{
		{name: "declared", pool: "pool1"},
		{name: "undeclared", pool: "other", want: `guest.cloudInit: no StoragePoolConfig document declares storage pool "other"`},
		{name: "missing", want: "guest.cloudInit.pool is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			vm := newCloudInitVirtualMachineDoc("vm1")
			vm.GuestConfig.CloudInitConfig.PoolConfig = test.pool
			docs := append(poolDocs(), vm)
			ctr, err := container.New(docs...)
			require.NoError(t, err)

			_, err = ctr.ValidateAsClient(validationMode{})
			if test.want != "" {
				require.ErrorContains(t, err, test.want)

				if test.pool == "" {
					assert.NotContains(t, err.Error(), `declares storage pool ""`)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVirtualMachinePoolReferences(t *testing.T) {
	t.Parallel()

	// A cdrom copies media into a pool-owned volume; an undeclared pool must be reported.
	cdromDoc := func(name string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
		doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
		doc.MetaName = name
		doc.CPUConfig.CPUCount = 1
		doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")
		doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
			{
				DiskName: "boot",
				DiskPool: "pool1",
				DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
				ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
					FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
						ImageLibrary: "images",
						ImageFile:    "talos.iso",
					},
				},
			},
		}

		return doc
	}

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "no virtual machines",
			docs: poolDocs(),
		},
		{
			name: "pool declared",
			docs: append(poolDocs(), newBlankDiskVirtualMachineDoc("vm1")),
		},
		{
			name: "pool not declared",
			docs: []config.Document{newBlankDiskVirtualMachineDoc("vm1")},
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no StoragePoolConfig document declares storage pool "pool1"`,
			},
		},
		{
			name: "cdrom pool not declared",
			docs: []config.Document{cdromDoc("vm1")},
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no StoragePoolConfig document declares storage pool "pool1"`,
			},
		},
		{
			name: "cdrom pool declared",
			docs: append(poolDocs(), cdromDoc("vm1")),
		},
		{
			name: "two virtual machines, same missing pool",
			docs: []config.Document{
				newBlankDiskVirtualMachineDoc("vm1"),
				newBlankDiskVirtualMachineDoc("vm2"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no StoragePoolConfig document declares storage pool "pool1"`,
				`virtual machine "vm2": disks[0]: no StoragePoolConfig document declares storage pool "pool1"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(test.docs...)
			require.NoError(t, err)

			_, err = ctr.ValidateAsClient(validationMode{})

			if len(test.expectedErrs) == 0 {
				if err != nil {
					// The bare documents fail their own validation, which is not what this test is
					// about.
					assert.NotContains(t, err.Error(), "no StoragePoolConfig document declares")
				}

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}
		})
	}
}
