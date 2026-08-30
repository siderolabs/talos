// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type LVMPhysicalVolumeSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSelectsMatchingDisks() {
	createDisk(&suite.DefaultSuite, "sda", "/dev/sda", "sata")
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `disk.transport == "nvme"`))

	ctest.AssertResources(
		suite,
		[]string{"nvme0n1", "nvme1n1"},
		func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal("vg-pool", pv.TypedSpec().VGName)
		},
	)

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "sda")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestEmptyConfigEmitsNothing() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")

	applyMachineConfig(&suite.DefaultSuite)

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "nvme0n1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestRemovingConfigCleansSpecs() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")

	cfg := applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `disk.transport == "nvme"`))

	ctest.AssertResources(
		suite,
		[]string{"nvme0n1"},
		func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal("/dev/nvme0n1", pv.TypedSpec().Device)
		},
	)

	suite.Destroy(cfg)

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "nvme0n1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSelectsPartitionByLabel() {
	// Whole disk plus a raw-volume partition on it.
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label == "r-lvmpv0"`))

	ctest.AssertResource(suite, "vdb1", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb1", pv.TypedSpec().Device)
		asrt.Equal("vg-pool", pv.TypedSpec().VGName)
	})

	// The disk-level selector must not also claim the whole parent disk.
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSelectsPartitionsByLabelPrefix() {
	// Mirrors the documented example selector
	// `volume.partition_label.startsWith("r-lvm")`.
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-lvmpv1")
	// A partition that should NOT match the prefix.
	createPartition(&suite.DefaultSuite, "vdb3", "/dev/vdb3", "/dev/vdb", "r-data0")

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label.startsWith("r-lvm")`))

	ctest.AssertResources(
		suite,
		[]string{"vdb1", "vdb2"},
		func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal("vg-pool", pv.TypedSpec().VGName)
		},
	)

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb3")
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsPartitionedDisk() {
	// A whole disk that carries partitions cannot be a PV; pvcreate rejects it.
	// The selector matches the whole disk (by transport) and a raw-volume
	// partition (by label); only the partition must yield a PV spec.
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-data0")
	// A whole, unpartitioned disk that should still be used directly.
	createDisk(&suite.DefaultSuite, "vdd", "/dev/vdd", "virtio")

	applyMachineConfig(&suite.DefaultSuite,
		newVGDoc("vg-pool", `disk.transport == "virtio" || volume.partition_label.startsWith("r-data")`),
	)

	ctest.AssertResources(
		suite,
		[]string{"vdb1", "vdd"},
		func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal("vg-pool", pv.TypedSpec().VGName)
		},
	)

	// The partitioned whole disk must not become a PV.
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestDiskSelectorMatchesWholeDiskOnly() {
	// A disk-level predicate matches only the whole disk, never its partitions:
	// partitions get an empty disk in the CEL context so disk.* evaluates false.
	// vdb is whole and unpartitioned; vdc carries a partition vdc1.
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createDisk(&suite.DefaultSuite, "vdc", "/dev/vdc", "virtio")
	createPartition(&suite.DefaultSuite, "vdc1", "/dev/vdc1", "/dev/vdc", "r-lvmpv0")

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `disk.dev_path == "/dev/vdb"`))

	ctest.AssertResource(suite, "vdb", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("vg-pool", pv.TypedSpec().VGName)
	})

	// A partition is never claimed by a disk-level selector.
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdc1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestOverlappingVGsSurfaceValidationError() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")

	applyMachineConfig(
		&suite.DefaultSuite,
		newVGDoc("vg-a", `disk.transport == "nvme"`),
		newVGDoc("vg-b", `disk.transport == "nvme"`),
	)

	// First VG (by config order) wins the device.
	ctest.AssertResource(suite, "nvme0n1", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("vg-a", pv.TypedSpec().VGName)
	})

	// The losing VG gets a validation error surfaced.
	ctest.AssertResource(suite, "vg-b", func(e *storageres.LVMValidationError, asrt *assert.Assertions) {
		asrt.Equal("vg-b", e.TypedSpec().VGName)
		asrt.Contains(e.TypedSpec().Message, "vg-a")
	})

	ctest.AssertNoResource[*storageres.LVMValidationError](suite, "vg-a")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestMultipleVGsDistinctDisks() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "sda", "/dev/sda", "sata")

	applyMachineConfig(
		&suite.DefaultSuite,
		newVGDoc("vg-nvme", `disk.transport == "nvme"`),
		newVGDoc("vg-sata", `disk.transport == "sata"`),
	)

	ctest.AssertResource(suite, "nvme0n1", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("vg-nvme", pv.TypedSpec().VGName)
	})

	ctest.AssertResource(suite, "sda", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("vg-sata", pv.TypedSpec().VGName)
	})
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSelectsDecryptedDeviceForEncryptedVolume() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createVolumeStatus(
		&suite.DefaultSuite, "lvmpv0",
		block.VolumePhaseReady, block.EncryptionProviderLUKS2,
		"/dev/vdb1", "/dev/dm-0",
	)

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label == "r-lvmpv0"`))

	ctest.AssertResource(suite, "dm-0", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/dm-0", pv.TypedSpec().Device)
		asrt.Equal("vg-pool", pv.TypedSpec().VGName)
	})

	// The ciphertext partition must never be handed to pvcreate.
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsEncryptedVolumeNotYetOpen() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-lvmpv1")
	createVolumeStatus(
		&suite.DefaultSuite, "lvmpv0",
		block.VolumePhaseWaiting, block.EncryptionProviderNone,
		"/dev/vdb1", "",
	)

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label.startsWith("r-lvm")`))

	ctest.AssertResource(suite, "vdb2", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb2", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "dm-0")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestUnencryptedVolumeStatusDoesNotRedirect() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createVolumeStatus(
		&suite.DefaultSuite, "lvmpv0",
		block.VolumePhaseReady, block.EncryptionProviderNone,
		"/dev/vdb1", "/dev/vdb1",
	)

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label == "r-lvmpv0"`))

	ctest.AssertResource(suite, "vdb1", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb1", pv.TypedSpec().Device)
	})
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsConfiguredEncryptedVolumeBeforeStatusLocation() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-plain0")
	createVolumeStatus(
		&suite.DefaultSuite, "lvmpv0",
		block.VolumePhaseWaiting, block.EncryptionProviderNone,
		"", "",
	)

	applyMachineConfigDocs(&suite.DefaultSuite,
		newEncryptedRawVolumeDoc("lvmpv0"),
		newVGDoc("vg-pool", `volume.partition_label.startsWith("r-")`),
	)

	ctest.AssertResource(suite, "vdb2", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb2", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsConfiguredEncryptedVolumeWithNoStatus() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-plain0")

	applyMachineConfigDocs(&suite.DefaultSuite,
		newEncryptedRawVolumeDoc("lvmpv0"),
		newVGDoc("vg-pool", `volume.partition_label.startsWith("r-")`),
	)

	ctest.AssertResource(suite, "vdb2", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb2", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsCiphertextWithNoStatusAndNoConfig() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createLUKSPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-plain0")

	applyMachineConfigDocs(&suite.DefaultSuite,
		newVGDoc("vg-pool", `volume.partition_label.startsWith("r-")`),
	)

	ctest.AssertResource(suite, "vdb2", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb2", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsEncryptedVolumeAfterStatusDestroyed() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-plain0")
	createDisk(&suite.DefaultSuite, "dm-0", "/dev/dm-0", "")

	createVolumeStatus(&suite.DefaultSuite, "r-lvmpv0", block.VolumePhaseReady,
		block.EncryptionProviderLUKS2, "/dev/vdb1", "/dev/dm-0")

	applyMachineConfigDocs(&suite.DefaultSuite,
		newEncryptedRawVolumeDoc("lvmpv0"),
		newVGDoc("vg-pool", `volume.partition_label.startsWith("r-")`),
	)

	ctest.AssertResource(suite, "dm-0", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/dm-0", pv.TypedSpec().Device)
	})

	vs := block.NewVolumeStatus(block.NamespaceName, "r-lvmpv0")
	suite.Destroy(vs)

	createPartition(&suite.DefaultSuite, "vdb3", "/dev/vdb3", "/dev/vdb", "r-plain1")

	ctest.AssertResource(suite, "vdb3", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb3", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestWaitsForVolumeManagerOnUnencryptedVolume() {
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createPartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-lvmpv0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-plain0")
	createVolumeStatus(
		&suite.DefaultSuite, "lvmpv0",
		block.VolumePhaseLocated, block.EncryptionProviderNone,
		"/dev/vdb1", "",
	)

	applyMachineConfig(&suite.DefaultSuite, newVGDoc("vg-pool", `volume.partition_label.startsWith("r-")`))

	// Barrier: the partition with no VolumeStatus at all is unaffected.
	ctest.AssertResource(suite, "vdb2", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb2", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "vdb1")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSkipsEncryptedWholeDiskVolumeBeforeStatus() {
	createDisk(&suite.DefaultSuite, "md0", "/dev/md0", "")
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")

	applyMachineConfigDocs(&suite.DefaultSuite,
		newEncryptedWholeDiskUserVolumeDoc("lvmdata", `disk.dev_path.startsWith("/dev/md")`),
		newVGDoc("vg-pool", `disk.dev_path.startsWith("/dev/md") || disk.transport == "virtio"`),
	)

	// Positive control: the plain disk still becomes a PV, proving reconciliation ran.
	ctest.AssertResource(suite, "vdb", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/vdb", pv.TypedSpec().Device)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "md0")
}

func (suite *LVMPhysicalVolumeSpecSuite) TestSelectsDecryptedDeviceForWholeDiskVolume() {
	// Once the array is open, the PV belongs on the opened device, not on /dev/md0.
	createDisk(&suite.DefaultSuite, "md0", "/dev/md0", "")
	createVolumeStatus(
		&suite.DefaultSuite, "u-lvmdata",
		block.VolumePhaseReady, block.EncryptionProviderLUKS2,
		"/dev/md0", "/dev/dm-0",
	)

	applyMachineConfigDocs(&suite.DefaultSuite,
		newEncryptedWholeDiskUserVolumeDoc("lvmdata", `disk.dev_path.startsWith("/dev/md")`),
		newVGDoc("vg-pool", `disk.dev_path.startsWith("/dev/md")`),
	)

	ctest.AssertResource(suite, "dm-0", func(pv *storageres.LVMPhysicalVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("/dev/dm-0", pv.TypedSpec().Device)
		asrt.Equal("vg-pool", pv.TypedSpec().VGName)
	})

	ctest.AssertNoResource[*storageres.LVMPhysicalVolumeSpec](suite, "md0")
}

func TestLVMPhysicalVolumeSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LVMPhysicalVolumeSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.LVMPhysicalVolumeSpecController{}))
			},
		},
	})
}
