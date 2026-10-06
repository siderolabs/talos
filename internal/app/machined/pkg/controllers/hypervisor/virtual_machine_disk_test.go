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
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
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
		Ready:    true,
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
		asrt.True(spec.Ready)
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
		asrt.True(spec.Ready)
		asrt.Empty(spec.Error)
	})
}

func (suite *VirtualMachineDiskSuite) TestRejectsDigestMismatch() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "talos.iso", digest.FromString("something else").String()))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Ready)
		asrt.Contains(spec.Error, "digest mismatch")
		asrt.Empty(spec.SourcePath)
	})
}

func (suite *VirtualMachineDiskSuite) TestReportsMissingFile() {
	suite.library()
	suite.createVM(cdromDiskSpec("install", libraryName, "absent.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Ready)
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
		asrt.False(spec.Ready)
		asrt.Contains(spec.Error, "not a regular file")
	})
}

func (suite *VirtualMachineDiskSuite) TestReportsUnconfiguredLibrary() {
	suite.createVM(cdromDiskSpec("install", "absent", "talos.iso", ""))

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.False(spec.Ready)
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
		asrt.False(spec.Ready)
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
		res.TypedSpec().Ready = true
		res.TypedSpec().Error = ""

		return nil
	})

	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Ready)
		asrt.Equal(filepath.Join(path, "talos.iso"), spec.SourcePath)
	})
}

// A disk this slice does not provision is reported, not silently dropped: the operator sees why
// the virtual machine never gets a domain.
func (suite *VirtualMachineDiskSuite) TestReportsUnsupportedDisks() {
	suite.createVM(
		hypervisor.VirtualMachineDiskSpec{
			Name: "data", Pool: "pool1", Size: 20 << 30, Format: "qcow2", Bus: "virtio", Type: "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
		},
		hypervisor.VirtualMachineDiskSpec{
			Name: "system", Pool: "pool1", Size: 20 << 30, Format: "qcow2", Bus: "virtio", Type: "disk",
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.qcow2"},
			},
		},
	)

	for _, disk := range []string{"data", "system"} {
		suite.assertDisk(disk, func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
			asrt.False(spec.Ready)
			asrt.Contains(spec.Error, "only cdrom disks are provisioned today")
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
			asrt.True(spec.Ready)
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
		asrt.True(spec.Ready)
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
		asrt.True(spec.Ready)
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
		asrt.True(spec.Ready)
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
		asrt.False(spec.Ready)
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
		asrt.True(spec.Ready)
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
		asrt.True(res.TypedSpec().Ready)
	})
}
