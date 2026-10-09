// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const (
	isoContents = "not really an ISO, but it hashes like one"

	// Every case in this suite works on one virtual machine drawing from one content library.
	vmName      = "vm"
	libraryName = "images"
)

type VirtualMachineDiskSuite struct {
	ctest.DefaultSuite

	// disks remembers what createVM configured, because a disk status is keyed by what the disk is
	// provisioned from as well as by its name.
	disks map[string]hypervisor.VirtualMachineDiskSpec
}

func TestVirtualMachineDiskSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineDiskSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDiskController{}))
		},
	})
}

// library writes an ISO into a fresh directory and publishes a ready content library over it.
func (suite *VirtualMachineDiskSuite) library() string {
	suite.T().Helper()

	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte(isoContents), 0o600))

	status := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	*status.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-" + libraryName,
		Path:     path,
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	suite.Create(status)

	return path
}

func (suite *VirtualMachineDiskSuite) createVM(disks ...hypervisor.VirtualMachineDiskSpec) {
	suite.T().Helper()

	suite.disks = map[string]hypervisor.VirtualMachineDiskSpec{}

	for _, disk := range disks {
		suite.disks[disk.Name] = disk
	}

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Disks:      disks,
	}
	suite.Create(spec)
}

func cdromDiskSpec(name, library, file, dgst string) hypervisor.VirtualMachineDiskSpec {
	return hypervisor.VirtualMachineDiskSpec{
		Name: name,
		Bus:  "sata",
		Type: "cdrom",
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
				Library: library,
				File:    file,
				Digest:  dgst,
			},
		},
	}
}

// diskStatusID is the ID the status of one of the configured disks is published under.
func (suite *VirtualMachineDiskSuite) diskStatusID(disk string) string {
	suite.T().Helper()

	spec, found := suite.disks[disk]
	suite.Require().True(found, "disk %q was never configured", disk)

	return hypervisor.VirtualMachineDiskStatusID(vmName, spec)
}

func (suite *VirtualMachineDiskSuite) assertDisk(disk string, check func(hypervisor.VirtualMachineDiskStatusSpec, *assert.Assertions)) {
	suite.T().Helper()

	ctest.AssertResource(suite, suite.diskStatusID(disk),
		func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(vmName, res.TypedSpec().VirtualMachine)
			asrt.Equal(disk, res.TypedSpec().Name)

			check(*res.TypedSpec(), asrt)
		})
}

func (suite *VirtualMachineDiskSuite) TestResolvesCDROMInPlace() {
	path := suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Empty(spec.Error)
		asrt.Equal(filepath.Join(path, "talos.iso"), spec.SourcePath)
		asrt.Equal("raw", spec.Format)
		asrt.True(spec.ReadOnly)
		asrt.Equal(hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.iso"}, spec.Image)
	})
}

func (suite *VirtualMachineDiskSuite) TestVerifiesDigest() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", digest.FromString(isoContents).String()))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Empty(spec.Error)
	})
}

// Ready is not terminal: every event must re-observe and rehash the file, even while a
// domain holds the status. A failed observation must not release the library underneath it.
func (suite *VirtualMachineDiskSuite) TestRevalidatesReadyImageAfterFileChange() {
	path := suite.library()
	dgst := digest.FromString(isoContents).String()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", dgst))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("install"))
	suite.AddFinalizer(status.Metadata(), "consumer")

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)

	for _, contents := range []string{"changed image", isoContents} {
		suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte(contents), 0o600))
		// File changes do not emit resource events; changing a library field triggers observation.
		ctest.UpdateWithConflicts(suite, library, func(res *hypervisor.ContentLibraryStatus) error {
			res.TypedSpec().Error = contents

			return nil
		})

		suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			if contents == isoContents {
				asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
				asrt.Empty(spec.Error)
				asrt.Equal(filepath.Join(path, "talos.iso"), spec.SourcePath)
			} else {
				asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
				asrt.Equal(`content library "images": file "talos.iso": digest mismatch: expected `+dgst, spec.Error)
				asrt.Empty(spec.SourcePath)
			}

			asrt.Equal(dgst, spec.Image.Digest)
		})
		suite.assertLibraryHeld(true)
	}
}

func (suite *VirtualMachineDiskSuite) TestRejectsDigestMismatch() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", digest.FromString("something else").String()))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "digest mismatch")
		asrt.Empty(spec.SourcePath)
	})
}

func (suite *VirtualMachineDiskSuite) TestReportsMissingFile() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "absent.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "absent.iso")
		// A failure is attributed too, so that a reader of the status knows which image it is about.
		asrt.Equal(hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "absent.iso"}, spec.Image)
	})
}

func (suite *VirtualMachineDiskSuite) TestRejectsDirectory() {
	path := suite.library()
	suite.Require().NoError(os.Mkdir(filepath.Join(path, "nested"), 0o700))
	suite.createVM(cdromDiskSpec("install", libraryName, "nested", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "not a regular file")
	})
}

func (suite *VirtualMachineDiskSuite) TestReportsUnconfiguredLibrary() {
	suite.createVM(cdromDiskSpec("install", "absent", "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, `content library "absent" is not configured`)
	})
}

func (suite *VirtualMachineDiskSuite) TestWaitsForLibraryToBecomeReady() {
	status := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	*status.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-" + libraryName,
		Error:    "volume is not mounted",
	}
	suite.Create(status)
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "volume is not mounted")
	})

	// A library a disk is still waiting on stays held throughout: giving the hold back and taking it
	// again on the next pass would be an endless churn of its finalizers.
	suite.assertLibraryHeld(true)
	ctx, st := suite.Ctx(), suite.State()

	suite.Require().Never(func() bool {
		library, err := safe.StateGet[*hypervisor.ContentLibraryStatus](ctx, st, status.Metadata())

		return err != nil || !library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController")
	}, 200*time.Millisecond, 10*time.Millisecond)

	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte(isoContents), 0o600))

	ctest.UpdateWithConflicts(suite, status, func(res *hypervisor.ContentLibraryStatus) error {
		res.TypedSpec().Path = path
		res.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
		res.TypedSpec().Error = ""

		return nil
	})

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Equal(filepath.Join(path, "talos.iso"), spec.SourcePath)
	})
}

// A materialized disk whose format is not qcow2 is reported, not silently dropped: both linked
// and copy modes write a qcow2 file, and letting a raw-format request through would make the
// status disagree with what landed on disk.
func (suite *VirtualMachineDiskSuite) TestReportsUnsupportedDisks() {
	suite.createVM(
		hypervisor.VirtualMachineDiskSpec{
			Name: "system", Pool: "pool1", Size: 20 << 30, Format: "raw", Bus: "virtio", Type: "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.qcow2"},
			},
		},
	)

	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "a materialized disk must be qcow2")
	})
}

// A shared VM spec need not have passed machine-configuration validation. An ambiguous source
// must not request a blank volume or publish a Ready source for the domain to attach.
func (suite *VirtualMachineDiskSuite) TestRejectsBothBlankAndImageSources() {
	suite.library()

	disk := blankDiskSpec("data", "pool1", "qcow2", 20<<30)
	disk.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
		Library: libraryName,
		File:    "talos.iso",
	}
	suite.createVM(disk)

	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Contains(spec.Error, "blank and fromImage are mutually exclusive")
		asrt.Empty(spec.SourcePath)
	})

	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite,
		storage.StoragePoolVolumeID("pool1", vmName+"__data.qcow2"))
	suite.assertLibraryHeld(false)
}

// blankDiskSpec is a disk provisioned as an empty volume in a pool.
func blankDiskSpec(name, pool, format string, size uint64) hypervisor.VirtualMachineDiskSpec {
	return hypervisor.VirtualMachineDiskSpec{
		Name:      name,
		Pool:      pool,
		Size:      size,
		Format:    format,
		Bus:       "virtio",
		Type:      "disk",
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
	}
}

// volume publishes the answer the storage slice would give for a blank disk's volume.
func (suite *VirtualMachineDiskSuite) volume(name, format string, ready bool) {
	suite.T().Helper()

	const (
		pool     = "pool1"
		capacity = 20 << 30
	)

	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID(pool, name))

	*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     pool,
		Name:     name,
		Path:     "/var/mnt/u-vms/" + pool + "/" + name,
		Format:   format,
		Capacity: capacity,
		Phase:    storage.StoragePoolVolumePhaseNotReady,
	}
	if ready {
		status.TypedSpec().Phase = storage.StoragePoolVolumePhaseReady
	}

	if !ready {
		status.TypedSpec().Error = "storage pool is not ready"
	}

	suite.Create(status, state.WithCreateOwner("storage.StoragePoolVolumeController"))
}

// A blank disk asks the storage slice for its volume, naming it after the virtual machine and the
// disk, and reports the volume once it is there.
func (suite *VirtualMachineDiskSuite) TestBlankDiskAsksForItsVolume() {
	suite.createVM(blankDiskSpec("data", "pool1", "qcow2", 20<<30))

	id := storage.StoragePoolVolumeID("pool1", vmName+"__data.qcow2")

	ctest.AssertResource(suite, id, func(res *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("pool1", res.TypedSpec().Pool)
		asrt.Equal(vmName+"__data.qcow2", res.TypedSpec().Name)
		asrt.Equal(uint64(20<<30), res.TypedSpec().Capacity)
		asrt.Equal("qcow2", res.TypedSpec().Format)
	})

	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "waiting for volume")
		// Stamped whether or not the disk resolved: this is what tells the pool it is in use.
		asrt.Equal("pool1", spec.Pool)
		asrt.Equal(vmName+"__data.qcow2", spec.Volume)
		asrt.True(spec.Blank)
	})

	suite.volume(vmName+"__data.qcow2", "qcow2", true)

	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady, spec.Error)
		asrt.Equal("/var/mnt/u-vms/pool1/"+vmName+"__data.qcow2", spec.SourcePath)
		asrt.Equal("qcow2", spec.Format)
		asrt.False(spec.ReadOnly, "a blank disk is the guest's to write into")
		asrt.Equal(uint64(20<<30), spec.Size)
	})
}

// Dropping configuration withdraws desired existence even while the attached disk
// is held. Storage delays request destruction until consumer-held status is released.
func (suite *VirtualMachineDiskSuite) TestKeepsHeldBlankVolumeUntilConsumersRelease() {
	suite.createVM(blankDiskSpec("data", "pool1", "raw", 20<<30))
	suite.volume(vmName+"__data.raw", "raw", true)
	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("data"))
	volume := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", vmName+"__data.raw"))

	suite.AddFinalizer(status.Metadata(), "domain")
	suite.AddFinalizer(volume.Metadata(), "storage")
	suite.Destroy(hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName))

	ctest.AssertResource(suite, status.Metadata().ID(), func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
	})
	ctest.AssertResource(suite, volume.Metadata().ID(), func(res *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		asrt.True(res.Metadata().Finalizers().Has("storage"))
	})

	suite.RemoveFinalizer(status.Metadata(), "domain")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, status.Metadata().ID())
	ctest.AssertResource(suite, volume.Metadata().ID(), func(res *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		asrt.True(res.Metadata().Finalizers().Has("storage"))
	})

	suite.RemoveFinalizer(volume.Metadata(), "storage")
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, volume.Metadata().ID())
}

func (suite *VirtualMachineDiskSuite) TestVolumePhasePropagationAndRecovery() {
	suite.createVM(blankDiskSpec("data", "pool1", "raw", 20<<30))

	name := vmName + "__data.raw"
	suite.volume(name, "raw", false)
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", name))

	cases := []struct {
		volumePhase storage.StoragePoolVolumePhase
		diskPhase   hypervisor.VirtualMachineDiskPhase
	}{
		{
			volumePhase: storage.StoragePoolVolumePhaseUnknown,
			diskPhase:   hypervisor.VirtualMachineDiskPhaseNotReady,
		},
		{
			volumePhase: storage.StoragePoolVolumePhaseNotReady,
			diskPhase:   hypervisor.VirtualMachineDiskPhaseNotReady,
		},
		{
			volumePhase: storage.StoragePoolVolumePhaseObservationUnavailable,
			diskPhase:   hypervisor.VirtualMachineDiskPhaseObservationUnavailable,
		},
		{
			volumePhase: storage.StoragePoolVolumePhaseReady,
			diskPhase:   hypervisor.VirtualMachineDiskPhaseReady,
		},
		{
			volumePhase: storage.StoragePoolVolumePhaseNotReady,
			diskPhase:   hypervisor.VirtualMachineDiskPhaseNotReady,
		},
	}

	for _, tt := range cases {
		ctest.UpdateWithConflicts(suite, volume, func(current *storage.StoragePoolVolumeStatus) error {
			current.TypedSpec().Phase = tt.volumePhase
			current.TypedSpec().Error = "storage observation unavailable"

			return nil
		}, state.WithUpdateOwner("storage.StoragePoolVolumeController"))
		suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(tt.diskPhase, spec.Phase)
		})
	}
}

func (suite *VirtualMachineDiskSuite) TestBlankDiskWaitsForAnUnreadyVolume() {
	suite.createVM(blankDiskSpec("data", "pool1", "qcow2", 20<<30))
	suite.volume(vmName+"__data.qcow2", "qcow2", false)

	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "storage pool is not ready")
	})
}

// Two names joined by one hyphen are not one name: "a-b" plus "c" and "a" plus "b-c" would be the
// same file, and two virtual machines would share one writable volume.
func TestBlankVolumeNamesAreInjective(t *testing.T) {
	t.Parallel()

	first := hypervisorctrl.BlankVolumeNameForTest("vm-a", blankDiskSpec("b", "pool1", "qcow2", 1<<30))
	second := hypervisorctrl.BlankVolumeNameForTest("vm", blankDiskSpec("a-b", "pool1", "qcow2", 1<<30))

	assert.NotEqual(t, first, second)
	assert.Equal(t, "vm-a__b.qcow2", first)
	assert.Equal(t, "vm__a-b.qcow2", second)
}

// A blank disk whose shape no volume could be made from is permanently unsupported, not pending.
func (suite *VirtualMachineDiskSuite) TestRejectsUnmakeableBlankDisks() {
	for _, test := range []struct {
		name   string
		disk   hypervisor.VirtualMachineDiskSpec
		reason string
	}{
		{"no size", blankDiskSpec("data", "pool1", "qcow2", 0), "requires a size"},
		{"bad format", blankDiskSpec("data", "pool1", "vmdk", 1<<30), "unsupported format"},
		// The enum's zero member parses by name, so a producer other than a machine configuration
		// can carry it here. It names no format, and libvirt would be handed it verbatim.
		{"zero format", blankDiskSpec("data", "pool1", "unknown", 1<<30), "unsupported format"},
		{"no pool", blankDiskSpec("data", "", "qcow2", 1<<30), "storage pool name is required"},
	} {
		suite.Run(test.name, func() {
			suite.createVM(test.disk)
			suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
				asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
				asrt.Contains(spec.Error, test.reason)
			})

			// The spec ID is the virtual machine's name, so each case has to give it back.
			suite.Destroy(hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName))
			ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("data"))
		})
	}
}

func (suite *VirtualMachineDiskSuite) TestRemovesStatusesWhenDisksDisappear() {
	suite.library()
	suite.createVM(
		cdromDiskSpec("install", libraryName, "talos.iso", ""),
		cdromDiskSpec("rescue", libraryName, "talos.iso", ""),
	)

	for _, disk := range []string{"install", "rescue"} {
		suite.assertDisk(disk, func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		})
	}

	spec, err := ctest.Get[*hypervisor.VirtualMachineSpec](suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata())
	suite.Require().NoError(err)

	ctest.UpdateWithConflicts(suite, spec, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().Disks = res.TypedSpec().Disks[:1]

		return nil
	})

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("rescue"))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
	})

	suite.Destroy(spec)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("install"))
}

// The image is attached where it lies, so the library's mount has to stay put. The hold is taken
// before the disk is reported ready: a ready disk is one something may start using at any moment.
func (suite *VirtualMachineDiskSuite) TestHoldsTheLibraryItResolvedAgainst() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
	})

	suite.assertLibraryHeld(true)

	spec, err := ctest.Get[*hypervisor.VirtualMachineSpec](suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata())
	suite.Require().NoError(err)
	suite.Destroy(spec)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("install"))
	suite.assertLibraryHeld(false)
}

// A disk status something else holds is a disk a domain is reading from. It tears down and waits,
// and the library it resolved against waits with it.
func (suite *VirtualMachineDiskSuite) TestKeepsAHeldStatusAndItsLibrary() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
	})

	id := suite.diskStatusID("install")
	diskStatus := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	suite.AddFinalizer(diskStatus.Metadata(), "consumer")

	spec, err := ctest.Get[*hypervisor.VirtualMachineSpec](suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata())
	suite.Require().NoError(err)
	suite.Destroy(spec)

	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
	})

	suite.assertLibraryHeld(true)

	suite.RemoveFinalizer(diskStatus.Metadata(), "consumer")

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, id)
	suite.assertLibraryHeld(false)
}

// Holding a library which is on its way out would block its teardown forever, so a disk resolved
// against one is held back instead.
func (suite *VirtualMachineDiskSuite) TestRefusesALibraryOnItsWayOut() {
	suite.library()

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	suite.AddFinalizer(library.Metadata(), "somebody-else")
	_, err := suite.State().Teardown(suite.Ctx(), library.Metadata())
	suite.Require().NoError(err)

	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "is going away")
	})

	suite.assertLibraryHeld(false)
}

// assertLibraryHeld waits for the content library to carry, or to have given back, the hold the
// disk controller takes on it.
func (suite *VirtualMachineDiskSuite) assertLibraryHeld(held bool) {
	suite.T().Helper()

	ctest.AssertResource(suite, libraryName, func(res *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.Equal(held, res.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController"))
	})
}

// Configuration can come back to an image whose status a domain has not let go of yet. Nothing can
// be written to a status in that phase, so the disk waits for the previous one to finish leaving
// and is published afresh after it.
func (suite *VirtualMachineDiskSuite) TestWaitsOutATearingDownStatusOfTheSameDisk() {
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
	})

	id := suite.diskStatusID("install")
	diskStatus := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	suite.AddFinalizer(diskStatus.Metadata(), "consumer")

	vm, err := ctest.Get[*hypervisor.VirtualMachineSpec](suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata())
	suite.Require().NoError(err)

	ctest.UpdateWithConflicts(suite, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().Disks = nil

		return nil
	})

	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
	})

	ctest.UpdateWithConflicts(suite, vm, func(res *hypervisor.VirtualMachineSpec) error {
		res.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}

		return nil
	})

	// The status the configuration asks for again is the one still leaving, so there is none to use.
	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
	})

	suite.RemoveFinalizer(diskStatus.Metadata(), "consumer")

	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, res.Metadata().Phase())
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, res.TypedSpec().Phase)
	})
}
