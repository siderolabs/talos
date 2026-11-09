// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/owned"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	cas "github.com/siderolabs/talos/internal/pkg/contentlibrary"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
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

	// Disk statuses include configured provisioning identity as well as the disk name.
	disks     map[string]hypervisor.VirtualMachineDiskSpec
	copyPause *atomic.Pointer[imageCopyPause]
	failures  chan error
}

func TestVirtualMachineDiskSuite(t *testing.T) {
	t.Parallel()

	testSuite := &VirtualMachineDiskSuite{}
	testSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(current *ctest.DefaultSuite) {
			pause := &atomic.Pointer[imageCopyPause]{}
			testSuite.copyPause = pause
			testSuite.failures = make(chan error, 20)
			current.Require().NoError(current.Runtime().RegisterController(&pausedImageCopyController{pause: pause, failures: testSuite.failures}))
		},
	}
	suite.Run(t, testSuite)
}

// library writes an ISO into a fresh directory and publishes a ready content library over it.
func (suite *VirtualMachineDiskSuite) library() string {
	suite.T().Helper()

	path := suite.T().TempDir()
	writeLibraryImage(suite.T(), path, "talos.iso", isoContents)

	status := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)
	*status.TypedSpec() = hypervisor.ContentLibraryStatusSpec{
		VolumeName: libraryName,
		Path:       path,
		Phase:      hypervisor.ContentLibraryPhaseReady,
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
	stagedInfo, err := staged.Stat()
	require.NoError(t, err)
	require.NoError(t, staged.Close())

	hash := digest.SHA256.FromString(contents).String()
	require.NoError(t, cas.Publish(root, filepath.Base(staged.Name()), name, hash, stagedInfo))
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

	pool := suite.copyPool()
	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "pool1",
		Name:     name,
		Path:     filepath.Join(pool.TypedSpec().TargetPath, name),
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

		request, getErr := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
		if getErr != nil {
			return getErr
		}

		current.TypedSpec().Copy = &storage.StoragePoolVolumeCopyStatusSpec{
			Phase:  storage.StoragePoolVolumeCopyPhaseSucceeded,
			Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
		}

		return nil
	})

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(volume.TypedSpec().Path, spec.SourcePath)
		asrt.True(spec.ReadOnly)
	})
	suite.recordCompletedBinding(name)
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
	oldPath := suite.publishImageCopy(oldID)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(oldPath, spec.SourcePath)
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
		asrt.NotEqual(oldPath, spec.SourcePath)
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
}

func (suite *VirtualMachineDiskSuite) TestAliasReplacementSelectsDifferentCopy() {
	path := suite.library()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)

	var first string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		first = spec.Volume
		asrt.NotEmpty(first)
	})

	suite.publishImageCopy(storage.StoragePoolVolumeID(disk.Pool, first))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(first, spec.Volume)
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})

	root, err := os.OpenRoot(path)
	suite.Require().NoError(err)
	suite.Require().NoError(root.Remove("talos.iso"))
	suite.Require().NoError(root.Close())
	writeLibraryImage(suite.T(), path, "talos.iso", "second image")
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Error = "alias updated"

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEqual(first, spec.Volume)
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, storage.StoragePoolVolumeID(disk.Pool, first))
}

func (suite *VirtualMachineDiskSuite) TestDoesNotGuessCopyWithoutLibraryOrBinding() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Empty(spec.Volume)
		asrt.Contains(spec.Error, "not configured")
	})
}

func (suite *VirtualMachineDiskSuite) TestAdoptsReadyCopyWithoutSourceLibrary() {
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool, disk.Format = "pool1", "raw"
	suite.createVM(disk)

	var name string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		name = spec.Volume
		asrt.NotEmpty(name)
	})

	suite.recordCompletedBinding(name)
	id := storage.StoragePoolVolumeID(disk.Pool, name)
	copyPath := suite.publishImageCopy(id)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})
	suite.assertLibraryHeld(false)
	suite.Destroy(hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(name, spec.Volume)
		asrt.Equal(copyPath, spec.SourcePath)
	})
}

// recordCompletedBinding simulates storage's completed publication with a real same-inode witness.
func (suite *VirtualMachineDiskSuite) recordCompletedBinding(name string) {
	suite.T().Helper()
	poolPath := suite.copyPool().TypedSpec().TargetPath
	suite.Require().NoError(os.MkdirAll(filepath.Join(poolPath, ".copy-on-use"), 0o700))
	suite.Require().NoError(os.WriteFile(filepath.Join(poolPath, name), []byte(isoContents), 0o600))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	suite.Require().NoError(os.Link(filepath.Join(poolPath, name), filepath.Join(poolPath, ".copy-on-use", witness)))

	request, err := ctest.Get[*storage.StoragePoolVolumeSpec](suite, storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID("pool1", name)).Metadata())
	suite.Require().NoError(err)

	key, ok := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	suite.Require().True(ok)

	bindings := copyowner.Bindings{}
	suite.Require().NoError(bindings.Record(poolPath, key, name))
}

// copyPool publishes the observed backing once so copy statuses use its persisted identity.
func (suite *VirtualMachineDiskSuite) copyPool() *storage.StoragePoolStatus {
	suite.T().Helper()

	pool, err := safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), "pool1")
	if err == nil {
		return pool
	}

	suite.Require().True(state.IsNotFoundError(err), "failed to read copy pool: %v", err)

	pool = storage.NewStoragePoolStatus(storage.NamespaceName, "pool1")
	*pool.TypedSpec() = storage.StoragePoolStatusSpec{
		TargetPath: suite.T().TempDir(),
		Phase:      storage.StoragePoolPhaseReady,
	}
	suite.Create(pool)
	pool, err = safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), "pool1")
	suite.Require().NoError(err)

	return pool
}

// publishImageCopy models storage's status for the current persisted request and pool.
func (suite *VirtualMachineDiskSuite) publishImageCopy(id string) string {
	suite.T().Helper()
	pool := suite.copyPool()
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)
	suite.Require().NotNil(request.TypedSpec().Copy)
	path := filepath.Join(pool.TypedSpec().TargetPath, request.TypedSpec().Name)
	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     request.TypedSpec().Pool,
		Name:     request.TypedSpec().Name,
		Path:     path,
		Format:   request.TypedSpec().Format,
		Capacity: uint64(len(isoContents)),
		Phase:    storage.StoragePoolVolumePhaseReady,
		Copy: &storage.StoragePoolVolumeCopyStatusSpec{
			Phase:  storage.StoragePoolVolumeCopyPhaseSucceeded,
			Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
		},
	}
	suite.Require().NoError(safe.StateModify(suite.Ctx(), suite.State(), status, func(current *storage.StoragePoolVolumeStatus) error {
		*current.TypedSpec() = *status.TypedSpec()

		return nil
	}))

	return path
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
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)

	var name string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		name = spec.Volume
		asrt.NotEmpty(name)
	})

	id := storage.StoragePoolVolumeID(disk.Pool, name)
	original, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)

	suite.Destroy(hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName))
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, suite.diskStatusID("install"))
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, id)

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, id)
	*request.TypedSpec() = *original.TypedSpec()
	suite.Create(request, state.WithCreateOwner("another.Controller"))

	suite.publishImageCopy(id)
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
	suite.library()

	dgst := digest.FromString(isoContents).String()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", dgst)

	suite.createVM(disk)
	copyPath := suite.completeImageCopy("install")
	suite.recordCompletedBinding(filepath.Base(copyPath))
	suite.assertLibraryHeld(false)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("install"))
	suite.AddFinalizer(status.Metadata(), "consumer")

	for _, test := range []struct {
		name     string
		contents string
	}{
		{name: "malformed-alias"},
		{name: "digest-mismatch", contents: "changed image"},
		{name: "missing-source"},
	} {
		suite.Run(test.name, func() {
			path := suite.T().TempDir()

			switch test.name {
			case "malformed-alias":
				suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte("not an alias"), 0o600))
			case "digest-mismatch":
				writeLibraryImage(suite.T(), path, "talos.iso", test.contents)
			}

			ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(res *hypervisor.ContentLibraryStatus) error {
				res.TypedSpec().Path = path

				return nil
			})

			// Reconstruct the output after changing the source. An old Ready observation
			// cannot satisfy recovery, and the durable request keeps its original intent.
			suite.RemoveFinalizer(status.Metadata(), "consumer")
			suite.Destroy(status, state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))
			ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(spec *hypervisor.VirtualMachineSpec) error {
				spec.Metadata().Labels().Set("test/recovery", test.name)

				return nil
			})
			assertReady := func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
				asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
				asrt.Empty(spec.Error)
				asrt.Equal(copyPath, spec.SourcePath)
				asrt.Equal(filepath.Base(copyPath), spec.Volume)
				asrt.Equal(dgst, spec.Image.Digest)
			}
			suite.assertDisk("install", assertReady)
			suite.AddFinalizer(status.Metadata(), "consumer")

			// Also require a fresh reconciliation while the consumer holds the disk.
			ctest.UpdateWithConflicts(suite, status, func(current *hypervisor.VirtualMachineDiskStatus) error {
				current.TypedSpec().Phase = hypervisor.VirtualMachineDiskPhaseNotReady
				current.TypedSpec().Error = "awaiting held-disk reconciliation"
				current.TypedSpec().SourcePath = ""

				return nil
			}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
			suite.assertDisk("install", assertReady)
			suite.assertLibraryHeld(false)
		})
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
	suite.assertLibraryHeld(false)
	requests, err := safe.StateListAll[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State())
	suite.Require().NoError(err)
	suite.Zero(requests.Len())
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
		VolumeName: libraryName,
		Error:      "volume is not mounted",
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
			Name: "system", Pool: "pool1", Size: 20 << 30, Format: "vmdk", Bus: "virtio", Type: "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.qcow2"},
			},
		},
	)

	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Phase == hypervisor.VirtualMachineDiskPhaseReady)
		asrt.Contains(spec.Error, "unsupported format")
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

func (suite *VirtualMachineDiskSuite) TestWritableImageRequestsStableDestinationBeforeSource() {
	disk := cdromDiskSpec("system", libraryName, "guest.qcow2", "")
	disk.Type = "disk"
	disk.Format = "qcow2"
	disk.Size = 20 << 30
	suite.createVM(disk)

	id := storage.StoragePoolVolumeID("pool1", vmName+"__system.img")
	ctest.AssertResource(suite, id, func(res *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(20<<30), res.TypedSpec().Capacity)
		asrt.Equal("qcow2", res.TypedSpec().Format)
		asrt.NotNil(res.TypedSpec().Copy)
		asrt.True(res.TypedSpec().Copy.Image)
	})
}

func (suite *VirtualMachineDiskSuite) TestWritableImageKeepsDestinationAfterAliasAndSelectorChanges() {
	path := suite.library()
	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type = "disk"
	disk.Size = 20 << 30
	suite.createVM(disk)
	destination := suite.completeImageCopy("system")
	id := storage.StoragePoolVolumeID("pool1", vmName+"__system.img")
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)

	original := *request.TypedSpec().Copy

	suite.Require().NoError(os.Remove(filepath.Join(path, "talos.iso")))
	writeLibraryImage(suite.T(), path, "talos.iso", "replacement source")
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(destination, spec.SourcePath)
		asrt.False(spec.ReadOnly)
	})

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	ctest.UpdateWithConflicts(suite, vm, func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks[0].Provision.FromImage.File = "missing.qcow2"
		current.TypedSpec().Disks[0].Provision.FromImage.Digest = digest.SHA256.FromString("different").String()
		suite.disks["system"] = current.TypedSpec().Disks[0]

		return nil
	})
	suite.assertLibraryHeld(false)
	suite.Destroy(hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName))
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(destination, spec.SourcePath)
	})
	ctest.AssertResource(suite, id, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(original, *current.TypedSpec().Copy)
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

	if suite.disks[disk].IsWritableImage() {
		suite.requireWritableImageSource(storage.StoragePoolVolumeID("pool1", name))
	}

	ctest.AssertResource(suite, storage.StoragePoolVolumeID("pool1", name), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		if asrt.NotNil(spec.TypedSpec().Copy) {
			asrt.NotEmpty(spec.TypedSpec().Copy.Source)
		}
	})

	return suite.publishImageCopy(storage.StoragePoolVolumeID("pool1", name))
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

	// The recreated request needs a fresh storage observation. The old request's
	// success cannot qualify a new incarnation, even when the destination is unchanged.
	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, res.Metadata().Phase())
		asrt.NotEqual(hypervisor.VirtualMachineDiskPhaseReady, res.TypedSpec().Phase)
	})
	suite.completeImageCopy("install")

	ctest.AssertResource(suite, id, func(res *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, res.Metadata().Phase())
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, res.TypedSpec().Phase)
	})
}

func (suite *VirtualMachineDiskSuite) TestSHA512CompletedCopySurvivesMissingLibrary() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", digest.SHA512.FromString(isoContents).String()))
	path := suite.completeImageCopy("install")
	suite.recordCompletedBinding(filepath.Base(path))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})
	suite.assertLibraryHeld(false)
	suite.Destroy(hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName))
	disk := cdromDiskSpec("install", libraryName, "talos.iso", digest.SHA512.FromString(isoContents).String())
	suite.Destroy(hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, disk)), state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(spec *hypervisor.VirtualMachineSpec) error {
		spec.Metadata().Labels().Set("test/recovery", "source-free")

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(filepath.Base(path), spec.Volume)
	})
}

func (suite *VirtualMachineDiskSuite) TestCompletedBindingRequiresCurrentDestination() {
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", digest.SHA256.FromString(isoContents).String())
	suite.createVM(disk)
	copyPath := suite.completeImageCopy("install")
	suite.recordCompletedBinding(filepath.Base(copyPath))
	suite.assertLibraryHeld(false)

	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte("not an alias"), 0o600))
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(library *hypervisor.ContentLibraryStatus) error {
		library.TypedSpec().Path = path

		return nil
	})

	id := storage.StoragePoolVolumeID(disk.Pool, filepath.Base(copyPath))

	for _, test := range []string{"stale-intent", "changed-backing", "missing-binding"} {
		suite.Run(test, func() {
			switch test {
			case "stale-intent":
				ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id), func(status *storage.StoragePoolVolumeStatus) error {
					status.TypedSpec().Copy.Intent = "old-attempt"

					return nil
				})
			case "changed-backing":
				suite.publishImageCopy(id)
				ctest.UpdateWithConflicts(suite, storage.NewStoragePoolStatus(storage.NamespaceName, disk.Pool), func(pool *storage.StoragePoolStatus) error {
					pool.TypedSpec().VolumeID = "different-backing"

					return nil
				})
			case "missing-binding":
				suite.publishImageCopy(id)
				request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
				suite.Require().NoError(err)

				key, _ := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
				pool := suite.copyPool()
				suite.Require().NoError(os.Remove(filepath.Join(pool.TypedSpec().TargetPath, ".copy-on-use", "binding-"+key)))
			}

			suite.Destroy(hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("install")), state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))
			suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
				asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
				asrt.Contains(spec.Error, "not a symlink")
				asrt.Empty(spec.SourcePath)
			})

			if test == "missing-binding" {
				suite.assertLibraryHeld(false)
			}
		})
	}
}

func (suite *VirtualMachineDiskSuite) TestRebindsFailedUnpublishedSource() {
	suite.testFailedSourceRebind(false, false, "failed")
}

func (suite *VirtualMachineDiskSuite) TestDoesNotRebindPublishedSourceAfterFailure() {
	suite.testFailedSourceRebind(true, false, "failed")
}

func (suite *VirtualMachineDiskSuite) testFailedSourceRebind(published, draining bool, outcome string) {
	oldRoot := suite.library()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.NotEmpty(spec.Volume)
	})
	observed, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](suite.Ctx(), suite.State(), hypervisor.VirtualMachineDiskStatusID(vmName, disk))
	suite.Require().NoError(err)

	name := observed.TypedSpec().Volume
	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID(disk.Pool, name))
	oldSource := filepath.Join(oldRoot, "objects", "sha256", digest.SHA256.FromString(isoContents).Encoded())

	ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(oldSource, spec.TypedSpec().Copy.Source)
	})

	if draining {
		suite.AddFinalizer(request.Metadata(), "test.copy-running")
	}

	target := suite.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, disk.Pool)
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = target
	suite.Create(pool)

	if published {
		suite.Require().NoError(os.WriteFile(filepath.Join(target, name), []byte(isoContents), 0o600))
	}

	boundRequest, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), request.Metadata().ID())
	suite.Require().NoError(err)
	pool, err = safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), pool.Metadata().ID())
	suite.Require().NoError(err)

	status := failedSourceCopyStatus(boundRequest, pool, outcome)
	if outcome != "missing" {
		suite.Create(status)
	}

	if outcome == "unreadable" {
		suite.copyPause.Store(&imageCopyPause{kind: storage.StoragePoolVolumeStatusType, id: request.Metadata().ID(), failRead: true})
	}

	replacement := suite.T().TempDir()
	writeLibraryImage(suite.T(), replacement, "talos.iso", isoContents)
	suite.Require().NoError(os.Remove(oldSource))
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(library *hypervisor.ContentLibraryStatus) error {
		library.TypedSpec().Path = replacement

		return nil
	})

	expected := filepath.Join(replacement, "objects", "sha256", digest.SHA256.FromString(isoContents).Encoded())

	if outcome != "failed" {
		ctx, st := suite.Ctx(), suite.State()
		suite.Require().Never(func() bool {
			bound, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](ctx, st, request.Metadata().ID())

			return err != nil || bound.Metadata().Phase() != resource.PhaseRunning || bound.TypedSpec().Copy.Source != oldSource
		}, 300*time.Millisecond, 10*time.Millisecond, "unknown copy outcome must preserve its source binding")

		return
	}

	if draining {
		ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseTearingDown, spec.Metadata().Phase())
			asrt.Equal(oldSource, spec.TypedSpec().Copy.Source)
		})
		// The old copy publishes after the absence check, while its request drains.
		suite.Require().NoError(os.WriteFile(filepath.Join(target, name), []byte(isoContents), 0o600))
		suite.Require().NoError(os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

		witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		suite.Require().NoError(os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
		old, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), request.Metadata().ID())
		suite.Require().NoError(err)
		suite.Equal(oldSource, old.TypedSpec().Copy.Source)
		suite.RemoveFinalizer(request.Metadata(), "test.copy-running")
	}

	if published {
		expected = oldSource

		suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
			asrt.Contains(spec.Error, "publication")
		})
	}

	ctest.AssertResource(suite, request.Metadata().ID(), func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(expected, spec.TypedSpec().Copy.Source)
		asrt.Equal(digest.SHA256.FromString(isoContents).String(), spec.TypedSpec().Copy.Digest)
	})
}

func failedSourceCopyStatus(request *storage.StoragePoolVolumeSpec, pool *storage.StoragePoolStatus, outcome string) *storage.StoragePoolVolumeStatus {
	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())
	*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Phase: storage.StoragePoolVolumePhaseNotReady,
		Error: "old source disappeared",
		Copy: &storage.StoragePoolVolumeCopyStatusSpec{
			Phase:  storage.StoragePoolVolumeCopyPhaseFailed,
			Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
		},
	}

	switch outcome {
	case "running":
		status.TypedSpec().Copy.Phase = storage.StoragePoolVolumeCopyPhaseRunning
	case "stale-failure":
		status.TypedSpec().Copy.Intent = "old-attempt"
	}

	return status
}

func (suite *VirtualMachineDiskSuite) TestSourceReplacementWaitsForOldCopyDrain() {
	suite.testFailedSourceRebind(false, true, "failed")
}

func (suite *VirtualMachineDiskSuite) TestStaleBindingRetainsIncompleteCopySourceHold() {
	path := suite.library()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)
	firstPath := suite.completeImageCopy("install")
	suite.recordCompletedBinding(filepath.Base(firstPath))
	suite.assertLibraryHeld(false)

	suite.Require().NoError(os.Remove(filepath.Join(path, "talos.iso")))
	writeLibraryImage(suite.T(), path, "talos.iso", "replacement")
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Error = "alias replaced"

		return nil
	})

	var second string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		second = spec.Volume
		asrt.NotEmpty(second)
		asrt.NotEqual(filepath.Base(firstPath), second)
	})
	ctest.AssertResource[*storage.StoragePoolVolumeSpec](suite, storage.StoragePoolVolumeID(disk.Pool, second), func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(current.TypedSpec().Copy.Source)
	})
	pool, err := safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), disk.Pool)
	suite.Require().NoError(err)
	suite.Require().NoError(os.Remove(filepath.Join(pool.TypedSpec().TargetPath, filepath.Base(firstPath))))
	suite.Require().NoError(os.Remove(filepath.Join(pool.TypedSpec().TargetPath, ".copy-on-use", fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Base(firstPath)))))))
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Equal(second, spec.Volume)
		asrt.Contains(spec.Error, "invalid copy binding")
	})
	ctx, st := suite.Ctx(), suite.State()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName)

	suite.Require().Never(func() bool {
		current, err := st.Get(ctx, library.Metadata())

		return err != nil || !current.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController")
	}, 250*time.Millisecond, 10*time.Millisecond, "an incomplete copy must retain its source even with a stale completed binding")

	// Simulate the storage owner's acknowledged B publication; its current-intent fence
	// must still select B while the source library is not ready.
	target := pool.TypedSpec().TargetPath
	suite.Require().NoError(os.WriteFile(filepath.Join(target, second), []byte("replacement"), 0o600))
	suite.Require().NoError(os.Link(filepath.Join(target, second), filepath.Join(target, ".copy-on-use", fmt.Sprintf("%x", sha256.Sum256([]byte(second))))))
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), storage.StoragePoolVolumeID(disk.Pool, second))
	suite.Require().NoError(err)

	key, _ := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	bindings := copyowner.Bindings{}
	suite.Require().NoError(bindings.Record(target, key, second))
	suite.completeImageCopy("install")
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(second, spec.Volume)
	})
	suite.assertLibraryHeld(false)
}

// These wrappers pause a real semantic interface, not the filesystem helper under test.
type imageCopyPause struct {
	kind     resource.Type
	id       resource.ID
	volume   string
	after    bool
	entered  chan struct{}
	release  chan struct{}
	resumed  sync.Once
	failRead bool
}

func (pause *imageCopyPause) matches(res resource.Resource, after bool) bool {
	if pause.after != after || pause.kind != res.Metadata().Type() || pause.id != res.Metadata().ID() {
		return false
	}

	if !after {
		return true
	}

	status, ok := res.(*hypervisor.VirtualMachineDiskStatus)

	return ok && status.TypedSpec().Volume == pause.volume
}

func (pause *imageCopyPause) resume() {
	pause.resumed.Do(func() { close(pause.release) })
}

type pausedImageCopyController struct {
	hypervisorctrl.VirtualMachineDiskController
	pause    *atomic.Pointer[imageCopyPause]
	failures chan error
}

func (ctrl *pausedImageCopyController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	err := ctrl.VirtualMachineDiskController.Run(ctx, &pausedImageCopyRuntime{
		Runtime: runtime,
		pause:   ctrl.pause,
	}, logger)
	if err != nil && ctrl.failures != nil {
		select {
		case ctrl.failures <- err:
		default:
		}
	}

	return err
}

type pausedImageCopyRuntime struct {
	controller.Runtime
	pause *atomic.Pointer[imageCopyPause]
}

func (runtime *pausedImageCopyRuntime) wait(ctx context.Context, res resource.Resource, after bool) error {
	pause := runtime.pause.Load()
	if pause == nil || !pause.matches(res, after) {
		return nil
	}

	if !runtime.pause.CompareAndSwap(pause, nil) {
		return nil
	}

	close(pause.entered)

	select {
	case <-pause.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (runtime *pausedImageCopyRuntime) Get(ctx context.Context, ptr resource.Pointer, options ...state.GetOption) (resource.Resource, error) {
	pause := runtime.pause.Load()
	if pause != nil && pause.failRead && pause.kind == ptr.Type() && pause.id == ptr.ID() {
		return nil, errors.New("injected status-read failure")
	}

	return runtime.Runtime.Get(ctx, ptr, options...)
}

func (runtime *pausedImageCopyRuntime) Modify(ctx context.Context, res resource.Resource, modify func(resource.Resource) error, options ...owned.ModifyOption) error {
	if err := runtime.wait(ctx, res, false); err != nil {
		return err
	}

	return runtime.Runtime.Modify(ctx, res, func(current resource.Resource) error {
		if err := modify(current); err != nil {
			return err
		}

		return runtime.wait(ctx, current, true)
	}, options...)
}

func (runtime *pausedImageCopyRuntime) ModifyWithResult(ctx context.Context, res resource.Resource, modify func(resource.Resource) error, options ...owned.ModifyOption) (resource.Resource, error) {
	if err := runtime.wait(ctx, res, false); err != nil {
		return nil, err
	}

	return runtime.Runtime.ModifyWithResult(ctx, res, func(current resource.Resource) error {
		if err := modify(current); err != nil {
			return err
		}

		return runtime.wait(ctx, current, true)
	}, options...)
}

func (suite *VirtualMachineDiskSuite) TestSourceFreeSelectionCannotOverwriteNewerBinding() {
	suite.testSourceFreePublicationRace(false)
}

func (suite *VirtualMachineDiskSuite) TestSourceFreeSelectionCASRetriesAfterPublication() {
	suite.testSourceFreePublicationRace(true)
}

func (suite *VirtualMachineDiskSuite) testSourceFreePublicationRace(afterCallback bool) {
	suite.T().Helper()

	path := suite.library()
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)
	firstPath := suite.completeImageCopy("install")
	first := filepath.Base(firstPath)
	suite.recordCompletedBinding(first)
	suite.assertLibraryHeld(false)
	suite.Require().NoError(os.Remove(filepath.Join(path, "talos.iso")))
	writeLibraryImage(suite.T(), path, "talos.iso", "replacement")
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Error = "alias replaced"

		return nil
	})

	var second string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		second = spec.Volume
		asrt.NotEmpty(second)
		asrt.NotEqual(first, second)
	})

	secondID := storage.StoragePoolVolumeID(disk.Pool, second)
	ctest.AssertResource[*storage.StoragePoolVolumeSpec](suite, secondID, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(current.TypedSpec().Copy.Source)
	})
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, storage.StoragePoolVolumeID(disk.Pool, first))
	before, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), secondID)
	suite.Require().NoError(err)

	statusID := hypervisor.VirtualMachineDiskStatusID(vmName, disk)

	pause := &imageCopyPause{
		kind:    storage.StoragePoolVolumeSpecType,
		id:      storage.StoragePoolVolumeID(disk.Pool, first),
		after:   afterCallback,
		volume:  first,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	if afterCallback {
		pause.kind = hypervisor.VirtualMachineDiskStatusType
		pause.id = statusID
	}
	defer pause.resume()

	suite.copyPause.Store(pause)
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})

	select {
	case <-pause.entered:
	case <-time.After(5 * time.Second):
		suite.T().Fatal("source-free reconciliation did not reach the pause")
	}

	// B publishes while the disk controller still holds an A selection (and, in
	// the CAS case, an already-mutated A status based on the old version).
	intent := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, statusID)
	suite.AddFinalizer(intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
	pool, err := safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), disk.Pool)
	suite.Require().NoError(err)

	target := pool.TypedSpec().TargetPath
	suite.Require().NoError(os.WriteFile(filepath.Join(target, second), []byte("replacement"), 0o600))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(second)))
	suite.Require().NoError(os.Link(filepath.Join(target, second), filepath.Join(target, ".copy-on-use", witness)))

	key, _ := before.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	bindings := copyowner.Bindings{}
	suite.Require().NoError(bindings.Record(target, key, second))
	suite.completeImageCopy("install")
	suite.RemoveFinalizer(intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
	pause.resume()

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.Equal(second, spec.Volume)
	})
	after, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), secondID)
	suite.Require().NoError(err)
	suite.Equal(before.TypedSpec().Copy.Source, after.TypedSpec().Copy.Source, "a stale A pass must not withdraw and recreate B")

	bound, err := copyowner.ReadBinding(target, key)
	suite.Require().NoError(err)
	suite.Equal(second, bound)
}

func (suite *VirtualMachineDiskSuite) TestUnavailableBindingDoesNotStrandUnrelatedRemovedDisk() {
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	disk.Pool, disk.Format = "pool1", "raw"
	cache := blankDiskSpec("cache", "pool1", "raw", 1024)
	suite.createVM(disk, cache)
	copied := suite.completeImageCopy("install")
	suite.recordCompletedBinding(filepath.Base(copied))
	suite.assertLibraryHeld(false)
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), storage.StoragePoolVolumeID(disk.Pool, filepath.Base(copied)))
	suite.Require().NoError(err)

	key, _ := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	pool, err := safe.StateGetByID[*storage.StoragePoolStatus](suite.Ctx(), suite.State(), disk.Pool)
	suite.Require().NoError(err)

	record := filepath.Join(pool.TypedSpec().TargetPath, ".copy-on-use", "binding-"+key)
	suite.Require().NoError(os.Remove(record))
	suite.Require().NoError(os.Mkdir(record, 0o700))
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(library *hypervisor.ContentLibraryStatus) error {
		library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	suite.assertDisk("install", func(current hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, current.Phase)
		asrt.Contains(current.Error, "binding")
	})
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(vm *hypervisor.VirtualMachineSpec) error {
		vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}

		return nil
	})
	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, hypervisor.VirtualMachineDiskStatusID(vmName, cache))
}

func (suite *VirtualMachineDiskSuite) TestPendingCopyOutcomeDoesNotRebindSource() {
	suite.testFailedSourceRebind(false, false, "running")
}

func (suite *VirtualMachineDiskSuite) TestStaleCopyFailureDoesNotRebindSource() {
	suite.testFailedSourceRebind(false, false, "stale-failure")
}

func (suite *VirtualMachineDiskSuite) TestMissingCopyOutcomeDoesNotRebindSource() {
	suite.testFailedSourceRebind(false, false, "missing")
}

func (suite *VirtualMachineDiskSuite) TestUnreadableCopyOutcomeDoesNotRebindSource() {
	suite.testFailedSourceRebind(false, false, "unreadable")
}

func (suite *VirtualMachineDiskSuite) TestRemovedDiskRetainsSourceUntilRequestDrain() {
	suite.testRetainsSourceUntilRequestDrain("disk")
}

func (suite *VirtualMachineDiskSuite) TestRemovedVMRetainsSourceUntilRequestDrain() {
	suite.testRetainsSourceUntilRequestDrain("vm")
}

func (suite *VirtualMachineDiskSuite) TestChangedLibraryRetainsOriginalSourceUntilRequestDrain() {
	suite.testRetainsSourceUntilRequestDrain("library")
}

func (suite *VirtualMachineDiskSuite) TestChangedSourceRetainsOriginalSourceUntilRequestDrain() {
	suite.testRetainsSourceUntilRequestDrain("source")
}

func (suite *VirtualMachineDiskSuite) testRetainsSourceUntilRequestDrain(change string) {
	suite.T().Helper()
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)
	suite.assertLibraryHeld(true)

	var id string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
		id = storage.StoragePoolVolumeID(disk.Pool, spec.Volume)
	})

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, id)
	suite.AddFinalizer(request.Metadata(), "test.worker")

	switch change {
	case "vm":
		suite.Destroy(hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName))
	case "library", "source":
		ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(vm *hypervisor.VirtualMachineSpec) error {
			if change == "library" {
				vm.TypedSpec().Disks[0].Provision.FromImage.Library = "new-unconfigured-library"
			} else {
				vm.TypedSpec().Disks[0].Provision.FromImage.File = "new-missing.iso"
			}

			return nil
		})
	default:
		ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(vm *hypervisor.VirtualMachineSpec) error {
			vm.TypedSpec().Disks = nil

			return nil
		})
	}

	ctest.AssertResource(suite, id, func(spec *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, spec.Metadata().Phase())
		asrt.Equal(libraryName, spec.TypedSpec().Copy.SourceLibrary)
	})
	ctx, st := suite.Ctx(), suite.State()
	suite.Require().Never(func() bool {
		library, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, st, libraryName)

		return err != nil || !library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController")
	}, 300*time.Millisecond, 10*time.Millisecond)
	suite.RemoveFinalizer(request.Metadata(), "test.worker")
	ctest.AssertNoResource[*storage.StoragePoolVolumeSpec](suite, id)

	if change != "source" {
		suite.assertLibraryHeld(false)
	}
}

func (suite *VirtualMachineDiskSuite) TestSourceResolutionRetriesWithoutResourceEvent() {
	path := suite.library()
	suite.Require().NoError(os.Rename(path, path+"-offline"))
	suite.T().Cleanup(func() {
		suite.Assert().NoError(os.RemoveAll(path + "-offline"))
	})
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", ""), blankDiskSpec("data", "pool1", "raw", 1024))
	ctest.AssertResource(suite, storage.StoragePoolVolumeID("pool1", "vm__data.raw"), func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(1024), request.TypedSpec().Capacity)
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Contains(spec.Error, "failed to open content library")
	})
	// Let self-generated status/finalizer events settle before restoring only the filesystem.
	time.Sleep(200 * time.Millisecond)
	suite.Require().NoError(os.Rename(path+"-offline", path))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
	})
}

func (suite *VirtualMachineDiskSuite) TestMissingSourceWaitsForInputWithoutPolling() {
	path := suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "missing.iso", ""))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Contains(spec.Error, "no such file")
	})
	time.Sleep(200 * time.Millisecond)
	writeLibraryImage(suite.T(), path, "missing.iso", isoContents)
	ctx, st := suite.Ctx(), suite.State()
	id := suite.diskStatusID("install")
	suite.Require().Never(func() bool {
		status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, st, id)

		return err != nil || status.TypedSpec().Volume != ""
	}, 1300*time.Millisecond, 20*time.Millisecond)
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(library *hypervisor.ContentLibraryStatus) error {
		library.TypedSpec().Error = "wake resolution"

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
	})
}

func (suite *VirtualMachineDiskSuite) TestLegacyPendingRequestDoesNotGuessSourceLibrary() {
	suite.library()

	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")
	suite.createVM(disk)

	var id string

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(spec.Volume)
		id = storage.StoragePoolVolumeID(disk.Pool, spec.Volume)
	})

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, id)
	suite.AddFinalizer(request.Metadata(), "test.worker")
	ctest.UpdateWithConflicts(suite, request, func(current *storage.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy.SourceLibrary = ""

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	// Volume requests only wake the controller once they are ready to be destroyed, so the update
	// above goes unnoticed if the controller has already settled: wake it through the library.
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(library *hypervisor.ContentLibraryStatus) error {
		library.Metadata().Annotations().Set("test.wake", "legacy-request")

		return nil
	})
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Contains(spec.Error, "cannot rebind source library")
	})
	ctx, st := suite.Ctx(), suite.State()
	suite.Require().Never(func() bool {
		current, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](ctx, st, id)
		if err != nil || current.TypedSpec().Copy.SourceLibrary != "" {
			return true
		}

		library, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, st, libraryName)

		return err != nil || !library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController")
	}, 300*time.Millisecond, 10*time.Millisecond)
	// A completed independent copy still permits source-free adoption of that request.
	suite.completeImageCopy("install")
	suite.assertLibraryHeld(false)
}

func (suite *VirtualMachineDiskSuite) TestInvalidSourceWaitsForInputWithoutPolling() {
	path := suite.library()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "invalid.iso"), []byte("not an alias"), 0o600))
	writeLibraryImage(suite.T(), path, "mismatch.iso", "wrong digest")

	tests := []struct {
		name  string
		file  string
		error string
	}{
		{name: "malformed", file: "invalid.iso", error: "not a symlink"},
		{name: "mismatched", file: "mismatch.iso", error: "digest mismatch"},
	}
	disks := make([]hypervisor.VirtualMachineDiskSpec, 0, len(tests))

	for _, test := range tests {
		disks = append(disks, cdromDiskSpec(test.name, libraryName, test.file, digest.FromString(isoContents).String()))
	}

	suite.createVM(disks...)

	diskStatusIDs := make([]string, 0, len(tests))

	for _, test := range tests {
		diskStatusIDs = append(diskStatusIDs, suite.diskStatusID(test.name))
		suite.assertDisk(test.name, func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.Error, test.error)
			asrt.Empty(spec.Volume)
			asrt.Empty(spec.SourcePath)
		})
	}

	suite.assertLibraryHeld(false)
	ctx, st := suite.Ctx(), suite.State()
	rejected := func() bool {
		for _, id := range diskStatusIDs {
			status, err := safe.StateGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, st, id)
			if err != nil || status.TypedSpec().Volume != "" {
				return true
			}
		}

		library, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, st, libraryName)
		if err != nil || library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController") {
			return true
		}

		requests, err := safe.StateListAll[*storage.StoragePoolVolumeSpec](ctx, st)

		return err != nil || requests.Len() != 0
	}

	// Verify settled permanent rejection before repairing only the filesystem.
	suite.Require().Never(rejected, 200*time.Millisecond, 10*time.Millisecond)

	for _, test := range tests {
		suite.Require().NoError(os.Remove(filepath.Join(path, test.file)))
		writeLibraryImage(suite.T(), path, test.file, isoContents)
	}

	suite.Require().Never(rejected, 1300*time.Millisecond, 20*time.Millisecond,
		"permanent rejection without a binding must not poll, request a copy, or acquire a library hold")
}

// Record every successful renderer write, including a transient forced stop that
// eventual-Ready assertions would miss.
type imageDomainPublications struct {
	mu     sync.Mutex
	forced bool
}

type observedImageDomainController struct {
	hypervisorctrl.VirtualMachineDomainSpecController
	publications *imageDomainPublications
}

func (ctrl *observedImageDomainController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	return ctrl.VirtualMachineDomainSpecController.Run(ctx, &observedImageDomainRuntime{Runtime: runtime, publications: ctrl.publications}, logger)
}

type observedImageDomainRuntime struct {
	controller.Runtime
	publications *imageDomainPublications
}

func (runtime *observedImageDomainRuntime) Modify(ctx context.Context, res resource.Resource, modify func(resource.Resource) error, options ...owned.ModifyOption) error {
	var forced bool

	err := runtime.Runtime.Modify(ctx, res, func(current resource.Resource) error {
		if err := modify(current); err != nil {
			return err
		}

		if domain, ok := current.(*hypervisor.VirtualMachineDomainSpec); ok {
			forced = domain.TypedSpec().PowerState == "stopped" && domain.TypedSpec().StopMode == "forced"
		}

		return nil
	}, options...)
	if err == nil && forced {
		runtime.publications.mu.Lock()
		runtime.publications.forced = true
		runtime.publications.mu.Unlock()
	}

	return err
}

func (suite *VirtualMachineDiskSuite) attachedWritableImage() (*imageDomainPublications, *hypervisor.VirtualMachineDomainSpec) {
	suite.T().Helper()

	publications := &imageDomainPublications{}

	suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&observedImageDomainController{publications: publications}))
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 20<<30
	suite.createVM(disk)
	suite.completeImageCopy("system")
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool1/vm__system.img"), func(current *storage.StoragePoolVolumeStatus) error {
		current.TypedSpec().Capacity = 20 << 30

		return nil
	})
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
		asrt.EqualValues(20<<30, spec.Size)
	})
	ctest.AssertResource(suite, vmName, func(domain *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", domain.TypedSpec().PowerState)
	})
	suite.AddFinalizer(hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, suite.diskStatusID("system")).Metadata(), "test.attached")
	suite.AddFinalizer(storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool1/vm__system.img").Metadata(), "test.attached")
	suite.assertLibraryHeld(false)
	suite.Destroy(hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName))
	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)
	suite.Require().NoError(err)
	// Initial provisioning is allowed to be pending. Only observe publications
	// after attachment and removal of its initialization source.
	publications.mu.Lock()
	publications.forced = false
	publications.mu.Unlock()

	return publications, domain
}

func (suite *VirtualMachineDiskSuite) assertImageRemainsRunning(publications *imageDomainPublications, before *hypervisor.VirtualMachineDomainSpec) {
	suite.T().Helper()
	suite.Require().Eventually(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)

		return err == nil && (domain.TypedSpec().PowerState == "stopped" || domain.TypedSpec().ObservedGeneration != before.TypedSpec().ObservedGeneration)
	}, 3*time.Second, time.Millisecond, "renderer must process the edited VM while its dependent controller is delayed")
	domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)
	suite.Require().NoError(err)
	suite.Equal("running", domain.TypedSpec().PowerState, "unchanged writable destination must remain attached")
	suite.Equal(before.TypedSpec().DomainXML, domain.TypedSpec().DomainXML)
	publications.mu.Lock()
	suite.False(publications.forced, "renderer published a forced stop during an initialization-only edit")
	publications.mu.Unlock()
}

func (suite *VirtualMachineDiskSuite) TestWritableImageCapacityEditsPreserveAttachedDomain() {
	publications, before := suite.attachedWritableImage()
	// No storage controller runs in this fixture: its old successful response
	// remains in place until after both edits have reached the renderer.
	for _, capacity := range []uint64{40 << 30, 10 << 30} {
		ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
			current.TypedSpec().Disks[0].Size = capacity

			return nil
		})
		ctest.AssertResource(suite, "pool1/vm__system.img", func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
			asrt.Equal(capacity, request.TypedSpec().Capacity)
		})
		suite.assertImageRemainsRunning(publications, before)
		suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
			asrt.EqualValues(20<<30, spec.Size)
		})

		var err error

		before, err = safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)
		suite.Require().NoError(err)
	}
}

func (suite *VirtualMachineDiskSuite) TestWritableImageSelectorEditsPreserveAttachedDomain() {
	publications, before := suite.attachedWritableImage()

	for _, selector := range []string{"library", "file", "digest"} {
		pause := &imageCopyPause{kind: storage.StoragePoolVolumeSpecType, id: "pool1/vm__system.img", entered: make(chan struct{}), release: make(chan struct{})}

		suite.copyPause.Store(pause)
		defer pause.resume()

		ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
			image := current.TypedSpec().Disks[0].Provision.FromImage

			switch selector {
			case "library":
				image.Library = "removed-library"
			case "file":
				image.File = "another.qcow2"
			case "digest":
				image.Digest = digest.SHA256.FromString("replacement").String()
			}

			suite.disks["system"] = current.TypedSpec().Disks[0]

			return nil
		})

		select {
		case <-pause.entered:
		case <-suite.Ctx().Done():
			suite.T().Fatal("disk controller did not reach barrier")
		}

		suite.assertImageRemainsRunning(publications, before)
		pause.resume()
		suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
			asrt.Equal("vm__system.img", spec.Volume)
		})

		var err error

		before, err = safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), vmName)
		suite.Require().NoError(err)
	}
}

func (suite *VirtualMachineDiskSuite) TestCompletedWritableImageOutageAfterSourceRemovalAndEdits() {
	publications, before := suite.attachedWritableImage()
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks[0].Size *= 2
		current.TypedSpec().Disks[0].Provision.FromImage.File = "no-longer-needed.raw"

		return nil
	})
	ctest.AssertResource(suite, "pool1/vm__system.img", func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.EqualValues(40<<30, request.TypedSpec().Capacity)
	})
	suite.assertImageRemainsRunning(publications, before)
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool1/vm__system.img"), func(current *storage.StoragePoolVolumeStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolVolumePhaseObservationUnavailable

		return nil
	})
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseObservationUnavailable, spec.Phase)
	})
	ctx, st, failures := suite.Ctx(), suite.State(), suite.failures
	suite.Require().Never(func() bool {
		domain, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, st, vmName)

		publications.mu.Lock()
		defer publications.mu.Unlock()

		return err != nil || domain.TypedSpec().PowerState != "running" || domain.TypedSpec().DomainXML != before.TypedSpec().DomainXML || publications.forced || len(failures) != 0
	}, 400*time.Millisecond, 10*time.Millisecond, "completed writable disk must survive loss of observation after source removal")
}

func (suite *VirtualMachineDiskSuite) TestWritableImageAdoptsBeforeSourceObservation() {
	path := suite.library()
	object := filepath.Join(path, "objects", "sha256", digest.SHA256.FromString(isoContents).Encoded())
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.Require().NoError(unix.Close(fd)) })

	_, err = unix.InotifyAddWatch(fd, object, unix.IN_ACCESS|unix.IN_OPEN)
	suite.Require().NoError(err)

	disk := cdromDiskSpec("system", libraryName, "talos.iso", digest.SHA256.FromString(isoContents).String())
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 20<<30
	suite.createVM(disk)
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Contains(spec.Error, "waiting for volume")
	})

	id := "pool1/vm__system.img"
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Empty(request.TypedSpec().Copy.Source)
	})
	suite.publishImageCopy(id) // Storage recovered an existing independent disk.
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, spec.Phase)
	})
	suite.assertLibraryHeld(false)

	var events [4096]byte

	_, err = unix.Read(fd, events[:])
	suite.Require().ErrorIs(err, unix.EAGAIN, "adoption must not open or hash initialization input")
}

// Model storage's failed adoption before supplying initialization input.
func (suite *VirtualMachineDiskSuite) requireWritableImageSource(id string) {
	suite.T().Helper()
	pool := suite.copyPool()
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)

	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)
	*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool: request.TypedSpec().Pool, Name: request.TypedSpec().Name,
		Phase: storage.StoragePoolVolumePhaseNotReady, Error: "se requiere una imagen inicial",
		Copy: &storage.StoragePoolVolumeCopyStatusSpec{
			Phase:  storage.StoragePoolVolumeCopyPhaseSourceRequired,
			Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
		},
	}
	suite.Create(status)
}

func (suite *VirtualMachineDiskSuite) TestFailedWritableImageSelectorAndCapacityEditDrainsBeforeRetry() {
	path := suite.library()
	writeLibraryImage(suite.T(), path, "replacement.raw", "replacement image B")

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)

	id := "pool1/vm__system.img"
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotNil(request.TypedSpec().Copy)
	})
	suite.requireWritableImageSource(id)
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(request.TypedSpec().Copy.Source)
	})
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)

	oldSource := request.TypedSpec().Copy.Source
	suite.AddFinalizer(request.Metadata(), "test.worker")
	pool := suite.copyPool()
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id), func(current *storage.StoragePoolVolumeStatus) error {
		*current.TypedSpec() = *failedSourceCopyStatus(request, pool, "failed").TypedSpec()

		return nil
	})
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks[0].Provision.FromImage.File = "replacement.raw"
		current.TypedSpec().Disks[0].Size = 4096

		return nil
	})
	ctest.AssertResource(suite, id, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
		asrt.Equal(oldSource, current.TypedSpec().Copy.Source)
		asrt.EqualValues(1024, current.TypedSpec().Capacity, "must not admit A at the edited capacity before retirement")
	})
	suite.Destroy(storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id))
	suite.RemoveFinalizer(request.Metadata(), "test.worker")
	ctest.AssertResource(suite, id, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, current.Metadata().Phase())
		asrt.Empty(current.TypedSpec().Copy.Source)
		asrt.EqualValues(4096, current.TypedSpec().Capacity)
	})
	suite.requireWritableImageSource(id)
	ctest.AssertResource(suite, id, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Equal(filepath.Join(path, "objects", "sha256", digest.SHA256.FromString("replacement image B").Encoded()), current.TypedSpec().Copy.Source)
		asrt.EqualValues(4096, current.TypedSpec().Capacity)
	})
}

func (suite *VirtualMachineDiskSuite) TestFailedWritableUnavailablePinnedSourceStillUpdatesCapacity() {
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", digest.SHA256.FromString(isoContents).String())
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)

	id := "pool1/vm__system.img"
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotNil(request.TypedSpec().Copy)
	})
	suite.requireWritableImageSource(id)
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(request.TypedSpec().Copy.Source)
	})
	request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)
	pool := suite.copyPool()
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id), func(current *storage.StoragePoolVolumeStatus) error {
		*current.TypedSpec() = *failedSourceCopyStatus(request, pool, "failed").TypedSpec()

		return nil
	})
	ctest.UpdateWithConflicts(suite, hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName), func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().Phase = hypervisor.ContentLibraryPhaseNotReady

		return nil
	})
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks[0].Size = 4096

		return nil
	})
	ctest.AssertResource(suite, id, func(current *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.EqualValues(4096, current.TypedSpec().Capacity)
	})
}

func (suite *VirtualMachineDiskSuite) TestWritableImageFormatEditExplainsRefusal() {
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)
	suite.completeImageCopy("system")
	ctest.UpdateWithConflicts(suite, hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName), func(current *hypervisor.VirtualMachineSpec) error {
		current.TypedSpec().Disks[0].Format = "qcow2"
		suite.disks["system"] = current.TypedSpec().Disks[0]

		return nil
	})
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
		asrt.Contains(spec.Error, `format changes from "raw" to "qcow2" are not supported`)
	})
}

// A durable publication remains independent when storage later reports Waiting, even
// if its diagnostic status can no longer prove copy completion by itself.
func (suite *VirtualMachineDiskSuite) TestCompletedWritableWaitingReleasesOtherLibraryWithoutSource() {
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)
	suite.completeImageCopy("system")
	suite.recordCompletedBinding("vm__system.img")
	suite.assertLibraryHeld(false)
	suite.Destroy(hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryName))

	other := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "unrelated")
	suite.Create(other)
	suite.AddFinalizer(other.Metadata(), "hypervisor.VirtualMachineDiskController")
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool1/vm__system.img"), func(current *storage.StoragePoolVolumeStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolVolumePhaseNotReady
		current.TypedSpec().Copy.Phase = storage.StoragePoolVolumeCopyPhaseWaiting

		return nil
	})
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseNotReady, spec.Phase)
	})
	ctest.AssertResource(suite, "unrelated", func(current *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController"))
	})
}

func (suite *VirtualMachineDiskSuite) TestCompletedWritableImageOutageDoesNotReholdSource() {
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)
	suite.completeImageCopy("system")
	suite.assertLibraryHeld(false)
	ctest.UpdateWithConflicts(suite, storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "pool1/vm__system.img"), func(current *storage.StoragePoolVolumeStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolVolumePhaseObservationUnavailable

		return nil
	})
	suite.assertDisk("system", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineDiskPhaseObservationUnavailable, spec.Phase)
	})
	ctx, st, failures := suite.Ctx(), suite.State(), suite.failures
	suite.Require().Never(func() bool {
		library, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, st, libraryName)

		return err != nil || library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController") || len(failures) != 0
	}, 400*time.Millisecond, 10*time.Millisecond, "completed initialization must not regain its source dependency")
}

func (suite *VirtualMachineDiskSuite) TestWritableSourceRequiredPhaseIgnoresDiagnostic() {
	suite.library()

	disk := cdromDiskSpec("system", libraryName, "talos.iso", "")
	disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
	suite.createVM(disk)

	id := "pool1/vm__system.img"
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.Empty(request.TypedSpec().Copy.Source)
	})
	suite.requireWritableImageSource(id) // Localized Error; only the enum conveys source admission.
	ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.NotEmpty(request.TypedSpec().Copy.Source)
	})
	suite.assertLibraryHeld(true)
}

func (suite *VirtualMachineDiskSuite) TestWritableMissingStaleAndProbingStatusDoNotHoldSource() {
	suite.library()

	configured := make([]hypervisor.VirtualMachineDiskSpec, 0, 3)

	for _, name := range []string{"missing", "stale", "probing"} {
		disk := cdromDiskSpec(name, libraryName, "talos.iso", digest.SHA256.FromString(isoContents).String())
		disk.Type, disk.Bus, disk.Size = "disk", "virtio", 1024
		configured = append(configured, disk)
	}

	suite.createVM(configured...)
	pool := suite.copyPool()

	for _, name := range []string{"stale", "probing"} {
		id := "pool1/vm__" + name + ".img"
		ctest.AssertResource(suite, id, func(request *storage.StoragePoolVolumeSpec, asrt *assert.Assertions) {
			asrt.Empty(request.TypedSpec().Copy.Source)
		})
		request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
		suite.Require().NoError(err)

		status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, id)

		*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseNotReady, Error: libvirtstorage.ErrCopySourceRequired.Error(),
			Copy: &storage.StoragePoolVolumeCopyStatusSpec{
				Phase:  storage.StoragePoolVolumeCopyPhaseWaiting,
				Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
			},
		}
		if name == "stale" {
			status.TypedSpec().Copy.Phase = storage.StoragePoolVolumeCopyPhaseSourceRequired
			status.TypedSpec().Copy.Intent = "previous-request"
		}

		suite.Create(status)
	}

	ctx, st := suite.Ctx(), suite.State()
	suite.Require().Never(func() bool {
		library, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](ctx, st, libraryName)
		if err != nil || library.Metadata().Finalizers().Has("hypervisor.VirtualMachineDiskController") {
			return true
		}

		for _, disk := range configured {
			request, getErr := safe.StateGetByID[*storage.StoragePoolVolumeSpec](ctx, st, "pool1/vm__"+disk.Name+".img")
			if getErr == nil && request.TypedSpec().Copy.Source != "" {
				return true
			}
		}

		return false
	}, 300*time.Millisecond, 10*time.Millisecond)
}
