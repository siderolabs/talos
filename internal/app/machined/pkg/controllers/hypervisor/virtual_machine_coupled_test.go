// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/utils/cpuset"

	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// memCgroupFS is a minimal in-memory cgroup v2 tree for the coupled tests.
type memCgroupFS struct {
	mu     sync.Mutex
	cpus   map[string]string
	writes []string
	// onlineHook is called (unlocked) at the start of every coordinator observation.
	onlineHook func()
	// writeHook is called (unlocked) before every cpuset write.
	writeHook func(path, cpus string)
}

func newMemCgroupFS() *memCgroupFS {
	fs := &memCgroupFS{cpus: map[string]string{}}

	for _, root := range []string{"init", "system", "podruntime", "kubepods", "taloscontainers", "virtualmachines.partition"} {
		fs.cpus[root] = ""
	}

	return fs
}

func (fs *memCgroupFS) Online() (cpuset.CPUSet, error) {
	fs.mu.Lock()
	hook := fs.onlineHook
	fs.mu.Unlock()

	if hook != nil {
		hook()
	}

	return cpuset.Parse("0-7")
}

func (fs *memCgroupFS) Exists(path string) (bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	_, ok := fs.cpus[path]

	return ok, nil
}

func (fs *memCgroupFS) Ensure(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.cpus[path]; !ok {
		fs.cpus[path] = ""
	}

	return nil
}

func (fs *memCgroupFS) Remove(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.cpus, path)

	return nil
}

func (fs *memCgroupFS) CPUs(path string) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	value, ok := fs.cpus[path]
	if !ok {
		return "", errors.New("no such cgroup")
	}

	return value, nil
}

func (fs *memCgroupFS) Effective(path string) (cpuset.CPUSet, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.effectiveLocked(path)
}

func (fs *memCgroupFS) effectiveLocked(path string) (cpuset.CPUSet, error) {
	parent, err := cpuset.Parse("0-7")
	if err != nil {
		return cpuset.New(), err
	}

	if dir, _, ok := strings.CutLast(path, "/"); ok {
		if parent, err = fs.effectiveLocked(dir); err != nil {
			return cpuset.New(), err
		}
	}

	configured := fs.cpus[path]
	if configured == "" {
		return parent, nil
	}

	set, err := cpuset.Parse(configured)
	if err != nil {
		return cpuset.New(), err
	}

	if intersection := set.Intersection(parent); !intersection.IsEmpty() {
		return intersection, nil
	}

	return parent, nil
}

func (fs *memCgroupFS) SetCPUs(path, cpus string) error {
	fs.mu.Lock()
	hook := fs.writeHook
	fs.mu.Unlock()

	if hook != nil {
		hook(path, cpus)
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// cgroup v2 rejects emptying the mask of a cpuset with tasks (ENOSPC, observed live).
	if cpus == "" && !strings.Contains(path, "/") {
		return errors.New("write cpuset.cpus: no space left on device")
	}

	fs.cpus[path] = cpus
	fs.writes = append(fs.writes, path+"="+cpus)

	return nil
}

func (fs *memCgroupFS) Populated(path string) (bool, error) {
	// Fixed roots always hold tasks on a real node; slices only when a fake domain sits in them.
	return !strings.Contains(path, "/"), nil
}

func (fs *memCgroupFS) LeafEffective(string) (map[string]cpuset.CPUSet, error) {
	return map[string]cpuset.CPUSet{}, nil
}

func (fs *memCgroupFS) recordedWrites() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return append([]string(nil), fs.writes...)
}

// rendezvous parks the caller: it signals parked and waits for resume.
func rendezvous(parked chan<- struct{}, resume <-chan struct{}) {
	parked <- struct{}{}

	<-resume
}

// coupledSuite runs the projection, coordinator, renderer, domain runtime and a fake libvirt
// together inside a synctest bubble.
type coupledSuite struct {
	VirtualMachineDomainSuite
	fs *memCgroupFS
}

func newCoupledSuite(t *testing.T) *coupledSuite {
	s := &coupledSuite{fs: newMemCgroupFS()}
	s.SetT(t)
	s.SetupTest()

	// The projection publishes the spec itself; the fixture's pre-created disabled spec goes.
	existing, err := safe.StateGetByID[*runtimeres.CPUPartitionSpec](s.Ctx(), s.State(), runtimeres.CPUPartitionSpecID)
	s.Require().NoError(err)
	s.Destroy(existing)

	s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{V1Alpha1Mode: machineruntime.ModeMetal}))
	s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionController{
		V1Alpha1Mode: machineruntime.ModeMetal, FS: s.fs, PollInterval: time.Second, BarrierTimeout: time.Minute,
	}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineSpecController{}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
	s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineController{Open: s.client.open}))

	return s
}

func acceptedPartition() *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}
	doc.TalosContainersConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs:     "4-7",
		SlicesConfig: []runtimecfg.CPUPartitionSlice{{SliceName: "database", SliceCPUs: "4-5", SliceExclusive: new(true)}},
	}

	return doc
}

func (s *coupledSuite) setConfig(vm *hypervisorcfg.VirtualMachineConfigV1Alpha1, partition *runtimecfg.CPUPartitionConfigV1Alpha1) {
	var cfg *container.Container

	var err error

	if partition == nil {
		cfg, err = container.New(vm)
	} else {
		cfg, err = container.New(vm, partition)
	}

	s.Require().NoError(err)

	res := config.NewMachineConfig(cfg)

	existing, getErr := safe.StateGetByID[*config.MachineConfig](s.Ctx(), s.State(), config.ActiveID)
	if getErr != nil {
		s.Create(res)

		return
	}

	res.Metadata().SetVersion(existing.Metadata().Version())
	s.Update(res)
}

func (s *coupledSuite) partitionStatus() *runtimeres.CPUPartitionStatusSpec {
	status, err := safe.StateGetByID[*runtimeres.CPUPartitionStatus](s.Ctx(), s.State(), runtimeres.CPUPartitionStatusID)
	if err != nil {
		return nil
	}

	return status.TypedSpec()
}

// settle lets the runtime retry through its restart backoff until the condition holds; fake
// time advances only while every goroutine is blocked, so no real time passes.
func (s *coupledSuite) settle(condition func() bool) {
	s.T().Helper()

	if !s.settled(condition) {
		s.FailNow("condition did not settle")
	}
}

func (s *coupledSuite) settled(condition func() bool) bool {
	for range 60 {
		synctest.Wait()

		if condition() {
			return true
		}

		synctest.Sleep(time.Second)
	}

	return false
}

func (s *coupledSuite) domainCount() int {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	return len(s.client.domains)
}

func (s *coupledSuite) domainText(name string) string {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	return s.client.texts[name]
}

// Policy introduction over a running, unplaced root machine, through the real controllers: the
// transition is blocked, no cgroup is written, no placement granted, the definition keeps the
// root partition (no restart), and the operator's stop + actual removal unblocks it.
func TestCoupledIntroductionOverRunningRootVM(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		vm := newVirtualMachine("legacy")
		vm.CPUConfig.CPUSlice = "database"
		vm.CPUConfig.CPUCount = 2

		s.setConfig(vm, nil)
		synctest.Wait()

		// The projection published a disabled policy, so the machine started in the root.
		xml := s.domainText("legacy")
		s.Require().Contains(xml, "<partition>/virtualmachines.partition</partition>")
		s.Require().Equal(1, s.starts("legacy"))
		s.Require().Nil(s.partitionStatus())

		s.setConfig(vm, acceptedPartition())
		synctest.Wait()

		status := s.partitionStatus()
		s.Require().NotNil(status)
		s.Require().Equal(runtimeres.CPUPartitionPhaseBlocked, status.Phase)
		s.Require().NotEmpty(status.Blocked)
		s.Assert().Equal([]string{"legacy"}, status.Blocked[0].VirtualMachines)
		s.Assert().Empty(s.fs.recordedWrites(), "no cgroup is written under a running root machine")
		s.Assert().Empty(status.Targets)
		s.Assert().Equal(xml, s.domainText("legacy"), "the running definition is unchanged")
		s.Assert().Equal(1, s.starts("legacy"), "no restart")

		_, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), "legacy")
		s.Assert().Error(err, "no placement is granted to a running root machine")

		select {
		case <-s.client.attemptedRemove:
			s.FailNow("a domain was removed to apply a policy")
		default:
		}

		// The operator stops the machine: real removal, claims released, then the policy applies
		// and, once started again, the machine runs in its slice.
		vm.PowerStateConfig = hypervisorhelpers.PowerStateStopped
		s.setConfig(vm, acceptedPartition())
		s.settle(func() bool {
			return s.domainCount() == 0 && s.partitionStatus().Phase == runtimeres.CPUPartitionPhaseReady
		})

		s.Require().Equal(runtimeres.CPUPartitionPhaseReady, s.partitionStatus().Phase, s.partitionStatus().Error)
		s.Assert().Contains(s.fs.recordedWrites(), "virtualmachines.partition/database.partition=4-5")

		vm.PowerStateConfig = hypervisorhelpers.PowerStateRunning
		s.setConfig(vm, acceptedPartition())
		s.settle(func() bool { return s.starts("legacy") == 2 })

		s.Require().Equal(2, s.starts("legacy"))
		s.Assert().Contains(s.domainText("legacy"), "<partition>/virtualmachines.partition/database.partition</partition>")

		placement, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), "legacy")
		s.Require().NoError(err)
		s.Assert().True(placement.Metadata().Finalizers().Has(runtimeFinalizer))
		s.Assert().Equal(resource.PhaseRunning, placement.Metadata().Phase())

		reservation, err := safe.StateGetByID[*k8s.KubeletCPUReservation](s.Ctx(), s.State(), k8s.KubeletID)
		s.Require().NoError(err)
		s.Assert().True(reservation.TypedSpec().Managed)
	})
}

// Finding 5: a start racing the coordinator. The coordinator closes admission, snapshots occupancy,
// then writes. A start whose claim lands before the close is in the re-snapshot and blocks the plan;
// a start attempted after the close reads the closed phase, keeps its claims (evidence), and does
// not start until admission reopens. Both boundaries are exercised by driving the real runtime from
// the coordinator's own observation and write hooks.
func TestCoupledStartRacesCoordinatorClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		web := newVirtualMachine("web")
		db := newVirtualMachine("db")
		db.CPUConfig.CPUSlice = "database"
		db.PowerStateConfig = hypervisorhelpers.PowerStateStopped

		s.setConfigs(acceptedPartition(), web, db)
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && status.Phase == runtimeres.CPUPartitionPhaseReady && s.starts("web") == 1
		})

		// db is stopped and released: it holds a placement but no claim, so a move of the database
		// slice is plannable. The plan: shared shrinks to 7, database moves to 6, shared grows.
		swap := acceptedPartition()
		swap.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "6"

		// Boundary A: the runtime starts db (claims, then Start) during the coordinator's first
		// observation, before the close. The hook parks the coordinator until the test goroutine
		// has driven the runtime; the re-observation after the close must see the claim.
		parked, resume := make(chan struct{}), make(chan struct{})

		var once sync.Once

		s.fs.mu.Lock()
		s.fs.onlineHook = func() {
			once.Do(func() { rendezvous(parked, resume) })
		}
		s.fs.mu.Unlock()

		s.setConfigs(swap, web, db)
		<-parked

		db.PowerStateConfig = hypervisorhelpers.PowerStateRunning
		s.setConfigs(swap, web, db)
		s.settle(func() bool { return s.starts("db") == 1 })
		close(resume)

		s.settle(func() bool { return s.partitionStatus().Phase == runtimeres.CPUPartitionPhaseBlocked })

		// The claim was seen: database is occupied, its disjoint move is blocked, nothing written.
		status := s.partitionStatus()
		s.Assert().NotContains(s.fs.recordedWrites(), "virtualmachines.partition/database.partition=6")
		s.Assert().NotContains(s.fs.recordedWrites(), "virtualmachines.partition/shared.partition=7")
		s.Assert().Equal("4-5", appliedOf(status, "virtualMachines/database"))
		s.Assert().Equal(1, s.starts("db"))
		s.Assert().Contains(s.domainText("db"), "database.partition")

		s.fs.mu.Lock()
		s.fs.onlineHook = nil
		s.fs.mu.Unlock()

		// Boundary B: the swap's first write happens with admission closed; the runtime tries to
		// start db from that moment: it claims, reads the closed phase, and does not start.
		parkedB, resumeB := make(chan struct{}), make(chan struct{})

		var onceB sync.Once

		s.fs.mu.Lock()
		s.fs.writeHook = func(string, string) {
			onceB.Do(func() { rendezvous(parkedB, resumeB) })
		}
		s.fs.mu.Unlock()

		// The operator stops db (real removal, claims released): the swap proceeds and parks.
		db.PowerStateConfig = hypervisorhelpers.PowerStateStopped
		s.setConfigs(swap, web, db)
		<-parkedB

		s.Require().Equal(1, s.domainCount())
		s.Require().Equal(runtimeres.CPUPartitionPhaseApplying, s.partitionStatus().Phase, "writes happen only with admission closed")

		db.PowerStateConfig = hypervisorhelpers.PowerStateRunning
		s.setConfigs(swap, web, db)
		synctest.Sleep(5 * time.Second)
		synctest.Wait()

		s.Assert().Equal(1, s.starts("db"), "a start attempted while admission is closed must wait")

		placement, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), "db")
		s.Require().NoError(err)
		s.Assert().False(placement.Metadata().Finalizers().Has(runtimeFinalizer), "a refused start leaves no claim the coordinator could plan around")
		close(resumeB)

		s.settle(func() bool {
			return s.partitionStatus().Phase == runtimeres.CPUPartitionPhaseReady && s.starts("db") == 2
		})

		s.Assert().Equal("6", appliedOf(s.partitionStatus(), "virtualMachines/database"))
		s.Assert().Contains(s.domainText("db"), "database.partition")
		s.Assert().Equal(1, s.starts("web"), "the running machine was never restarted")
	})
}

func appliedOf(status *runtimeres.CPUPartitionStatusSpec, key string) string {
	target, _ := status.Target(key)

	return target.LastApplied
}

func (s *coupledSuite) setConfigs(partition *runtimecfg.CPUPartitionConfigV1Alpha1, vms ...*hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	docs := make([]configcfg.Document, 0, len(vms)+1)

	if partition != nil {
		docs = append(docs, partition)
	}

	for _, vm := range vms {
		docs = append(docs, vm)
	}

	cfg, err := container.New(docs...)
	s.Require().NoError(err)

	res := config.NewMachineConfig(cfg)

	existing, getErr := safe.StateGetByID[*config.MachineConfig](s.Ctx(), s.State(), config.ActiveID)
	if getErr != nil {
		s.Create(res)

		return
	}

	res.Metadata().SetVersion(existing.Metadata().Version())
	s.Update(res)
}

// Finding 5 (root side): the runtime read an open, non-placing policy and claimed the domain
// spec; the policy starts placing machines before the Start completes, and the Start fails
// without creating a domain. The stale domain claim must not survive: the coordinator would
// otherwise plan around a machine that never ran in the root. The window is opened by parking
// the Start and failing it.
func TestCoupledRootStartRacesPolicyIntroduction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		s.setConfig(newVirtualMachine("bystander"), nil)
		s.settle(func() bool { return s.starts("bystander") == 1 })

		vm := newVirtualMachine("late")
		vm.CPUConfig.CPUSlice = "database"
		vm.CPUConfig.CPUCount = 2

		parked, resume := make(chan struct{}), make(chan struct{})

		var once sync.Once

		s.client.mu.Lock()
		s.client.beforeStart = func(name string) error {
			if name != "late" {
				return nil
			}

			failed := false

			once.Do(func() {
				rendezvous(parked, resume)

				failed = true
			})

			if failed {
				return errors.New("virtqemud went away")
			}

			return nil
		}
		s.client.mu.Unlock()

		s.setConfigs(nil, newVirtualMachine("bystander"), vm)
		<-parked

		lateSpec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "late")
		s.Require().NoError(err)
		s.Require().True(lateSpec.Metadata().Finalizers().Has(runtimeFinalizer), "the domain claim precedes the start")

		s.setConfigs(acceptedPartition(), newVirtualMachine("bystander"), vm)
		close(resume)

		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && status.Phase == runtimeres.CPUPartitionPhaseBlocked
		})

		s.Assert().Empty(s.fs.recordedWrites())
		s.Assert().Empty(s.domainText("late"), "the failed start created no domain")

		// Only the machine really running in the root blocks the introduction; the stale claim
		// of the failed root start has been released.
		s.settle(func() bool {
			spec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), "late")

			return err == nil && !spec.Metadata().Finalizers().Has(runtimeFinalizer)
		})
		s.Assert().Equal([]string{"bystander"}, s.partitionStatus().Blocked[0].VirtualMachines)
	})
}

// crashDomain removes a fake domain the way a guest crash does: libvirt no longer lists it, but
// the runtime keeps every claim it holds (a failed or unknown outcome never drops them).
func (s *coupledSuite) crashDomain(name string) {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	delete(s.client.domains, name)
	delete(s.client.texts, name)
}

// foreignWriteDatabase changes the database slice mask outside Talos, as an operator or another
// agent would.
func (s *coupledSuite) foreignWriteDatabase(cpus string) {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()

	s.fs.cpus["virtualmachines.partition/database.partition"] = cpus
}

// wakeCoordinator makes the coordinator re-observe (a CPU core resource is a wake-up input).
func (s *coupledSuite) wakeCoordinator(id string) {
	s.Create(hardware.NewCPUCore(id))
}

func (s *coupledSuite) domainClaimed(name string) bool {
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](s.Ctx(), s.State(), name)

	return err == nil && spec.Metadata().Finalizers().Has(runtimeFinalizer)
}

// A held placement claim is evidence of possible occupancy, not an authorisation to start a
// replacement domain. An admitted machine whose domain is gone (guest crash, or a Start whose
// outcome was unknown) keeps its claims, and must not be started again while a boundary has lost
// enforcement or admission is closed; the other machine, really running, is left alone. Once the
// loss is cleared and observed, the restart is admitted.
func TestCoupledHeldClaimDoesNotBypassAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		web := newVirtualMachine("web")
		db := newVirtualMachine("db")
		db.CPUConfig.CPUSlice = "database"

		s.setConfigs(acceptedPartition(), web, db)
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && status.Phase == runtimeres.CPUPartitionPhaseReady && s.starts("web") == 1 && s.starts("db") == 1
		})

		s.Require().True(s.placementHeld("db"))
		s.Require().True(s.domainClaimed("db"))

		// The db guest crashes: libvirt no longer lists it; the claims stay (the runtime cannot
		// tell a crash from a start whose QEMU is still coming up).
		s.crashDomain("db")
		s.Require().Equal(1, s.domainCount())

		// Before the runtime looks again, the database boundary is changed outside Talos and the
		// coordinator observes the loss.
		s.foreignWriteDatabase("5")
		s.wakeCoordinator("cpu0")
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && len(status.EnforcementLoss) > 0
		})

		s.Require().Equal([]string{"virtualMachines/database"}, s.partitionStatus().EnforcementLoss)

		// The runtime's turn: a domain spec change (any input) drives a reconcile with the claims
		// held and the domain absent.
		s.wakeRuntimeDB()
		synctest.Sleep(5 * time.Second)
		synctest.Wait()

		s.Assert().Equal(1, s.starts("db"), "a held claim must not start a replacement domain while enforcement is lost")
		s.Assert().Equal(1, s.domainCount(), "no new domain while enforcement is lost")
		s.Assert().Equal(1, s.starts("web"), "the running machine is left alone")
		s.Assert().Equal("web", s.onlyDomain(), "web keeps running")
		s.Assert().True(s.placementHeld("db"), "the placement claim is kept: the outcome of the previous start is unknown to the runtime")
		// The bare domain claim is released under a placing policy (the placement claim carries the
		// occupancy the coordinator plans around); it is retaken by the next admitted start.

		// The operator restores the mask; the next observation clears the loss and the restart is
		// admitted through the ordinary path.
		s.foreignWriteDatabase("4-5")
		s.wakeCoordinator("cpu1")
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && len(status.EnforcementLoss) == 0 && status.Phase == runtimeres.CPUPartitionPhaseReady
		})

		s.wakeRuntimeDB()
		s.settle(func() bool { return s.starts("db") == 2 })

		s.Assert().Equal(2, s.domainCount())
		s.Assert().Equal(1, s.starts("web"))
		s.Assert().Contains(s.domainText("db"), "database.partition")
	})
}

// The same held/absent case against a closed phase: a coordinator write in flight closes admission,
// and the restart of a crashed, still-claimed machine must wait for it like any other new start.
func TestCoupledHeldClaimWaitsForClosedAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		web := newVirtualMachine("web")
		db := newVirtualMachine("db")
		db.CPUConfig.CPUSlice = "database"

		s.setConfigs(acceptedPartition(), web, db)
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && status.Phase == runtimeres.CPUPartitionPhaseReady && s.starts("web") == 1 && s.starts("db") == 1
		})

		s.crashDomain("db")

		// A live, safe change (database shrinks to 4) parks at its first write: admission is closed.
		parked, resume := make(chan struct{}), make(chan struct{})

		var once sync.Once

		s.fs.mu.Lock()
		s.fs.writeHook = func(string, string) {
			once.Do(func() { rendezvous(parked, resume) })
		}
		s.fs.mu.Unlock()

		shrink := acceptedPartition()
		shrink.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "4"
		s.setConfigs(shrink, web, db)
		<-parked

		s.Require().Equal(runtimeres.CPUPartitionPhaseApplying, s.partitionStatus().Phase)

		s.wakeRuntimeDB()
		synctest.Sleep(5 * time.Second)
		synctest.Wait()

		s.Assert().Equal(1, s.starts("db"), "a held claim must not start a replacement domain while admission is closed")
		s.Assert().Equal(1, s.domainCount())
		s.Assert().True(s.placementHeld("db"), "the claims are kept while the outcome is unknown")

		close(resume)

		s.settle(func() bool {
			return s.partitionStatus().Phase == runtimeres.CPUPartitionPhaseReady && s.starts("db") == 2
		})
		s.Assert().Equal(1, s.starts("web"), "the running machine was never restarted")
	})
}

// A Start which failed keeps its claims (no proof that no QEMU was created); a loss observed
// afterwards must stop the retry, and the retry resumes when the loss is cleared.
func TestCoupledFailedStartRetainsClaimsAndObeysLoss(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCoupledSuite(t)
		defer s.TearDownTest()

		web := newVirtualMachine("web")
		db := newVirtualMachine("db")
		db.CPUConfig.CPUSlice = "database"
		db.PowerStateConfig = hypervisorhelpers.PowerStateStopped

		s.setConfigs(acceptedPartition(), web, db)
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && status.Phase == runtimeres.CPUPartitionPhaseReady && s.starts("web") == 1
		})

		var attempts atomic.Int32

		s.client.mu.Lock()
		s.client.beforeStart = func(name string) error {
			if name == "db" {
				attempts.Add(1)

				return errors.New("virtqemud: unknown outcome")
			}

			return nil
		}
		s.client.mu.Unlock()

		db.PowerStateConfig = hypervisorhelpers.PowerStateRunning
		s.setConfigs(acceptedPartition(), web, db)
		s.settle(func() bool { return attempts.Load() >= 1 && s.placementHeld("db") && s.domainClaimed("db") })

		// Loss observed; the retry loop must stop attempting Start.
		s.foreignWriteDatabase("5")
		s.wakeCoordinator("cpu0")
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && len(status.EnforcementLoss) > 0
		})

		before := attempts.Load()

		s.wakeRuntimeDB()
		synctest.Sleep(10 * time.Second)
		synctest.Wait()

		s.Assert().Equal(before, attempts.Load(), "no Start retry while enforcement is lost")
		s.Assert().True(s.placementHeld("db"), "the claims are kept across the failed start")
		s.Assert().Equal(1, s.starts("web"))

		// Loss cleared and the daemon recovered: the retry goes through.
		s.client.mu.Lock()
		s.client.beforeStart = nil
		s.client.mu.Unlock()

		s.foreignWriteDatabase("4-5")
		s.wakeCoordinator("cpu1")
		s.settle(func() bool {
			status := s.partitionStatus()

			return status != nil && len(status.EnforcementLoss) == 0
		})

		s.wakeRuntimeDB()
		s.settle(func() bool { return s.starts("db") == 1 })
		s.Assert().Equal(2, s.domainCount())
	})
}

// wakeRuntimeDB nudges the domain runtime through a strong input it already watches: a no-op
// bump of db's placement resource.
func (s *coupledSuite) wakeRuntimeDB() {
	placement, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), "db")
	s.Require().NoError(err)

	s.Require().NoError(s.State().Update(s.Ctx(), placement, state.WithUpdateOwner(placement.Metadata().Owner())))
}

func (s *coupledSuite) onlyDomain() string {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	for name := range s.client.domains {
		return name
	}

	return ""
}
