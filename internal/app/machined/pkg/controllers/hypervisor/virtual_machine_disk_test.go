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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	cas "github.com/siderolabs/talos/internal/pkg/contentlibrary"
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
	writeLibraryImage(suite.T(), path, "talos.iso", isoContents)

	status := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	*status.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeID: "u-" + libraryName,
		Path:     path,
		Phase:    hypervisor.ContentLibraryPhaseReady,
	}
	suite.Create(status)

	return path
}

func writeLibraryImage(t *testing.T, path, name, contents string) {
	t.Helper()

	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	staged, err := os.CreateTemp(path, ".fixture-*.upload")
	require.NoError(t, err)
	_, err = staged.WriteString(contents)
	require.NoError(t, err)
	require.NoError(t, staged.Sync())
	require.NoError(t, staged.Close())

	hash := digest.SHA256.FromString(contents).String()
	require.NoError(t, cas.Publish(root, filepath.Base(staged.Name()), name, hash))
	require.NoError(t, os.Remove(staged.Name()))
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
		Name:   name,
		Pool:   "pool1",
		Format: "raw",
		Bus:    "sata",
		Type:   "cdrom",
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

func (suite *VirtualMachineDiskSuite) TestCopiesCDROMIntoPoolAndSurvivesLibraryRemoval() {
	libraryPath := suite.library()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool = "pool1"
	disk.Format = "raw"
	suite.createVM(disk)

	var name string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		name = spec.Volume
		asrt.NotEmpty(name)
	})

	id := storage.StoragePoolVolumeID("pool1", name)
	ctest.AssertResource(suite, id, func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		if asrt.NotNil(spec.TypedSpec().Copy) {
			asrt.Equal(filepath.Join(libraryPath, "objects", "sha256", digest.SHA256.FromString(isoContents).Encoded()), spec.TypedSpec().Copy.Source)
		}

		asrt.Equal("raw", spec.TypedSpec().Format)
	})

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
	suite.assertLibraryHeld(true)

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	_, err := suite.State().Teardown(suite.Ctx(), library.Metadata())
	suite.Require().NoError(err)

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "pool1",
		Name:     name,
		Path:     "/pool1/" + name,
		Format:   "raw",
		Capacity: uint64(len(isoContents)),
		Phase:    storage.StoragePoolVolumePhaseObservationUnavailable,
	}
	suite.Create(volume)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseObservationUnavailable, spec.Phase)
	})
	suite.assertLibraryHeld(true)
	ctest.UpdateWithConflicts(suite, volume, func(current *storage.StoragePoolVolumeStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolVolumePhaseReady

		return nil
	})

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(volume.TypedSpec().Path, spec.SourcePath)
		asrt.True(spec.ReadOnly)
	})
	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("install"))
	suite.AddFinalizer(status.Metadata(), "consumer")
	suite.assertLibraryHeld(false)

	suite.Destroy(library)
	ctest.AssertNoResource[*hypervisor.ContentLibraryStatus](suite, libraryName)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(volume.TypedSpec().Path, spec.SourcePath)
	})
}

func (suite *VirtualMachineDiskSuite) TestChangedCDROMIdentityDoesNotAttachOldCopy() {
	path := suite.library()
	first := cdromDiskSpec("install", libraryName, "talos.iso", digest.SHA256.FromString(isoContents).String())
	first.Pool, first.Format = "pool1", "raw"
	suite.createVM(first)

	var oldName string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		oldName = spec.Volume
		asrt.NotEmpty(oldName)
	})

	oldID := storage.StoragePoolVolumeID(first.Pool, oldName)
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, oldID)
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool: first.Pool, Name: oldName, Path: "/pool1/" + oldName, Format: "raw", Phase: storage.StoragePoolVolumePhaseReady,
	}
	suite.Create(volume)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(volume.TypedSpec().Path, spec.SourcePath)
	})

	writeLibraryImage(suite.T(), path, "different.iso", "different ISO bytes")

	second := first
	second.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
		Library: libraryName, File: "different.iso", Digest: digest.SHA256.FromString("different ISO bytes").String(),
	}

	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{second}

		return nil
	})
	suite.disks["install"] = second
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEqual(oldName, spec.Volume)
		asrt.NotEqual(volume.TypedSpec().Path, spec.SourcePath)
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
}

func (suite *VirtualMachineDiskSuite) TestPublishesCopyAdoptionWithoutLibraryOrEphemeralVolume() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool, disk.Format = "pool1", "raw"
	suite.createVM(disk)

	var volumeName string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		volumeName = spec.Volume
		asrt.NotEmpty(volumeName)
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
	ctest.AssertResource(suite, storage.StoragePoolVolumeID(disk.Pool, volumeName), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		if asrt.NotNil(spec.TypedSpec().Copy) {
			asrt.Empty(spec.TypedSpec().Copy.Source)
		}
	})
}

func (suite *VirtualMachineDiskSuite) TestAdoptsReadyCopyWithoutSourceLibrary() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool, disk.Format = "pool1", "raw"
	suite.createVM(disk)

	var name string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		name = spec.Volume
		asrt.NotEmpty(name)
	})

	id := storage.StoragePoolVolumeID(disk.Pool, name)
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool: disk.Pool, Name: name, Path: "/pool1/" + name, Format: "raw", Phase: storage.StoragePoolVolumePhaseReady,
	}
	suite.Create(volume)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(volume.TypedSpec().Path, spec.SourcePath)
	})
}

func (suite *VirtualMachineDiskSuite) TestRejectsLinkedCDROMWithoutCopying() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool, disk.Format = "pool1", "raw"
	disk.Provision.FromImage.Mode = "linked"
	suite.createVM(disk)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Contains(spec.Error, "unsupported image mode")
	})
}

func (suite *VirtualMachineDiskSuite) TestDoesNotAdoptUnrelatedVolumeRequest() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))

	var volumeName string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
		volumeName = spec.Volume
	})

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", volumeName))
	ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotNil(spec.TypedSpec().Copy)
	})
	ctest.UpdateWithConflicts(suite, request, func(spec *storage.StoragePoolVolumeSpec) error {
		spec.TypedSpec().Copy = nil

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	ctest.UpdateWithConflicts(suite, vm, func(spec *hypervisor.VirtualMachineSpec) error {
		spec.TypedSpec().Memory.Size++

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Contains(spec.Error, "belongs to another request")
	})
	ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Nil(spec.TypedSpec().Copy)
	})
}

func (suite *VirtualMachineDiskSuite) TestDoesNotAdoptAnotherControllersReadyCopy() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)

	var name string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		name = spec.Volume
		asrt.NotEmpty(name)
	})

	id := storage.StoragePoolVolumeID(disk.Pool, name)

	suite.Destroy(hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName))
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("install"))
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, id)

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, id)
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool: disk.Pool, Name: name, Format: "raw", Copy: &storage.StoragePoolVolumeCopySpec{},
	}
	suite.Create(request, state.WithCreateOwner("another.Controller"))

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool: disk.Pool, Name: name, Path: "/pool1/" + name, Format: "raw", Phase: storage.StoragePoolVolumePhaseReady,
	}
	suite.Create(volume)
	suite.createVM(disk)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Empty(spec.SourcePath)
	})
	ctest.AssertResource(suite, id, func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal("another.Controller", spec.Metadata().Owner())
		asrt.Equal(resource.PhaseRunning, spec.Metadata().Phase())
	})
}

func (suite *VirtualMachineDiskSuite) TestDoesNotWithdrawAnotherControllersPoolCopy() {
	const owner = "hypervisor.CloudInitISOController"

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", "other.iso"))
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:   "pool1",
		Name:   "other.iso",
		Format: "raw",
		Copy:   &storage.StoragePoolVolumeCopySpec{},
	}
	suite.Create(request, state.WithCreateOwner(owner))
	suite.AddFinalizer(request.Metadata(), "consumer")

	suite.createVM(blankDiskSpec("data", "pool1", "raw", 20<<30))
	suite.volume(vmName+"__data.raw", "raw", true)
	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	ctest.UpdateWithConflicts(suite, vm, func(spec *hypervisor.VirtualMachineSpec) error {
		spec.TypedSpec().Disks = nil

		return nil
	})
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("data"))
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, storage.StoragePoolVolumeID("pool1", vmName+"__data.raw"))
	ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(owner, spec.Metadata().Owner())
		asrt.Equal(resource.PhaseRunning, spec.Metadata().Phase())
		asrt.True(spec.Metadata().Finalizers().Has("consumer"))
	})
}

func (suite *VirtualMachineDiskSuite) TestRejectsWritableImageWithoutRequestingACopy() {
	suite.library()

	disk := blankDiskSpec("data", "pool1", "raw", 20<<30)
	disk.Provision.Blank = false
	disk.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
		Library: libraryName,
		File:    "talos.iso",
		Mode:    "copy",
	}
	suite.createVM(disk)

	suite.assertDisk("data", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Contains(spec.Error, "a disk is only provisioned from provision.blank today")
	})
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite,
		storage.StoragePoolVolumeID("pool1", vmName+"__data.raw"))
	suite.assertLibraryHeld(false)
}

func (suite *VirtualMachineDiskSuite) TestResolvesCDROMFromPool() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))
	path := suite.completeImageCopy("install")

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Empty(spec.Error)
		asrt.Equal(path, spec.SourcePath)
		asrt.Equal("raw", spec.Format)
		asrt.True(spec.ReadOnly)
		asrt.Equal(hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.iso"}, spec.Image)
	})
	suite.assertLibraryHeld(false)
}

func (suite *VirtualMachineDiskSuite) TestVerifiesDigest() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", digest.FromString(isoContents).String()))
	suite.completeImageCopy("install")

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Empty(spec.Error)
	})
}

// A completed copy is independent of later source changes, even while a domain holds the disk.
func (suite *VirtualMachineDiskSuite) TestReadyCopyDoesNotRevalidateChangedSource() {
	path := suite.library()
	dgst := digest.FromString(isoContents).String()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", dgst))
	copyPath := suite.completeImageCopy("install")
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("install"))
	suite.AddFinalizer(status.Metadata(), "consumer")

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)

	for _, contents := range []string{"changed image", isoContents} {
		suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte(contents), 0o600))
		ctest.UpdateWithConflicts(suite, library, func(res *hypervisor.ContentLibraryStatus) error {
			res.TypedSpec().Error = contents

			return nil
		})
		suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
			asrt.Empty(spec.Error)
			asrt.Equal(copyPath, spec.SourcePath)
			asrt.Equal(dgst, spec.Image.Digest)
		})
		suite.assertLibraryHeld(false)
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
		asrt.Contains(spec.Error, "not a symlink")
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
	writeLibraryImage(suite.T(), path, "talos.iso", isoContents)

	ctest.UpdateWithConflicts(suite, status, func(res *hypervisor.ContentLibraryStatus) error {
		res.TypedSpec().Path = path
		res.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
		res.TypedSpec().Error = ""

		return nil
	})

	copyPath := suite.completeImageCopy("install")
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Equal(copyPath, spec.SourcePath)
	})
}

// A disk this slice does not provision is reported, not silently dropped: the operator sees why
// the virtual machine never gets a domain.
func (suite *VirtualMachineDiskSuite) TestReportsUnsupportedDisks() {
	suite.createVM(
		hypervisor.VirtualMachineDiskSpec{
			Name: "system", Pool: "pool1", Size: 20 << 30, Format: "qcow2", Bus: "virtio", Type: "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.qcow2"},
			},
		},
	)

	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "a disk is only provisioned from provision.blank today")
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
		suite.completeImageCopy(disk)
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

// Source holds end at publication, before a domain can acquire the independent disk.
func (suite *VirtualMachineDiskSuite) TestReleasesSourceLibraryAfterCopy() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))
	suite.completeImageCopy("install")

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
	})

	suite.assertLibraryHeld(false)

	spec, err := ctest.Get[*hypervisor.VirtualMachineSpec](suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName).Metadata())
	suite.Require().NoError(err)
	suite.Destroy(spec)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("install"))
	suite.assertLibraryHeld(false)
}

// A held disk status waits for detach without retaining the source library.
func (suite *VirtualMachineDiskSuite) TestKeepsHeldStatusWithoutHoldingSourceLibrary() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""))
	suite.completeImageCopy("install")

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

	suite.assertLibraryHeld(false)

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

// completeImageCopy publishes the pool observation after the controller has resolved its source.
func (suite *VirtualMachineDiskSuite) completeImageCopy(disk string) string {
	suite.T().Helper()

	var name string

	suite.assertDisk(disk, func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
		name = spec.Volume
	})
	ctest.AssertResource(suite, storage.StoragePoolVolumeID("pool1", name), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		if asrt.NotNil(spec.TypedSpec().Copy) {
			asrt.NotEmpty(spec.TypedSpec().Copy.Source)
		}
	})
	suite.volume(name, "raw", true)

	return "/var/mnt/u-vms/pool1/" + name
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
	suite.completeImageCopy("install")

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
