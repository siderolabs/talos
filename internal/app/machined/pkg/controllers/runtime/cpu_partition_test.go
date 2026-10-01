// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// fakeCgroupFS is an in-memory cgroup v2 tree: every cgroup has a configured mask, its effective
// mask is the intersection with its parent's effective mask (falling back to the parent's), and
// the test decides which cgroups are populated and what the kubepods leaves see.
type fakeCgroupFS struct {
	mu        sync.Mutex
	online    cpuset.CPUSet
	cpus      map[string]string
	populated map[string]bool
	leaves    map[string]cpuset.CPUSet
	writes    []string
	writeErr  map[string]error
	// beforeWrite is called before every SetCPUs with the path and value, to sample state.
	beforeWrite func(path, cpus string)
}

func mustCPUs(list string) cpuset.CPUSet {
	set, err := cpuset.Parse(list)
	if err != nil {
		panic(err)
	}

	return set
}

func newFakeCgroupFS() *fakeCgroupFS {
	fs := &fakeCgroupFS{
		online:    mustCPUs("0-7"),
		cpus:      map[string]string{},
		populated: map[string]bool{},
		leaves:    map[string]cpuset.CPUSet{},
		writeErr:  map[string]error{},
	}

	for _, root := range []string{"init", "system", "podruntime", "kubepods", "taloscontainers", "virtualmachines.partition"} {
		fs.cpus[root] = ""
		// Every fixed root but taloscontainers has tasks on a real node.
		fs.populated[root] = root != "taloscontainers"
	}

	return fs
}

func (fs *fakeCgroupFS) Online() (cpuset.CPUSet, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.online, nil
}

func (fs *fakeCgroupFS) Exists(path string) (bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	_, ok := fs.cpus[path]

	return ok, nil
}

func (fs *fakeCgroupFS) Ensure(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.cpus[path]; !ok {
		fs.cpus[path] = ""
	}

	return nil
}

func (fs *fakeCgroupFS) Remove(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.populated[path] {
		return errors.New("cgroup is populated")
	}

	delete(fs.cpus, path)
	fs.writes = append(fs.writes, "remove "+path)

	return nil
}

func (fs *fakeCgroupFS) CPUs(path string) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	value, ok := fs.cpus[path]
	if !ok {
		return "", errors.New("no such cgroup")
	}

	return value, nil
}

func (fs *fakeCgroupFS) effectiveLocked(path string) cpuset.CPUSet {
	parent := fs.online

	if dir, _, ok := strings.CutLast(path, "/"); ok {
		parent = fs.effectiveLocked(dir)
	}

	configured := fs.cpus[path]
	if configured == "" {
		return parent
	}

	set := mustCPUs(configured)

	if intersection := set.Intersection(parent); !intersection.IsEmpty() {
		return intersection
	}

	return parent
}

func (fs *fakeCgroupFS) Effective(path string) (cpuset.CPUSet, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.cpus[path]; !ok && path != "" {
		return cpuset.New(), errors.New("no such cgroup")
	}

	return fs.effectiveLocked(path), nil
}

func (fs *fakeCgroupFS) SetCPUs(path, cpus string) error {
	fs.mu.Lock()
	hook := fs.beforeWrite
	fs.mu.Unlock()

	if hook != nil {
		hook(path, cpus)
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.writeErr[path]; err != nil {
		return err
	}

	if _, ok := fs.cpus[path]; !ok {
		return errors.New("no such cgroup")
	}

	// cgroup v2 rejects emptying the mask of a cpuset with tasks (ENOSPC, observed live).
	if cpus == "" && fs.populated[path] {
		return errors.New("write cpuset.cpus: no space left on device")
	}

	fs.cpus[path] = cpus
	fs.writes = append(fs.writes, fmt.Sprintf("%s=%s", path, cpus))

	return nil
}

func (fs *fakeCgroupFS) Populated(path string) (bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.populated[path], nil
}

func (fs *fakeCgroupFS) LeafEffective(string) (map[string]cpuset.CPUSet, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return maps.Clone(fs.leaves), nil
}

func (fs *fakeCgroupFS) recordedWrites() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return slices.Clone(fs.writes)
}

func (fs *fakeCgroupFS) set(path, cpus string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.cpus[path] = cpus
}

func (fs *fakeCgroupFS) setPopulated(path string, populated bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.populated[path] = populated
}

// setKubepodsWriteError makes writes to the kubepods cgroup fail with err (nil clears it).
func (fs *fakeCgroupFS) setKubepodsWriteError(err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err == nil {
		delete(fs.writeErr, "kubepods")

		return
	}

	fs.writeErr["kubepods"] = err
}

func (fs *fakeCgroupFS) setLeaf(name string, cpus string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if cpus == "" {
		delete(fs.leaves, name)

		return
	}

	fs.leaves[name] = mustCPUs(cpus)
}

// CPUPartitionSuite drives the coordinator inside a synctest bubble.
type CPUPartitionSuite struct {
	ctest.DefaultSuite
	fs *fakeCgroupFS
}

func newCPUPartitionSuite(t *testing.T) *CPUPartitionSuite {
	s := &CPUPartitionSuite{Timeout: time.Minute, fs: newFakeCgroupFS()}
	s.SetT(t)
	s.SetupTest()

	return s
}

func (s *CPUPartitionSuite) start() {
	s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionController{
		V1Alpha1Mode: machineruntime.ModeMetal,
		FS:           s.fs,
		PollInterval: time.Second,
	}))
}

func acceptedPolicy() *runtime.CPUPartitionSpec {
	spec := runtime.NewCPUPartitionSpec()
	spec.TypedSpec().Enabled = true
	spec.TypedSpec().Roots = map[string]string{
		"init": "0-1", "system": "0-1", "podruntime": "0-1", "kubepods": "2-3", "taloscontainers": "0-1", "virtualMachines": "4-7",
	}
	spec.TypedSpec().Slices = []runtime.CPUPartitionSliceSpec{{Name: "database", CPUs: "4-5", Exclusive: true}}

	return spec
}

func (s *CPUPartitionSuite) publish(spec *runtime.CPUPartitionSpec) {
	existing, err := safe.StateGetByID[*runtime.CPUPartitionSpec](s.Ctx(), s.State(), runtime.CPUPartitionSpecID)
	if err != nil {
		s.Create(spec)

		return
	}

	spec.Metadata().SetVersion(existing.Metadata().Version())
	s.Update(spec)
}

func (s *CPUPartitionSuite) vm(name, slice string, pins ...string) *hypervisor.VirtualMachineSpec {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().CPU.Count = 1
	spec.TypedSpec().CPU.Slice = slice
	spec.TypedSpec().PowerState = "running"

	for i, pin := range pins {
		spec.TypedSpec().CPU.Pins = append(spec.TypedSpec().CPU.Pins, hypervisor.VirtualMachineVCPUPinSpec{VCPU: uint32(i), CPUs: pin})
	}

	return spec
}

// claim simulates the runtime: the domain is started, the placement and domain spec are claimed
// and libvirt reports the domain.
func (s *CPUPartitionSuite) claim(name string) {
	s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, name).Metadata(), "hypervisor.VirtualMachineController")
	s.Create(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name))
}

// release simulates the runtime after the operator stopped the machine and the domain is gone.
func (s *CPUPartitionSuite) release(name string) {
	s.RemoveFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, name).Metadata(), "hypervisor.VirtualMachineController")
	s.Destroy(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name))
}

func (s *CPUPartitionSuite) status() *runtime.CPUPartitionStatusSpec {
	status, err := safe.StateGetByID[*runtime.CPUPartitionStatus](s.Ctx(), s.State(), runtime.CPUPartitionStatusID)
	if err != nil {
		return nil
	}

	return status.TypedSpec()
}

func (s *CPUPartitionSuite) reservation() k8s.KubeletCPUReservationSpec {
	res, err := safe.StateGetByID[*k8s.KubeletCPUReservation](s.Ctx(), s.State(), k8s.KubeletID)
	s.Require().NoError(err)

	return *res.TypedSpec()
}

func (s *CPUPartitionSuite) placement(name string) *hypervisor.VirtualMachineCPUPlacement {
	res, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), name)
	if err != nil {
		return nil
	}

	return res
}

func (s *CPUPartitionSuite) applied() map[string]string {
	out := map[string]string{}

	status := s.status()
	if status == nil {
		return out
	}

	for _, target := range status.Targets {
		out[target.Key] = target.LastApplied
	}

	return out
}

func acceptedApplied() map[string]string {
	return map[string]string{
		"init": "0-1", "system": "0-1", "podruntime": "0-1", "kubepods": "2-3", "taloscontainers": "0-1",
		"virtualMachines": "4-7", "virtualMachines/shared": "6-7", "virtualMachines/database": "4-5",
	}
}

func TestCPUPartitionNoPolicyPublishesUnmanagedOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		synctest.Wait()

		// Projection pending: nothing at all.
		ctest.AssertNoResource[*k8s.KubeletCPUReservation](s, k8s.KubeletID)

		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		s.Assert().Equal(k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Nil(s.status())
		s.Assert().Empty(s.fs.recordedWrites())

		// No heartbeat: time passing produces no writes.
		synctest.Sleep(time.Minute)
		s.Assert().Empty(s.fs.recordedWrites())
	})
}

func TestCPUPartitionContainerModeNoSideEffects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionController{
			V1Alpha1Mode: machineruntime.ModeContainer,
			FS:           s.fs,
		}))
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.Assert().Equal(k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Nil(s.status())
		s.Assert().Empty(s.fs.recordedWrites())
	})
}

func TestCPUPartitionAppliesAndAdmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("init", "0-7")
		s.Create(s.vm("db", "database", "4"))
		s.Create(s.vm("web", ""))
		s.Create(s.vm("stray", "cache"))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		status := s.status()
		s.Require().NotNil(status)
		s.Assert().Equal(runtime.CPUPartitionPhaseReady, status.Phase)
		s.Assert().Empty(status.Blocked)
		s.Assert().Equal(acceptedApplied(), s.applied())

		initTarget, _ := status.Target("init")
		s.Assert().Equal("0-7", initTarget.Initial, "the initial value is recorded before the first write")
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1,4-7"}, s.reservation())

		// Ordering: the VM root grows before its children, kubepods is capped.
		writes := s.fs.recordedWrites()
		s.Assert().Less(slices.Index(writes, "virtualmachines.partition=4-7"), slices.Index(writes, "virtualmachines.partition/database.partition=4-5"))
		s.Assert().Contains(writes, "kubepods=2-3")

		s.Require().NotNil(s.placement("db"))
		s.Assert().Equal(hypervisor.VirtualMachineCPUPlacementSpec{Partition: "/virtualmachines.partition/database.partition", Slice: "database", Exclusive: true}, *s.placement("db").TypedSpec())
		s.Assert().Equal(hypervisor.VirtualMachineCPUPlacementSpec{Partition: "/virtualmachines.partition/shared.partition"}, *s.placement("web").TypedSpec())
		s.Assert().Nil(s.placement("stray"), "an undeclared slice gets no placement")

		// Second owner of the exclusive slice (external spec): no placement for either until resolved.
		s.Create(s.vm("db2", "database"))
		synctest.Wait()
		s.Assert().Nil(s.placement("db2"))
	})
}

func TestCPUPartitionBlockedLeavesEverythingUntouched(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("db", "database"))
		s.Create(s.vm("web", ""))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.claim("db")
		s.claim("web")

		synctest.Wait()

		before := s.fs.recordedWrites()
		reservation := s.reservation()
		dbVersion := s.placement("db").Metadata().Version()

		// Fully occupied swap: rejected.
		swap := acceptedPolicy()
		swap.TypedSpec().Slices[0].CPUs = "6-7"
		s.publish(swap)
		synctest.Wait()

		status := s.status()
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, status.Phase)
		s.Require().NotEmpty(status.Blocked)
		s.Assert().Contains(status.Blocked[0].VirtualMachines, "web")
		s.Assert().Equal(acceptedApplied(), s.applied(), "applied masks are unchanged")
		s.Assert().Equal(before, s.fs.recordedWrites(), "a blocked transition writes nothing")
		s.Assert().Equal(reservation, s.reservation())
		s.Assert().Equal(dbVersion, s.placement("db").Metadata().Version())
		s.Assert().Equal(resource.PhaseRunning, s.placement("db").Metadata().Phase())
		s.Assert().Equal(resource.PhaseRunning, s.placement("web").Metadata().Phase())

		// Desired stopped is not released: still blocked until the domain is gone.
		webSpec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), "web")
		s.Require().NoError(err)
		ctest.UpdateWithConflicts(s, webSpec, func(res *hypervisor.VirtualMachineSpec) error {
			res.TypedSpec().PowerState = "stopped"

			return nil
		})
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Assert().Equal(before, s.fs.recordedWrites())

		// The operator's stop completes (runtime releases): the shared slice is free, database moves.
		s.release("web")
		s.release("db")
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Equal("6-7", s.applied()["virtualMachines/database"])
		s.Assert().Equal("4-5", s.applied()["virtualMachines/shared"])
		s.Assert().Empty(s.status().Blocked)
	})
}

// A claimed placement is proof of occupancy even before libvirt reports the domain (start in
// flight): a swap is blocked on the claim alone.
func TestCPUPartitionInFlightStartOccupies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("db", "database"))
		s.Create(s.vm("web", ""))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		// The runtime claimed both placements; no domain status yet.
		s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "db").Metadata(), "hypervisor.VirtualMachineController")
		s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "web").Metadata(), "hypervisor.VirtualMachineController")
		synctest.Wait()

		before := s.fs.recordedWrites()

		swap := acceptedPolicy()
		swap.TypedSpec().Slices[0].CPUs = "6-7"
		s.publish(swap)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Assert().Equal(before, s.fs.recordedWrites())
		s.Assert().Equal(acceptedApplied(), s.applied())
	})
}

func TestCPUPartitionLiveShrinkPreservesPlacements(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("db", "database"))
		s.Create(s.vm("web1", ""))
		s.Create(s.vm("web2", ""))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.claim("db")
		s.claim("web1")
		s.claim("web2")

		synctest.Wait()

		versions := map[string]resource.Version{}
		for _, name := range []string{"db", "web1", "web2"} {
			versions[name] = s.placement(name).Metadata().Version()
		}

		shrink := acceptedPolicy()
		shrink.TypedSpec().Slices[0].CPUs = "4"
		s.publish(shrink)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Equal("4", s.applied()["virtualMachines/database"])
		s.Assert().Equal("5-7", s.applied()["virtualMachines/shared"])

		for name, version := range versions {
			s.Assert().Equal(version, s.placement(name).Metadata().Version(), "placement %s must not be rewritten by a live mask change", name)
			s.Assert().Equal(resource.PhaseRunning, s.placement(name).Metadata().Phase())
		}

		// One of several shared machines stops: nothing waits for the whole slice to drain.
		s.release("web2")

		web2, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), "web2")
		s.Require().NoError(err)
		s.Destroy(web2)
		synctest.Wait()

		s.Assert().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)
		s.Assert().Equal(resource.PhaseRunning, s.placement("web1").Metadata().Phase())
		ctest.AssertNoResource[*hypervisor.VirtualMachineCPUPlacement](s, "web2")
	})
}

func TestCPUPartitionMixedKubepodsChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)

		// A pod still sits on CPU 2.
		s.fs.setLeaf("burstable/pod/app", "2")

		mixed := acceptedPolicy()
		mixed.TypedSpec().Roots["init"] = "0"
		mixed.TypedSpec().Roots["kubepods"] = "1,3"
		s.publish(mixed)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseConverging, s.status().Phase)
		s.Assert().Equal("1-3", s.applied()["kubepods"], "kubepods widened to the union first")
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0,2,4-7"}, s.reservation())

		// The kubelet moves the container: the barrier is the leaf's effective set, then the cap.
		synctest.Sleep(3 * time.Second)
		s.Require().Equal("1-3", s.applied()["kubepods"], "cap waits for the leaf")

		s.fs.setLeaf("burstable/pod/app", "1")
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Equal("1,3", s.applied()["kubepods"])
	})
}

func TestCPUPartitionWriteFailureAndRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		// Every kernel write must be preceded by a published intent for that exact value: a crash
		// right after the write is then recoverable from the status alone.
		var intents []string

		s.fs.beforeWrite = func(path, cpus string) {
			status := s.status()
			if status == nil {
				intents = append(intents, path+": no status")

				return
			}

			for _, target := range status.Targets {
				if parsed, ok := cpupartition.ParseKey(target.Key); ok && parsed.CgroupPath() == path && target.Intended == cpus {
					return
				}
			}

			intents = append(intents, fmt.Sprintf("%s=%s not announced", path, cpus))
		}

		s.fs.setKubepodsWriteError(errors.New("EBUSY"))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		status := s.status()
		s.Require().NotNil(status)
		s.Assert().Equal(runtime.CPUPartitionPhaseConverging, status.Phase)
		s.Assert().Contains(status.Error, "EBUSY")
		kubepodsTarget, _ := status.Target("kubepods")
		s.Assert().Equal("2-3", kubepodsTarget.Intended, "the intent is recorded before the write")
		s.Assert().Empty(kubepodsTarget.LastApplied)

		// The write actually landed before the failure was reported (crash between write and
		// publication): recovery confirms it from the kernel instead of re-writing.
		s.fs.set("kubepods", "2-3")
		s.fs.setKubepodsWriteError(nil)
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Equal("2-3", s.applied()["kubepods"])
		s.Assert().Empty(intents, "every write must be announced in the status first")
		s.Assert().Zero(strings.Count(strings.Join(s.fs.recordedWrites(), "\n"), "kubepods="), "kubepods was confirmed from the kernel, not rewritten")
	})
}

func TestCPUPartitionForeignDriftAndOfflineCPU(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)

		before := s.fs.recordedWrites()

		s.fs.set("system", "0-3")
		// Drift produces no resource event on its own; the CPU core inventory does.
		s.Create(hardwareCPUCore("cpu0"))
		synctest.Wait()

		s.Assert().Equal([]string{"system"}, s.status().EnforcementLoss)
		s.Assert().Equal(before, s.fs.recordedWrites(), "drift is reported, never silently overwritten")

		s.fs.mu.Lock()
		s.fs.online = mustCPUs("0-6")
		s.fs.mu.Unlock()
		s.Create(hardwareCPUCore("cpu1"))
		synctest.Wait()

		s.Assert().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Assert().Contains(s.status().EnforcementLoss, "virtualMachines/shared")
		s.Assert().Equal(before, s.fs.recordedWrites())
	})
}

func TestCPUPartitionRemovalRestoresAfterRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("init", "0-7")
		s.Create(s.vm("db", "database"))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.claim("db")
		s.fs.setPopulated("virtualmachines.partition/database.partition", true)

		synctest.Wait()

		// Policy removed while the machine runs: rejected (its placement would change), the
		// kernel masks, the placement and the guard stay until the operator stops the machine.
		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Require().NotEmpty(s.status().Blocked)
		s.Assert().Equal([]string{"db"}, s.status().Blocked[0].VirtualMachines)
		s.Assert().Equal(acceptedApplied(), s.applied())
		s.Assert().Equal(resource.PhaseRunning, s.placement("db").Metadata().Phase())
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1,4-7"}, s.reservation())

		s.release("db")
		s.fs.setPopulated("virtualmachines.partition/database.partition", false)
		synctest.Wait()
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Assert().Nil(s.status(), "the guard goes away last")
		ctest.AssertNoResource[*hypervisor.VirtualMachineCPUPlacement](s, "db")
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{}, s.reservation())

		init, err := s.fs.CPUs("init")
		s.Require().NoError(err)
		s.Assert().Equal("0-7", init, "the initial value is restored")

		exists, err := s.fs.Exists("virtualmachines.partition/database.partition")
		s.Require().NoError(err)
		s.Assert().False(exists)
	})
}

// Live finding: a populated root whose initial mask was empty (inherit) cannot be emptied again
// (cgroup v2 ENOSPC); restoration must lift the mask through the parent's effective set instead,
// which is what an empty mask resolves to.
func TestCPUPartitionRemovalRestoresInheritedRoots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)

		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Sleep(3 * time.Second)
		synctest.Wait()

		s.Require().Nil(s.status(), "restoration must complete on populated roots with an inherited initial mask")

		for _, root := range []string{"init", "system", "podruntime", "kubepods", "virtualmachines.partition"} {
			effective, err := s.fs.Effective(root)
			s.Require().NoError(err)
			s.Assert().Equal("0-7", effective.String(), "%s must be back on every CPU", root)
		}
	})
}

func TestCPUPartitionRootsOnlyLeavesVirtualMachinesAlone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("web", ""))
		s.start()

		spec := runtime.NewCPUPartitionSpec()
		spec.TypedSpec().Enabled = true
		spec.TypedSpec().Roots = map[string]string{"init": "0-1"}
		s.publish(spec)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)
		s.Assert().Equal(map[string]string{"init": "0-1"}, s.applied())
		s.Assert().Nil(s.placement("web"), "no virtual machine root: no placements")
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{}, s.reservation())
	})
}

func hardwareCPUCore(id string) resource.Resource {
	return hardware.NewCPUCore(id)
}

// Finding 1: a virtual machine running in the root before the policy managed virtual machines has
// no placement; its claimed domain spec and observed domain are the occupancy evidence. Introducing
// slices would change its placement: blocked, nothing written, no placement, until released.
func TestCPUPartitionIntroductionWithUnplacedRunningVM(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("legacy", "database"))

		domainSpec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "legacy")
		domainSpec.TypedSpec().PowerState = "running"
		s.Create(domainSpec)
		s.AddFinalizer(domainSpec.Metadata(), "hypervisor.VirtualMachineController")
		s.Create(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "legacy"))

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		status := s.status()
		s.Require().NotNil(status)
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, status.Phase)
		s.Require().NotEmpty(status.Blocked)
		s.Assert().Equal([]string{"legacy"}, status.Blocked[0].VirtualMachines)
		s.Assert().Contains(status.Blocked[0].Reason, "placement change")
		s.Assert().Empty(s.applied(), "no mask is written while a root machine runs")
		s.Assert().Empty(s.fs.recordedWrites())
		s.Assert().Nil(s.placement("legacy"), "no placement is granted to a running root machine")
		ctest.AssertNoResource[*k8s.KubeletCPUReservation](s, k8s.KubeletID)

		// A domain whose spec vanished still occupies.
		vm, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](s.Ctx(), s.State(), "legacy")
		s.Require().NoError(err)
		s.Destroy(vm)
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Assert().Empty(s.fs.recordedWrites())

		// The operator's stop: domain removed, claim released (spec removed afterwards).
		s.RemoveFinalizer(domainSpec.Metadata(), "hypervisor.VirtualMachineController")
		s.Destroy(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "legacy"))
		s.Destroy(domainSpec)
		s.Create(s.vm("legacy", "database"))
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Equal(acceptedApplied(), s.applied())
		s.Require().NotNil(s.placement("legacy"))
		s.Assert().Equal("/virtualmachines.partition/database.partition", s.placement("legacy").TypedSpec().Partition)
	})
}

// Finding 2: kubepods is created by the kubelet; until it exists the plan is pending (not ready),
// the kubelet has already received the staged reservation (no bootstrap deadlock), polling keeps
// re-checking, and exclusive machines are admitted only once the cap is verified.
func TestCPUPartitionAbsentKubepodsIsPending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.mu.Lock()
		delete(s.fs.cpus, "kubepods")
		s.fs.mu.Unlock()

		s.Create(s.vm("db", "database"))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		status := s.status()
		s.Require().NotNil(status)
		s.Require().Equal(runtime.CPUPartitionPhaseConverging, status.Phase)
		s.Assert().Contains(status.Waiting, "kubepods")
		s.Assert().Equal(k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1,4-7"}, s.reservation(), "the kubelet gets the reservation before its cgroup is awaited")
		s.Assert().Nil(s.placement("db"), "no admission before every competing boundary is verified")
		s.Assert().NotContains(s.applied(), "kubepods")

		// The kubelet creates its cgroup: polling picks it up without any resource event.
		s.Require().NoError(s.fs.Ensure("kubepods"))
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Empty(s.status().Waiting)
		s.Assert().Equal("2-3", s.applied()["kubepods"])
		s.Require().NotNil(s.placement("db"))
	})
}

// Finding 3: a foreign change to a competing root while the desired policy is unchanged yields no
// steps, but the loss must not produce ready/admission: it blocks, and new machines get no placement.
func TestCPUPartitionEnforcementLossClosesAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)

		before := s.fs.recordedWrites()

		s.fs.set("init", "0-5")
		s.Create(s.vm("db", "database"))
		synctest.Wait()

		status := s.status()
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, status.Phase)
		s.Assert().Equal([]string{"init"}, status.EnforcementLoss)
		s.Require().NotEmpty(status.Blocked)
		s.Assert().Contains(status.Blocked[0].Reason, "enforcement lost")
		s.Assert().Nil(s.placement("db"), "no admission on a boundary known to be invalid")
		s.Assert().Equal(before, s.fs.recordedWrites(), "the foreign mask is not overwritten")

		// Restored outside: admission reopens.
		s.fs.set("init", "0-1")
		s.Create(hardwareCPUCore("cpu0"))
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)
		s.Require().NotNil(s.placement("db"))
	})
}

// Finding 6: a kubepods leaf which never converges stalls the transition observably and bounded:
// the barrier and the leaf are in the status, a timeout marks the transition failed, nothing is
// granted, nothing reverted, and progress resumes when the leaf moves. Nested pod cgroups and
// leaves without an explicit mask are identified correctly.
func TestCPUPartitionKubepodsBarrierIsObservableAndBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionController{
			V1Alpha1Mode:   machineruntime.ModeMetal,
			FS:             s.fs,
			PollInterval:   time.Second,
			BarrierTimeout: 10 * time.Second,
		}))
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.fs.setLeaf("burstable/pod-a/app", "2")

		shrink := acceptedPolicy()
		shrink.TypedSpec().Roots["kubepods"] = "3"

		s.Create(s.vm("db", "database"))
		s.publish(shrink)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseConverging, s.status().Phase)
		s.Assert().Equal(`kubepods leaf "burstable/pod-a/app" still runs on CPUs 2`, s.status().Waiting)
		s.Assert().Empty(s.status().Error)
		s.Assert().Nil(s.placement("db"), "nothing is granted while the transition is pending")
		s.Assert().Equal("2-3", s.applied()["kubepods"], "the cap is not tightened under the leaf")

		synctest.Sleep(11 * time.Second)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseConverging, s.status().Phase)
		s.Assert().Contains(s.status().Error, "did not converge within 10s")
		s.Assert().Contains(s.status().Error, "burstable/pod-a/app")
		s.Assert().Equal("2-3", s.applied()["kubepods"], "a stalled transition keeps the applied state for retry or revert")

		s.fs.setLeaf("burstable/pod-a/app", "3")
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)
		s.Assert().Empty(s.status().Waiting)
		s.Assert().Equal("3", s.applied()["kubepods"])
		s.Require().NotNil(s.placement("db"))
	})
}

// Finding 7: admission refusals appear in the status, and unrelated valid machines keep theirs.
func TestCPUPartitionAdmissionErrorsInStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("web", ""))
		s.Create(s.vm("db1", "database"))
		s.Create(s.vm("db2", "database"))
		s.Create(s.vm("stray", "cache"))
		s.Create(s.vm("pinned", "", "1"))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		status := s.status()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, status.Phase)
		s.Require().NotNil(s.placement("web"))
		s.Assert().Nil(s.placement("db1"))
		s.Assert().Nil(s.placement("db2"))
		s.Assert().Nil(s.placement("stray"))
		s.Assert().Nil(s.placement("pinned"))

		var refused []string
		for _, block := range status.AdmissionErrors {
			refused = append(refused, block.VirtualMachines...)
		}

		s.Assert().ElementsMatch([]string{"db1", "db2", "stray", "pinned"}, refused)
	})
}

// Finding 9: after a failed write, a kernel value matching neither the intent nor the last applied
// value is foreign drift, not a forgotten intent (which would silently trust LastApplied).
func TestCPUPartitionThirdValueAfterFailedWriteIsDrift(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)

		// The next write to kubepods fails: the intent (2) differs from the last applied (2-3).
		s.fs.setKubepodsWriteError(errors.New("EBUSY"))

		shrink := acceptedPolicy()
		shrink.TypedSpec().Roots["kubepods"] = "2"
		s.publish(shrink)
		synctest.Wait()

		kubepods, _ := s.status().Target("kubepods")
		s.Require().Equal("2", kubepods.Intended)
		s.Require().Equal("2-3", kubepods.LastApplied)

		// Someone else wrote a third value before the retry. A recovery which forgets the intent
		// would trust LastApplied, plan the shrink again and overwrite the foreign value in the
		// very same reconcile; the drift must be detected before any plan is executed.
		var observedOnRetry []string

		s.fs.beforeWrite = func(path, cpus string) {
			observedOnRetry = append(observedOnRetry, path+"="+cpus)
		}

		s.fs.set("kubepods", "2-5")
		s.fs.setKubepodsWriteError(nil)
		synctest.Sleep(2 * time.Second)
		synctest.Wait()

		s.Assert().Empty(observedOnRetry, "the foreign value is not overwritten on the retry")
		s.Require().Equal(runtime.CPUPartitionPhaseBlocked, s.status().Phase)
		s.Assert().Equal([]string{"kubepods"}, s.status().EnforcementLoss)
		s.Assert().Empty(s.status().Error, "the stale write error is cleared once the loss is reported")
	})
}

// Steady state: once the applied policy equals the desired one, unrelated events (virtual
// machine specs, domain observations, claims, CPU core inventory) must produce no status or
// reservation write, no cgroup write and no admission closure.
func TestCPUPartitionSteadyStateIsQuiet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.Create(s.vm("db", "database"))
		s.Create(s.vm("web", ""))
		s.start()
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.claim("db")
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase, s.status().Error)

		statusRes, err := safe.StateGetByID[*runtime.CPUPartitionStatus](s.Ctx(), s.State(), runtime.CPUPartitionStatusID)
		s.Require().NoError(err)

		reservationRes, err := safe.StateGetByID[*k8s.KubeletCPUReservation](s.Ctx(), s.State(), k8s.KubeletID)
		s.Require().NoError(err)

		statusVersion, reservationVersion := statusRes.Metadata().Version(), reservationRes.Metadata().Version()
		writes := s.fs.recordedWrites()

		var phases []runtime.CPUPartitionPhase

		s.fs.beforeWrite = func(string, string) { phases = append(phases, s.status().Phase) }

		// Unrelated events: a new machine, a domain observation, a claim, a core.
		s.Create(s.vm("extra", ""))
		synctest.Wait()
		s.claim("web")
		synctest.Wait()
		s.Create(hardwareCPUCore("cpu3"))
		synctest.Wait()
		s.release("web")
		synctest.Wait()

		s.Require().Equal(runtime.CPUPartitionPhaseReady, s.status().Phase)
		s.Assert().Equal(statusVersion, s.statusVersion(), "the status must not be rewritten by unrelated events")
		s.Assert().Equal(reservationVersion, s.reservationVersion(), "the reservation must not be republished")
		s.Assert().Equal(writes, s.fs.recordedWrites())
		s.Assert().Empty(phases)
		s.Require().NotNil(s.placement("extra"), "admission stays open")
	})
}

func (s *CPUPartitionSuite) statusVersion() resource.Version {
	res, err := safe.StateGetByID[*runtime.CPUPartitionStatus](s.Ctx(), s.State(), runtime.CPUPartitionStatusID)
	s.Require().NoError(err)

	return res.Metadata().Version()
}

func (s *CPUPartitionSuite) reservationVersion() resource.Version {
	res, err := safe.StateGetByID[*k8s.KubeletCPUReservation](s.Ctx(), s.State(), k8s.KubeletID)
	s.Require().NoError(err)

	return res.Metadata().Version()
}
