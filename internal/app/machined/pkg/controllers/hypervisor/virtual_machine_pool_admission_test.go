// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// A Ready volume is published only after its controller commits a pool-status
// hold. Preserve that invariant in the snapshot while exercising VM admission
// against a retarget marker published after the volume became Ready.
func TestVMStartRejectsPoolRetargetExclusion(t *testing.T) {
	t.Parallel()

	s := &VirtualMachineDomainSuite{}
	s.Timeout = 5 * time.Second
	s.SetT(t)
	s.SetupTest()

	defer s.TearDownTest()

	path := filepath.Join(t.TempDir(), "vm__system.raw")
	s.Require().NoError(os.WriteFile(path, make([]byte, 1<<20), 0o600))

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "vms")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Create(pool)
	s.AddFinalizer(pool.Metadata(), "storage.StoragePoolVolumeController")

	volume := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, "vms/vm__system.raw")
	*volume.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:     "vms",
		Name:     "vm__system.raw",
		Path:     path,
		Format:   "raw",
		Capacity: 1 << 20,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
	s.Create(volume)

	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "vm/system")
	*disk.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: "vm",
		Name:           "system",
		Blank:          true,
		Pool:           "vms",
		Volume:         "vm__system.raw",
		SourcePath:     path,
		Format:         "raw",
		Size:           1 << 20,
		Phase:          hypervisor.VirtualMachineDiskPhaseReady,
	}
	s.Create(disk)

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "vm")
	domain.TypedSpec().PowerState = "running"
	domain.TypedSpec().DomainXML = `<domain><name>vm</name><vcpu>1</vcpu></domain>`
	domain.TypedSpec().Disks = []string{disk.Metadata().ID()}
	s.Create(domain)

	s.AddFinalizer(pool.Metadata(), "storage.StoragePoolController/mutating/backing")
	s.start()
	ctest.AssertResource(s, volume.Metadata().ID(), func(current *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has("hypervisor.VirtualMachineController/" + disk.Metadata().ID()))
	})
	client := s.client
	s.Require().Never(func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.starts["vm"] > 0
	}, 200*time.Millisecond, 10*time.Millisecond)
	s.RemoveFinalizer(pool.Metadata(), "storage.StoragePoolController/mutating/backing")
	current, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "vm")
	s.Require().NoError(err)
	s.assertDomain("vm", current.TypedSpec().DomainXML, true)
}
