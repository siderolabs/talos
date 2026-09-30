// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func (suite *VirtualMachineDiskSuite) assertDisk(disk string, check func(hypervisor.VirtualMachineDiskStatusSpec, *assert.Assertions)) {
	suite.T().Helper()

	ctest.AssertResource(suite, hypervisor.VirtualMachineDiskStatusID(vmName, disk),
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

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, hypervisor.VirtualMachineDiskStatusID(vmName, "rescue"))
	suite.assertDisk("install", func(spec hypervisor.VirtualMachineDiskStatusSpec, asrt *assert.Assertions) {
		asrt.True(spec.Ready)
	})

	suite.Destroy(spec)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](suite, hypervisor.VirtualMachineDiskStatusID(vmName, "install"))
}
