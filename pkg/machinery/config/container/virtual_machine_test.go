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
)

// newStoragePoolDoc builds a StoragePool document on its own backing volume. Every virtual machine
// document here puts its disks in "pool1", so declaring it keeps the image-reference test about
// image references.
func newStoragePoolDoc(name, volumeName string) *storagecfg.StoragePoolV1Alpha1 {
	doc := storagecfg.NewStoragePoolV1Alpha1()
	doc.MetaName = name
	doc.VolumeConfig.VolumeName = volumeName

	return doc
}

// poolDocs are the documents a virtual machine with a disk in "pool1" needs to resolve it.
func poolDocs() []config.Document {
	return []config.Document{newUserVolumeDoc("vm-pool"), newStoragePoolDoc("pool1", "vm-pool")}
}

// newVirtualMachineDoc builds a VirtualMachineConfig document whose disks are provisioned from the
// named content libraries, one disk per library.
func newVirtualMachineDoc(name string, libraryNames ...string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")

	for i, libraryName := range libraryNames {
		doc.DisksConfig = append(doc.DisksConfig, hypervisorcfg.VirtualMachineDisk{
			DiskName: "disk" + string(rune('a'+i)),
			DiskPool: "pool1",
			DiskSize: meta.MustByteSize("20GiB"),
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

func newCloudInitVirtualMachineDoc(name, library string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := newBlankDiskVirtualMachineDoc(name)
	doc.GuestConfig.CloudInitConfig = &hypervisorcfg.VirtualMachineCloudInit{
		LibraryConfig:  library,
		UserDataConfig: "#cloud-config\n",
	}

	return doc
}

func TestVirtualMachineImageReferences(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "no virtual machines",
			docs: []config.Document{
				newUserVolumeDoc("vm-images"),
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
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newVirtualMachineDoc("vm1", "images"),
			),
		},
		{
			name: "library not declared",
			docs: append(poolDocs(),
				newVirtualMachineDoc("vm1", "images"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no ContentLibraryConfig declares content library "images"`,
			},
		},
		{
			name: "one disk resolves, one does not",
			docs: append(poolDocs(),
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newVirtualMachineDoc("vm1", "images", "nowhere"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": disks[1]: no ContentLibraryConfig declares content library "nowhere"`,
			},
		},
		{
			name: "cloud-init library declared",
			docs: append(poolDocs(),
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newCloudInitVirtualMachineDoc("vm1", "images"),
			),
		},
		{
			name: "cloud-init library not declared",
			docs: append(poolDocs(),
				newCloudInitVirtualMachineDoc("vm1", "images"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": guest.cloudInit: no ContentLibraryConfig declares content library "images"`,
			},
		},
		{
			name: "cloud-init library differs from declared library",
			docs: append(poolDocs(),
				newUserVolumeDoc("vm-images"),
				newContentLibraryDoc("images", "vm-images"),
				newCloudInitVirtualMachineDoc("vm1", "seeds"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": guest.cloudInit: no ContentLibraryConfig declares content library "seeds"`,
			},
		},
		{
			name: "multiple cloud-init libraries not declared",
			docs: append(poolDocs(),
				newCloudInitVirtualMachineDoc("vm2", "seeds"),
				newCloudInitVirtualMachineDoc("vm1", "images"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": guest.cloudInit: no ContentLibraryConfig declares content library "images"`,
				`virtual machine "vm2": guest.cloudInit: no ContentLibraryConfig declares content library "seeds"`,
			},
		},
		{
			name: "cloud-init missing library name",
			docs: append(poolDocs(),
				newCloudInitVirtualMachineDoc("vm1", ""),
			),
		},
		{
			name: "two virtual machines, same missing library",
			docs: append(poolDocs(),
				newVirtualMachineDoc("vm1", "images"),
				newVirtualMachineDoc("vm2", "images"),
			),
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no ContentLibraryConfig declares content library "images"`,
				`virtual machine "vm2": disks[0]: no ContentLibraryConfig declares content library "images"`,
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
					// The bare volume documents fail their own validation, which is not what this
					// test is about.
					assert.NotContains(t, err.Error(), "no ContentLibraryConfig declares")
					assert.NotContains(t, err.Error(), "disks[")
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

func TestVirtualMachinePoolReferences(t *testing.T) {
	t.Parallel()

	// A cdrom takes no pool at all, so it must not be reported as naming an undeclared one.
	cdromDoc := func(name string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
		doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
		doc.MetaName = name
		doc.CPUConfig.CPUCount = 1
		doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")
		doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
			{
				DiskName: "boot",
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
				`virtual machine "vm1": disks[0]: no StoragePool document declares storage pool "pool1"`,
			},
		},
		{
			name: "cdrom needs no pool",
			docs: []config.Document{cdromDoc("vm1")},
		},
		{
			name: "two virtual machines, same missing pool",
			docs: []config.Document{
				newBlankDiskVirtualMachineDoc("vm1"),
				newBlankDiskVirtualMachineDoc("vm2"),
			},
			expectedErrs: []string{
				`virtual machine "vm1": disks[0]: no StoragePool document declares storage pool "pool1"`,
				`virtual machine "vm2": disks[0]: no StoragePool document declares storage pool "pool1"`,
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
					assert.NotContains(t, err.Error(), "no StoragePool document declares")
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
