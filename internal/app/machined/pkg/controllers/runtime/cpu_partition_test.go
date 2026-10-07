// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/containerd/containerd/v2/core/events"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const runtimeFinalizer = "hypervisor.VirtualMachineController"

// fakeCgroupFS is an in-memory cgroup v2 tree: a cgroup's effective set is its mask intersected
// with its parent's effective set, falling back to the parent's when empty. Kernel refusals and
// effective-set anomalies are injected explicitly per path; populated is set per cgroup.
type fakeCgroupFS struct {
	mu        sync.Mutex
	online    cpuset.CPUSet
	cpus      map[string]string
	populated map[string]bool
	leaves    map[string]cpuset.CPUSet
	writeErr  map[string]error
	// emptyErr fails only writes of the empty mask.
	emptyErr map[string]error
	// effective overrides the computed effective set; effectiveErr fails reading it.
	effective    map[string]string
	effectiveErr map[string]error
	writes       []string
	calls        int

	// beforeWrite runs before every SetCPUs and Remove, outside the lock.
	beforeWrite func(op string)
	// onPopulated runs on every Populated call, outside the lock.
	onPopulated func(path string)
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

		emptyErr:     map[string]error{},
		effective:    map[string]string{},
		effectiveErr: map[string]error{},
	}

	for _, root := range []string{"init", "system", "podruntime", "kubepods", "taloscontainers", "virtualmachines.partition"} {
		fs.cpus[root] = ""
		fs.populated[root] = root != "taloscontainers" && root != "virtualmachines.partition"
	}

	return fs
}

func (fs *fakeCgroupFS) lock() func() {
	fs.mu.Lock()
	fs.calls++

	return fs.mu.Unlock
}

func (fs *fakeCgroupFS) Online() (cpuset.CPUSet, error) {
	defer fs.lock()()

	return fs.online, nil
}

func (fs *fakeCgroupFS) Exists(path string) (bool, error) {
	defer fs.lock()()

	_, ok := fs.cpus[path]

	return ok, nil
}

func (fs *fakeCgroupFS) Ensure(path string) error {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		fs.cpus[path] = ""
		fs.writes = append(fs.writes, "mkdir "+path)
	}

	return nil
}

func (fs *fakeCgroupFS) Remove(path string) error {
	fs.hook("remove " + path)

	defer fs.lock()()

	if fs.populated[path] {
		return errors.New("device or resource busy")
	}

	delete(fs.cpus, path)
	fs.writes = append(fs.writes, "remove "+path)

	return nil
}

func (fs *fakeCgroupFS) CPUs(path string) (string, error) {
	defer fs.lock()()

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

	if fs.cpus[path] == "" {
		return parent
	}

	if set := mustCPUs(fs.cpus[path]).Intersection(parent); !set.IsEmpty() {
		return set
	}

	return parent
}

func (fs *fakeCgroupFS) Effective(path string) (cpuset.CPUSet, error) {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		return cpuset.New(), errors.New("no such cgroup")
	}

	if err := fs.effectiveErr[path]; err != nil {
		return cpuset.New(), err
	}

	if override, ok := fs.effective[path]; ok {
		return mustCPUs(override), nil
	}

	return fs.effectiveLocked(path), nil
}

// Children lists the direct child cgroups.
func (fs *fakeCgroupFS) Children(path string) ([]string, error) {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		return nil, errors.New("no such cgroup")
	}

	var children []string

	for candidate := range fs.cpus {
		if child, ok := strings.CutPrefix(candidate, path+"/"); ok && !strings.Contains(child, "/") {
			children = append(children, child)
		}
	}

	slices.Sort(children)

	return children, nil
}

func (fs *fakeCgroupFS) SetCPUs(path, cpus string) error {
	fs.hook(path + "=" + cpus)

	defer fs.lock()()

	if err := fs.writeErr[path]; err != nil {
		return err
	}

	if _, ok := fs.cpus[path]; !ok {
		return errors.New("no such cgroup")
	}

	if err := fs.emptyErr[path]; err != nil && cpus == "" {
		return err
	}

	fs.cpus[path] = cpus
	fs.writes = append(fs.writes, path+"="+cpus)

	return nil
}

func (fs *fakeCgroupFS) Populated(path string) (bool, error) {
	fs.mu.Lock()
	hook := fs.onPopulated
	fs.mu.Unlock()

	if hook != nil {
		hook(path)
	}

	defer fs.lock()()

	return fs.populated[path], nil
}

func (fs *fakeCgroupFS) LeafEffective(string) (map[string]cpuset.CPUSet, error) {
	defer fs.lock()()

	return maps.Clone(fs.leaves), nil
}

func (fs *fakeCgroupFS) hook(op string) {
	fs.mu.Lock()
	hook := fs.beforeWrite
	fs.mu.Unlock()

	if hook != nil {
		hook(op)
	}
}

func (fs *fakeCgroupFS) do(fn func(fs *fakeCgroupFS)) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fn(fs)
}

func (fs *fakeCgroupFS) set(path, cpus string) {
	fs.do(func(fs *fakeCgroupFS) { fs.cpus[path] = cpus })
}

func (fs *fakeCgroupFS) mask(path string) string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.cpus[path]
}

func (fs *fakeCgroupFS) exists(path string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	_, ok := fs.cpus[path]

	return ok
}

func (fs *fakeCgroupFS) recordedWrites() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return slices.Clone(fs.writes)
}

func (fs *fakeCgroupFS) callCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.calls
}

type cpuPartitionSuite struct {
	ctest.DefaultSuite

	fs  *fakeCgroupFS
	cpu *partitionKubeletClient
}

// newCPUPartitionSuite must be called inside the synctest bubble; the caller defers TearDownTest.
func newCPUPartitionSuite(t *testing.T) *cpuPartitionSuite {
	s := &cpuPartitionSuite{fs: newFakeCgroupFS(), cpu: &partitionKubeletClient{events: make(chan *events.Envelope, 1)}}
	s.Timeout = time.Hour
	s.SetT(t)
	s.SetupTest()

	return s
}

func (s *cpuPartitionSuite) start() {
	s.startWith(&runtimectrls.CPUPartitionController{V1Alpha1Mode: machineruntime.ModeMetal})
}

func (s *cpuPartitionSuite) startWith(ctrl *runtimectrls.CPUPartitionController) {
	ctrl.FS = s.fs

	if ctrl.PollInterval == 0 {
		ctrl.PollInterval = time.Second
	}

	s.Require().NoError(s.Runtime().RegisterController(&k8sctrls.KubeletCPUObservationController{
		NewClient: func() (k8sctrls.KubeletCPUClient, error) { return s.cpu, nil },
	}))
	s.Require().NoError(s.Runtime().RegisterController(ctrl))
}

func acceptedPolicy() *runtime.CPUPartitionSpec {
	spec := runtime.NewCPUPartitionSpec()
	spec.TypedSpec().Enabled = true
	spec.TypedSpec().Roots = map[string]string{
		"init": "0-1", "system": "0-1", "podruntime": "0-1", "kubepods": "2-3", "taloscontainers": "0-1", "virtualmachines": "4-7",
	}
	spec.TypedSpec().Slices = []runtime.CPUPartitionSliceSpec{{Name: "database", CPUs: "4-5", Exclusive: true}}

	return spec
}

func kubernetesOnlyPolicy() *runtime.CPUPartitionSpec {
	spec := runtime.NewCPUPartitionSpec()
	spec.TypedSpec().Enabled = true
	spec.TypedSpec().Roots = map[string]string{"init": "0", "system": "0", "podruntime": "0-1", "taloscontainers": "0", "kubepods": "1-3"}

	return spec
}

func acceptedApplied() map[string]string {
	return map[string]string{
		"init": "0-1", "system": "0-1", "podruntime": "0-1", "kubepods": "2-3", "taloscontainers": "0-1",
		"virtualmachines": "4-7", "virtualmachines/shared": "6-7", "virtualmachines/database": "4-5",
	}
}

// replace creates or updates a resource with the current version.
func replace[R resource.Resource](s *cpuPartitionSuite, res R) {
	existing, err := s.State().Get(s.Ctx(), res.Metadata())
	if err != nil {
		s.Create(res)

		return
	}

	res.Metadata().SetVersion(existing.Metadata().Version())
	s.Update(res)
}

func (s *cpuPartitionSuite) publish(spec *runtime.CPUPartitionSpec) {
	replace(s, spec)
}

// kubelet renders the kubelet configuration with the reservation, as the kubelet integration will.
func (s *cpuPartitionSuite) kubelet(reserved string) {
	spec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	spec.TypedSpec().Config = map[string]any{}

	if reserved != "" {
		spec.Metadata().Annotations().Set(k8s.KubeletCPUManagedAnnotation, "true")
		spec.TypedSpec().Config["reservedSystemCPUs"] = reserved
		spec.TypedSpec().Args = []string{"--reserved-cpus=" + reserved, "--cpu-manager-policy=static", "--cpu-manager-policy-options=strict-cpu-reservation=true"}
	}

	replace(s, spec)
	current, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
	s.Require().NoError(err)
	s.Require().NoError(s.cpu.launch(current))
}

func (s *cpuPartitionSuite) vm(name, slice string, pins ...string) {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	spec.TypedSpec().CPU.Count = 1
	spec.TypedSpec().CPU.Slice = slice
	spec.TypedSpec().PowerState = "running"

	for i, pin := range pins {
		spec.TypedSpec().CPU.Pins = append(spec.TypedSpec().CPU.Pins, hypervisor.VirtualMachineVCPUPinSpec{VCPU: uint32(i), CPUs: pin})
	}

	replace(s, spec)
}

// start simulates the runtime's admitted start: the placement is claimed, then libvirt lists the domain.
func (s *cpuPartitionSuite) claim(name string) {
	s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, name).Metadata(), runtimeFinalizer)
	s.Create(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name))
}

// releaseDB simulates the runtime removing the database domain and only then releasing its claim.
func (s *cpuPartitionSuite) releaseDB() {
	s.Destroy(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "db"))
	s.RemoveFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "db").Metadata(), runtimeFinalizer)
}

func (s *cpuPartitionSuite) status() *runtime.CPUPartitionStatusSpec {
	status, err := safe.StateGetByID[*runtime.CPUPartitionStatus](s.Ctx(), s.State(), runtime.CPUPartitionStatusID)
	if err != nil {
		return nil
	}

	return status.TypedSpec()
}

func (s *cpuPartitionSuite) requirePhase(phase runtime.CPUPartitionPhase) *runtime.CPUPartitionStatusSpec {
	status := s.status()
	s.Require().NotNil(status)
	s.Require().Equal(phase, status.Phase, "error %q, waiting %q, blocked %v", status.Error, status.Waiting, status.Blocked)

	return status
}

func (s *cpuPartitionSuite) reservation() *k8s.KubeletCPUReservationSpec {
	res, err := safe.StateGetByID[*k8s.KubeletCPUReservation](s.Ctx(), s.State(), k8s.KubeletID)
	if err != nil {
		return nil
	}

	return res.TypedSpec()
}

func (s *cpuPartitionSuite) placement(name string) *hypervisor.VirtualMachineCPUPlacement {
	res, err := safe.StateGetByID[*hypervisor.VirtualMachineCPUPlacement](s.Ctx(), s.State(), name)
	if err != nil {
		return nil
	}

	return res
}

func (s *cpuPartitionSuite) applied() map[string]string {
	out := map[string]string{}

	if status := s.status(); status != nil {
		for _, target := range status.Targets {
			out[target.Key] = target.LastApplied
		}
	}

	return out
}

func (s *cpuPartitionSuite) version(res resource.Resource) resource.Version {
	current, err := s.State().Get(s.Ctx(), res.Metadata())
	s.Require().NoError(err)

	return current.Metadata().Version()
}

func (s *cpuPartitionSuite) wake(id string) {
	s.Create(hardware.NewCPUCore(id))
}

func (s *cpuPartitionSuite) setOnline(list string) {
	s.fs.do(func(fs *fakeCgroupFS) { fs.online = mustCPUs(list) })
}

// converge publishes the accepted policy with the kubelet already reading back the reservation.
func (s *cpuPartitionSuite) converge(spec *runtime.CPUPartitionSpec) {
	s.kubelet("0-1,4-7")
	s.start()
	s.publish(spec)
	synctest.Wait()
}

// announced checks that a kernel write ("path=cpus" or "remove path") was published as the intent
// of its target, with admission closed.
func announced(status *runtime.CPUPartitionStatusSpec, op string) string {
	if status == nil || status.Phase.AdmissionOpen() {
		return "admission open"
	}

	path, cpus, isWrite := strings.Cut(op, "=")
	if !isWrite {
		path = strings.TrimPrefix(op, "remove ")
	}

	for _, entry := range status.Targets {
		target, ok := cpupartition.ParseKey(entry.Key)
		if !ok || target.CgroupPath() != path {
			continue
		}

		if entry.Intended == cpus && entry.Intended != entry.LastApplied {
			return ""
		}
	}

	return "not announced"
}

func TestCPUPartitionPendingThenDisabledTouchesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("web", "")
		s.start()
		synctest.Wait()

		s.Assert().Nil(s.reservation(), "projection pending: nothing is published")
		s.Assert().Nil(s.status())

		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Nil(s.status())
		s.Assert().Nil(s.placement("web"))
		s.Assert().Zero(s.fs.callCount(), "no policy and nothing applied: no filesystem access")

		synctest.Sleep(time.Hour)
		synctest.Wait()
		s.Assert().Zero(s.fs.callCount(), "no heartbeat")
	})
}

func TestCPUPartitionContainerModeDoesNotEnforce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.startWith(&runtimectrls.CPUPartitionController{V1Alpha1Mode: machineruntime.ModeContainer})
		s.publish(acceptedPolicy())
		synctest.Wait()

		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Nil(s.status())
		s.Assert().Zero(s.fs.callCount())
	})
}

// Kubernetes-only: no virtual machine resources, libvirt or hypervisor readiness exist. The pod
// parent is narrowed only after the kubelet configuration reads the reservation back and every
// configured pod leaf left the removed CPUs; removal widens it before the reservation goes.
func TestCPUPartitionKubernetesOnlyLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("init", "0-5")
		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves["burstable/pod-a/app"] = mustCPUs("5") })

		var kubepodsCap []string

		s.fs.beforeWrite = func(op string) {
			if strings.HasPrefix(op, "kubepods=") {
				if reservation := s.reservation(); reservation != nil && !reservation.Managed {
					op += " after the reservation was withdrawn"
				}

				kubepodsCap = append(kubepodsCap, op)
			}
		}

		s.start()
		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Contains(status.Waiting, "kubelet configuration has not been rendered")
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0,4-7"}, s.reservation())
		s.Assert().Empty(kubepodsCap, "a published reservation is not a consumed one")
		s.Assert().Equal("0", s.fs.mask("init"))

		s.kubelet("0,4-7")
		synctest.Wait()

		status = s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Equal(`kubepods leaf "burstable/pod-a/app" still runs on CPUs 5`, status.Waiting)
		s.Assert().Empty(kubepodsCap, "the parent is not narrowed under a pinned pod")

		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves["burstable/pod-a/app"] = mustCPUs("2") })
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal([]string{"kubepods=1-3"}, kubepodsCap)
		s.Assert().Equal(map[string]string{
			"init": "0", "system": "0", "podruntime": "0-1", "taloscontainers": "0", "kubepods": "1-3",
		}, s.applied())
		s.Assert().False(s.fs.exists("virtualmachines.partition/shared.partition"))
		s.Assert().Empty(s.fs.mask("virtualmachines.partition"))

		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		s.Assert().Nil(s.status(), "restoration completes and the guard goes last")
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Equal("0-5", s.fs.mask("init"), "the original mask is restored")

		for _, root := range []string{"system", "podruntime", "kubepods", "taloscontainers"} {
			s.Assert().Empty(s.fs.mask(root), "an inherited original is inherited again, populated or not")
		}

		// Widened to every CPU while the reservation is still managed; inheriting again afterwards
		// keeps the same CPUs.
		s.Assert().Equal([]string{"kubepods=1-3", "kubepods=0-7", "kubepods= after the reservation was withdrawn"}, kubepodsCap)
	})
}

func TestCPUPartitionKubepodsRemovalKeepsReservationUntilRootRestored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("kubepods", "0-7")
		s.kubelet("0,4-7")
		s.start()
		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Require().Equal("1-3", s.fs.mask("kubepods"))
		s.Require().True(s.reservation().Managed)

		var restored bool

		s.fs.beforeWrite = func(op string) {
			if op == "kubepods=0-7" {
				restored = true

				s.Assert().True(s.reservation().Managed, "reservation must remain managed until kubepods root is restored")
			}
		}

		spec := kubernetesOnlyPolicy()
		delete(spec.TypedSpec().Roots, "kubepods")
		spec.TypedSpec().Roots["init"] = "0-1"
		s.publish(spec)
		synctest.Wait()

		s.Require().True(restored, "must exercise the root restoration write: status=%+v writes=%v reservation=%+v", s.status(), s.fs.recordedWrites(), s.reservation())
		s.Assert().False(s.reservation().Managed)
	})
}

// Without kubepods there is no handoff: the reservation is unmanaged and nothing waits for the kubelet.
func TestCPUPartitionWithoutKubepodsNeedsNoKubelet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		spec := runtime.NewCPUPartitionSpec()
		spec.TypedSpec().Enabled = true
		spec.TypedSpec().Roots = map[string]string{"init": "0-1", "virtualmachines": "2-7"}

		s.vm("web", "")
		s.start()
		s.publish(spec)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Equal(map[string]string{"init": "0-1", "virtualmachines": "2-7", "virtualmachines/shared": "2-7"}, s.applied())
		s.Assert().Equal("/virtualmachines.partition/shared.partition", s.placement("web").TypedSpec().Partition)
	})
}

// A retry finds the reservation already published and plans no publication; the kubelet's
// configuration still has to read it back before kubepods is capped.
func TestCPUPartitionReadBackRequiredWithoutPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		replace(s, func() *k8s.KubeletCPUReservation {
			res := k8s.NewKubeletCPUReservation()
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0,4-7"}

			return res
		}())
		s.kubelet("0-7")
		s.start()
		s.publish(kubernetesOnlyPolicy())
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Contains(status.Waiting, `reserves CPUs "0-7", waiting for "0,4-7"`)
		s.Assert().Empty(s.fs.mask("kubepods"))

		s.kubelet("0,4-7")
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("1-3", s.fs.mask("kubepods"))
	})
}

func TestCPUPartitionAbsentKubepodsIsPending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.do(func(fs *fakeCgroupFS) { delete(fs.cpus, "kubepods") })
		s.vm("db", "database")
		s.converge(acceptedPolicy())

		status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Contains(status.Waiting, "kubepods cgroup has not been created")
		s.Assert().NotContains(s.applied(), "kubepods")
		s.Assert().Nil(s.placement("db"), "nothing is admitted before every boundary is verified")

		s.fs.set("kubepods", "")
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("2-3", s.applied()["kubepods"])
		s.Assert().NotNil(s.placement("db"))
	})
}

func TestCPUPartitionAppliesAndAdmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("init", "0-7")
		s.vm("db", "database", "4")
		s.vm("web", "")
		s.vm("stray", "cache")
		s.vm("pinned", "", "1")
		s.converge(acceptedPolicy())

		status := s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal(acceptedApplied(), s.applied())
		s.Assert().Equal([]string{"database"}, status.Exclusive)

		initTarget, _ := status.Target("init")
		s.Assert().Equal("0-7", initTarget.Initial)

		writes := s.fs.recordedWrites()
		s.Assert().Less(slices.Index(writes, "virtualmachines.partition=4-7"), slices.Index(writes, "virtualmachines.partition/database.partition=4-5"))
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1,4-7"}, s.reservation())

		s.Require().NotNil(s.placement("db"))
		s.Assert().Equal(hypervisor.VirtualMachineCPUPlacementSpec{
			Partition: "/virtualmachines.partition/database.partition", Slice: "database", Exclusive: true,
		}, *s.placement("db").TypedSpec())
		s.Assert().Equal(hypervisor.VirtualMachineCPUPlacementSpec{Partition: "/virtualmachines.partition/shared.partition"}, *s.placement("web").TypedSpec())
		s.Assert().Nil(s.placement("stray"))
		s.Assert().Nil(s.placement("pinned"))

		var refused []string
		for _, block := range status.AdmissionErrors {
			refused = append(refused, block.VirtualMachines...)
		}

		s.Assert().ElementsMatch([]string{"stray", "pinned"}, refused)

		// A second selector of the granted exclusive slice is refused; the first keeps its grant.
		s.vm("db2", "database")
		synctest.Wait()

		s.Assert().Nil(s.placement("db2"))
		s.Assert().NotNil(s.placement("db"))
	})
}

// Every kernel write is preceded by a published intent for that exact value, admission is closed
// while writing, and a failed write leaves a recoverable intent which the kernel then decides.
func TestCPUPartitionIntentBeforeWriteAndRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		var violations []string

		s.fs.beforeWrite = func(op string) {
			if reason := announced(s.status(), op); reason != "" {
				violations = append(violations, op+": "+reason)
			}
		}

		s.fs.do(func(fs *fakeCgroupFS) { fs.writeErr["kubepods"] = errors.New("device busy") })
		s.converge(acceptedPolicy())

		status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Contains(status.Error, "device busy")

		kubepods, _ := status.Target("kubepods")
		s.Assert().Equal("2-3", kubepods.Intended)
		s.Assert().Empty(kubepods.LastApplied)

		// The write landed although it was reported failed: the kernel confirms it, no rewrite.
		s.fs.do(func(fs *fakeCgroupFS) {
			fs.cpus["kubepods"] = "2-3"
			delete(fs.writeErr, "kubepods")
		})
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal(acceptedApplied(), s.applied())
		s.Assert().Empty(violations)
		s.Assert().NotContains(s.fs.recordedWrites(), "kubepods=2-3")
	})
}

// After a failed write, a third value is foreign drift: neither the intent nor the last applied
// value is adopted, and nothing overwrites it.
func TestCPUPartitionThirdValueAfterFailedWriteIsDrift(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		s.fs.do(func(fs *fakeCgroupFS) { fs.writeErr["kubepods"] = errors.New("device busy") })

		shrink := acceptedPolicy()
		shrink.TypedSpec().Roots["kubepods"] = "2-4"
		shrink.TypedSpec().Roots["virtualmachines"] = "5-7"
		shrink.TypedSpec().Slices[0].CPUs = "5"
		s.publish(shrink)
		synctest.Wait()

		kubepods, _ := s.status().Target("kubepods")
		s.Require().Equal("2-4", kubepods.Intended)
		s.Require().Equal("2-3", kubepods.LastApplied)

		before := s.fs.recordedWrites()

		s.fs.do(func(fs *fakeCgroupFS) {
			fs.cpus["kubepods"] = "2-6"
			delete(fs.writeErr, "kubepods")
		})
		synctest.Sleep(time.Second)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Equal([]string{"kubepods"}, status.EnforcementLoss)
		s.Assert().Empty(status.Error)
		s.Assert().Equal(before, s.fs.recordedWrites())
		s.Assert().Equal("2-6", s.fs.mask("kubepods"))
	})
}

// Enforcement loss blocks every transition, including a desired policy which drops the offline CPU
// and the removal of the policy, refuses new placements and keeps existing ones.
func TestCPUPartitionEnforcementLoss(t *testing.T) {
	for _, tc := range []struct {
		name    string
		desired func() *runtime.CPUPartitionSpec
		phase   runtime.CPUPartitionPhase
	}{
		{"unchanged policy", acceptedPolicy, runtime.CPUPartitionPhaseBlocked},
		{"policy dropping the offline CPU", func() *runtime.CPUPartitionSpec {
			spec := acceptedPolicy()
			spec.TypedSpec().Roots["virtualmachines"] = "4,6-7"
			spec.TypedSpec().Slices[0].CPUs = "4"

			return spec
		}, runtime.CPUPartitionPhaseBlocked},
		{"policy removal", runtime.NewCPUPartitionSpec, runtime.CPUPartitionPhaseRestoring},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newCPUPartitionSuite(t)
				defer s.TearDownTest()

				s.vm("db", "database")
				s.converge(acceptedPolicy())
				s.requirePhase(runtime.CPUPartitionPhaseReady)
				s.claim("db")
				synctest.Wait()

				before, reservation := s.fs.recordedWrites(), *s.reservation()

				s.setOnline("0-4,6-7")
				s.vm("web", "")
				s.publish(tc.desired())
				synctest.Wait()

				status := s.requirePhase(tc.phase)
				s.Require().NotEmpty(status.Blocked)
				s.Assert().Equal([]string{"virtualmachines", "virtualmachines/database"}, status.EnforcementLoss)
				s.Assert().Contains(status.Blocked[0].Reason, "applied CPUs 5 are offline")
				s.Assert().Equal(acceptedApplied(), s.applied())
				s.Assert().Equal(before, s.fs.recordedWrites())
				s.Assert().Equal(reservation, *s.reservation())
				s.Assert().Nil(s.placement("web"), "no new admission on a lost boundary")
				s.Assert().Equal(resource.PhaseRunning, s.placement("db").Metadata().Phase(), "the running machine is left alone")

				s.setOnline("0-7")
				s.wake("cpu5")
				synctest.Wait()

				if status := s.status(); status != nil {
					s.Assert().Empty(status.EnforcementLoss)
				}
			})
		})
	}
}

func TestCPUPartitionForeignDriftIsReported(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		before := s.fs.recordedWrites()

		s.fs.set("init", "0-5")
		s.fs.set("virtualmachines.partition/shared.partition", "")
		s.vm("db", "database")
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Equal([]string{"init", "virtualmachines/shared"}, status.EnforcementLoss)
		s.Assert().Nil(s.placement("db"))
		s.Assert().Equal(before, s.fs.recordedWrites(), "a foreign mask is never overwritten")

		// Removal does not restore over a foreign mask either.
		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		s.Assert().NotEmpty(s.requirePhase(runtime.CPUPartitionPhaseRestoring).Blocked)
		s.Assert().Equal(before, s.fs.recordedWrites())

		s.fs.set("init", "0-1")
		s.fs.set("virtualmachines.partition/shared.partition", "6-7")
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.Assert().Nil(s.status(), "restored outside: removal proceeds")
	})
}

// The host CPU pins of a running domain are those it was started with, which no resource records:
// an occupied partition is not shrunk, whatever the current specification says.
func TestCPUPartitionOccupiedShrinkProtectsUnknownPins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.claim("db")
		s.claim("web")
		synctest.Wait()

		before := s.fs.recordedWrites()
		versions := map[string]resource.Version{"db": s.version(s.placement("db")), "web": s.version(s.placement("web"))}

		// The running database domain may be pinned to 5 even though its specification is not.
		shrink := acceptedPolicy()
		shrink.TypedSpec().Slices[0].CPUs = "4"
		s.publish(shrink)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Contains(status.Blocked[0].Reason, "host CPU pins")
		s.Assert().Equal([]string{"db"}, status.Blocked[0].VirtualMachines)
		s.Assert().Equal(before, s.fs.recordedWrites())

		for name, version := range versions {
			s.Assert().Equal(version, s.version(s.placement(name)), "a blocked transition leaves placement %s alone", name)
		}

		// Once the domain is removed and its claim released, the slice shrinks live.
		s.releaseDB()
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("4", s.applied()["virtualmachines/database"])
		s.Assert().Equal("5-7", s.applied()["virtualmachines/shared"])
		s.Assert().Equal(versions["web"], s.version(s.placement("web")), "a mask change keeps the running domain's placement")
	})
}

// A claim the first look missed but which was taken before admission closed is in the
// authoritative snapshot, and the transition it makes unsafe is not executed.
func TestCPUPartitionSnapshotAfterAdmissionClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		before := s.fs.recordedWrites()

		var once sync.Once

		s.fs.onPopulated = func(string) {
			once.Do(func() {
				// The runtime claims its placement while the controller is observing, before it
				// publishes the closed phase.
				s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "web").Metadata(), runtimeFinalizer)
			})
		}

		swap := acceptedPolicy()
		swap.TypedSpec().Slices[0].CPUs = "6-7"
		s.publish(swap)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Contains(status.Blocked[0].VirtualMachines, "web")
		s.Assert().Equal(before, s.fs.recordedWrites())
		s.Assert().Equal(acceptedApplied(), s.applied())
	})
}

// Occupancy ends only with the domain gone and the claims released: a held claim without a
// listed domain and a listed domain without a claim (failed or uncertain removal) both occupy.
func TestCPUPartitionOccupancyEvidence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		s.AddFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "web").Metadata(), runtimeFinalizer)
		synctest.Wait()

		before := s.fs.recordedWrites()

		swap := acceptedPolicy()
		swap.TypedSpec().Slices[0].CPUs = "6-7"
		s.publish(swap)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseBlocked)

		// The claim is released but libvirt still lists the domain: removal failed or is uncertain.
		s.Create(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "web"))
		s.RemoveFinalizer(hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, "web").Metadata(), runtimeFinalizer)
		synctest.Wait()

		// The listed domain is counted in the virtual machine root, which then blocks the exclusive slice.
		s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Equal(before, s.fs.recordedWrites())

		// A desired stop or a changed specification is not a release either.
		s.vm("web", "database")
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseBlocked)

		s.Destroy(hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "web"))
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("6-7", s.applied()["virtualmachines/database"])
		s.Assert().Equal("4-5", s.applied()["virtualmachines/shared"])
	})
}

// A released machine selecting another partition gives its old placement back before the new one
// is granted, and an exclusive slice is not granted while its previous holder still occupies it.
func TestCPUPartitionReleaseBeforeReuse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.claim("db")
		synctest.Wait()

		// db wants the shared slice now, web the database slice: db still runs in database.
		s.vm("db", "")
		s.vm("web", "database")
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("database", s.placement("db").TypedSpec().Slice, "an occupying machine keeps its placement")
		s.Assert().Nil(s.placement("web"), "the exclusive slice is still held")

		status := s.status()
		s.Require().Len(status.AdmissionErrors, 1)
		s.Assert().Contains(status.AdmissionErrors[0].Reason, "still held by [db]")

		s.releaseDB()
		synctest.Wait()

		s.Require().NotNil(s.placement("db"))
		s.Assert().Equal(hypervisor.VirtualMachineCPUPlacementSpec{Partition: "/virtualmachines.partition/shared.partition"}, *s.placement("db").TypedSpec())
		s.Assert().Equal("1", s.version(s.placement("db")).String(), "a granted placement is withdrawn and granted anew, never modified")
		s.Require().NotNil(s.placement("web"))
		s.Assert().Equal("database", s.placement("web").TypedSpec().Slice)
		s.Assert().Empty(s.status().AdmissionErrors)
	})
}

// Removal withdraws placements and restores nothing until the runtime released every one; the
// status goes last.
func TestCPUPartitionRemovalRestoresAfterRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.set("init", "0-7")
		s.vm("db", "database")
		s.converge(acceptedPolicy())
		s.claim("db")
		s.fs.do(func(fs *fakeCgroupFS) { fs.populated["virtualmachines.partition/database.partition"] = true })
		synctest.Wait()

		before := s.fs.recordedWrites()

		var violations []string

		s.fs.beforeWrite = func(op string) {
			if reason := announced(s.status(), op); reason != "" {
				violations = append(violations, op+": "+reason)
			}
		}

		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseRestoring)
		s.Assert().Contains(status.Waiting, "db")
		s.Assert().Equal(acceptedApplied(), s.applied())
		s.Assert().Equal(before, s.fs.recordedWrites())
		s.Assert().Equal(resource.PhaseTearingDown, s.placement("db").Metadata().Phase())

		s.releaseDB()
		synctest.Wait()

		// The domain's tasks are still in the slice.
		s.requirePhase(runtime.CPUPartitionPhaseRestoring)
		s.Assert().True(s.fs.exists("virtualmachines.partition/database.partition"))

		s.fs.do(func(fs *fakeCgroupFS) { fs.populated["virtualmachines.partition/database.partition"] = false })
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.Assert().Nil(s.status())
		ctest.AssertNoResource[*hypervisor.VirtualMachineCPUPlacement](s, "db")
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Equal("0-7", s.fs.mask("init"))
		s.Assert().Empty(s.fs.mask("virtualmachines.partition"), "the unpopulated inherited root inherits again")
		s.Assert().False(s.fs.exists("virtualmachines.partition/database.partition"))
		s.Assert().False(s.fs.exists("virtualmachines.partition/shared.partition"))

		writes := s.fs.recordedWrites()[len(before):]
		s.Assert().Less(slices.Index(writes, "kubepods=0-7"), slices.Index(writes, "remove virtualmachines.partition/database.partition"))
		s.Assert().Empty(violations, "restoration writes and removals are announced first")
	})
}

// A slice dropped from the policy is left as it is while a machine occupies it; the machine keeps
// its placement until the runtime releases it, then it is refused or placed elsewhere and the
// slice is removed.
func TestCPUPartitionDroppedSliceReleasedAfterItsMachine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.converge(acceptedPolicy())
		s.claim("db")
		synctest.Wait()

		dropped := acceptedPolicy()
		dropped.TypedSpec().Slices = nil
		s.publish(dropped)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Equal("4-7", s.applied()["virtualmachines/shared"])
		s.Assert().Equal("4-5", s.applied()["virtualmachines/database"], "an occupied slice is not given up")
		s.Assert().Contains(status.Waiting, "virtualmachines/database: waiting for the CPU placement of db")
		s.Assert().Equal(resource.PhaseRunning, s.placement("db").Metadata().Phase())

		s.releaseDB()
		synctest.Wait()

		status = s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Nil(s.placement("db"), "the undeclared slice is refused")
		s.Assert().NotContains(s.applied(), "virtualmachines/database")
		s.Assert().False(s.fs.exists("virtualmachines.partition/database.partition"))
		s.Require().Len(status.AdmissionErrors, 1)

		s.vm("db", "")
		synctest.Wait()
		s.Assert().Equal("/virtualmachines.partition/shared.partition", s.placement("db").TypedSpec().Partition)
	})
}

// seedStatus publishes a status as the controller would have before a restart.
func (s *cpuPartitionSuite) seedStatus(spec runtime.CPUPartitionStatusSpec) {
	status := runtime.NewCPUPartitionStatus()
	*status.TypedSpec() = spec

	s.Create(status, state.WithCreateOwner("runtime.CPUPartitionController"))
}

// Tasks below the virtual machine root which no domain, claim or managed slice accounts for (a
// legacy domain's scope after its observation and claim disappeared, or a populated slice no
// resource names any more) share the root's CPUs: an exclusive slice must not be carved under them.
func TestCPUPartitionUnattributedVirtualMachineTasksOccupy(t *testing.T) {
	for _, tc := range []struct {
		name, path, occupant string
	}{
		{"unclaimed scope under the root", "virtualmachines.partition/machine-qemu-legacy.scope", "tasks in virtualmachines.partition/machine-qemu-legacy.scope"},
		{"orphan slice named by no resource", "virtualmachines.partition/old.partition", "tasks in virtualmachines.partition/old.partition"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newCPUPartitionSuite(t)
				defer s.TearDownTest()

				s.fs.do(func(fs *fakeCgroupFS) {
					fs.cpus[tc.path] = ""
					fs.populated[tc.path] = true
					fs.populated["virtualmachines.partition"] = true
				})
				s.vm("db", "database")
				s.converge(acceptedPolicy())

				status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
				s.Assert().Contains(status.Blocked[0].VirtualMachines, tc.occupant, "unattributed tasks are named as the occupant")
				s.Assert().Contains(status.Blocked[0].Reason, "exclusive slice database")
				s.Assert().Nil(s.placement("db"), "no exclusive grant over tasks no resource accounts for")
				s.Assert().Empty(s.fs.mask("virtualmachines.partition/database.partition"))

				// The tasks exit: polling picks it up and the policy applies.
				s.fs.do(func(fs *fakeCgroupFS) {
					fs.populated[tc.path] = false
					fs.populated["virtualmachines.partition"] = false
				})
				synctest.Sleep(time.Second)
				synctest.Wait()

				s.requirePhase(runtime.CPUPartitionPhaseReady)
				s.Assert().NotNil(s.placement("db"))
				s.Assert().True(s.fs.exists(tc.path), "a cgroup Talos does not own is never removed")
			})
		})
	}
}

// A written mask whose effective set does not match (or cannot be read) is not a verified boundary:
// neither the immediate write, nor a retry recovering the intent from the configured mask, nor the
// adoption of an already-configured mask may complete the transition or open admission.
func TestCPUPartitionEffectiveMismatchIsNotEnforcement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(fs *fakeCgroupFS)
		clear   func(fs *fakeCgroupFS)
		initial string
	}{
		{
			name:    "effective set differs from the written mask",
			prepare: func(fs *fakeCgroupFS) { fs.effective["system"] = "0" },
			clear:   func(fs *fakeCgroupFS) { delete(fs.effective, "system") },
		},
		{
			name:    "effective set cannot be read",
			prepare: func(fs *fakeCgroupFS) { fs.effectiveErr["system"] = errors.New("input/output error") },
			clear:   func(fs *fakeCgroupFS) { delete(fs.effectiveErr, "system") },
		},
		{
			name: "already-configured mask adopted with a different effective set",
			prepare: func(fs *fakeCgroupFS) {
				fs.cpus["system"] = "0-1"
				fs.effective["system"] = "0"
			},
			clear:   func(fs *fakeCgroupFS) { delete(fs.effective, "system") },
			initial: "0-1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newCPUPartitionSuite(t)
				defer s.TearDownTest()

				s.fs.do(tc.prepare)
				s.vm("db", "database")
				s.converge(acceptedPolicy())

				// Retries recover the intent from the configured mask; it must still not verify.
				synctest.Sleep(3 * time.Second)
				synctest.Wait()

				status := s.status()
				s.Require().NotNil(status)
				s.Assert().False(status.Phase.AdmissionOpen(), "admission stays closed over an unverified boundary (phase %s)", status.Phase)
				s.Assert().Contains(status.Error, "system")
				s.Assert().Nil(s.placement("db"), "nothing is granted over an unverified boundary")

				system, managed := status.Target("system")
				if tc.initial == "" {
					s.Require().True(managed)
					s.Assert().Empty(system.Initial, "the original inherited mask is kept")
					s.Assert().Empty(system.LastApplied, "an unverified write is not applied")
					s.Assert().Equal("0-1", system.Intended, "the intent stays owned for recovery")
				} else {
					s.Assert().False(managed && system.LastApplied == "0-1", "an unverified mask is not adopted as applied")
				}

				s.fs.do(tc.clear)
				synctest.Sleep(time.Second)
				synctest.Wait()

				status = s.requirePhase(runtime.CPUPartitionPhaseReady)
				system, _ = status.Target("system")
				s.Assert().Equal("0-1", system.LastApplied)
				s.Assert().Equal(tc.initial, system.Initial)
				s.Assert().NotNil(s.placement("db"))
			})
		})
	}
}

// After a restart, a status with a pending intent the kernel holds (status publication failed after
// the write) is confirmed only once the effective set verifies.
func TestCPUPartitionRecoveredIntentVerifiesEffective(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.fs.do(func(fs *fakeCgroupFS) {
			fs.cpus["init"] = "0-1"
			fs.effective["init"] = "0"
		})
		s.seedStatus(runtime.CPUPartitionStatusSpec{
			Phase:   runtime.CPUPartitionPhaseApplying,
			Targets: []runtime.CPUPartitionTargetStatus{{Key: "init", Initial: "", LastApplied: "", Intended: "0-1"}},
		})

		spec := runtime.NewCPUPartitionSpec()
		spec.TypedSpec().Enabled = true
		spec.TypedSpec().Roots = map[string]string{"init": "0-1"}

		s.start()
		s.publish(spec)
		synctest.Wait()

		status := s.status()
		s.Assert().False(status.Phase.AdmissionOpen(), "phase %s", status.Phase)
		s.Assert().Contains(status.Error, "init")

		target, _ := status.Target("init")
		s.Assert().Equal(runtime.CPUPartitionTargetStatus{Key: "init", Intended: "0-1"}, target, "the intent is neither confirmed nor dropped")

		s.fs.do(func(fs *fakeCgroupFS) { delete(fs.effective, "init") })
		synctest.Sleep(time.Second)
		synctest.Wait()

		status = s.requirePhase(runtime.CPUPartitionPhaseReady)
		target, _ = status.Target("init")
		s.Assert().Equal(runtime.CPUPartitionTargetStatus{Key: "init", LastApplied: "0-1", Intended: "0-1"}, target)
		s.Assert().NotContains(s.fs.recordedWrites(), "init=0-1", "confirmed from the kernel, not rewritten")
	})
}

// A populated root whose original mask was inherited is put back on inheritance on removal: the
// obligation stays in the status, announced, until the empty mask is actually written, and a
// kernel refusal keeps it pending rather than claiming restoration.
func TestCPUPartitionInheritedPopulatedRootRestoration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		spec := runtime.NewCPUPartitionSpec()
		spec.TypedSpec().Enabled = true
		spec.TypedSpec().Roots = map[string]string{"system": "0"}

		s.start()
		s.publish(spec)
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		var violations []string

		s.fs.beforeWrite = func(op string) {
			if reason := announced(s.status(), op); reason != "" {
				violations = append(violations, op+": "+reason)
			}
		}

		s.fs.do(func(fs *fakeCgroupFS) { fs.emptyErr["system"] = errors.New("no space left on device") })
		s.publish(runtime.NewCPUPartitionSpec())
		synctest.Wait()
		synctest.Sleep(3 * time.Second)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseRestoring)
		system, managed := status.Target("system")
		s.Require().True(managed, "a refused empty write must not drop the restoration obligation")
		s.Assert().Empty(system.Initial)
		s.Assert().Equal("0-7", system.LastApplied)
		s.Assert().Contains(status.Error, "no space left on device")
		s.Assert().Equal("0-7", s.fs.mask("system"))

		s.fs.do(func(fs *fakeCgroupFS) { delete(fs.emptyErr, "system") })
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.Assert().Nil(s.status(), "restoration completes and the guard goes last")
		s.Assert().Empty(s.fs.mask("system"), "the populated root inherits again")
		s.Assert().Equal(&k8s.KubeletCPUReservationSpec{}, s.reservation())
		s.Assert().Empty(violations, "the empty restoration write is announced first")
	})
}

// Tasks no domain, claim or placement accounts for in a slice the policy declares exclusive (a
// previous owner whose state was lost) must not get a new exclusive owner: the new machine is
// refused while they run, polling notices their exit, and a legitimate holder keeps its placement.
func TestCPUPartitionOrphanTasksInDesiredExclusiveSlice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		const slice = "virtualmachines.partition/database.partition"

		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		s.fs.do(func(fs *fakeCgroupFS) { fs.populated[slice] = true })
		s.vm("db", "database")
		synctest.Wait()

		status := s.status()
		s.Require().NotNil(status)
		s.Assert().Nil(s.placement("db"), "no new exclusive owner over tasks no machine accounts for")
		s.Assert().NotNil(s.placement("web"), "unrelated machines stay admitted")
		s.Require().Len(status.AdmissionErrors, 1)
		s.Assert().Equal([]string{"db"}, status.AdmissionErrors[0].VirtualMachines)
		s.Assert().Contains(status.AdmissionErrors[0].Reason, "tasks no machine accounts for")

		s.fs.do(func(fs *fakeCgroupFS) { fs.populated[slice] = false })
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.Require().NotNil(s.placement("db"), "polling notices the unattributed tasks exited")
		s.Assert().Empty(s.status().AdmissionErrors)

		// The legitimate owner's own tasks keep its placement.
		s.claim("db")
		s.fs.do(func(fs *fakeCgroupFS) { fs.populated[slice] = true })
		s.wake("cpu1")
		synctest.Wait()

		s.Assert().Equal(resource.PhaseRunning, s.placement("db").Metadata().Phase())
		s.Assert().Empty(s.status().AdmissionErrors)
	})
}

// A written mask pending effective verification outranks the classification of a new desired
// policy: an invalid or planner-blocked policy must not publish an admission-open phase over an
// unverified boundary, and polling must keep checking until the effective set is in effect.
func TestCPUPartitionUnverifiedIntentOutranksDesiredRejection(t *testing.T) {
	narrowed := func() *runtime.CPUPartitionSpec {
		spec := acceptedPolicy()
		spec.TypedSpec().Roots["system"] = "0"

		return spec
	}

	for _, tc := range []struct {
		name    string
		desired func() *runtime.CPUPartitionSpec
	}{
		{"invalid desired policy", func() *runtime.CPUPartitionSpec {
			spec := narrowed()
			spec.TypedSpec().Roots["init"] = "0-"

			return spec
		}},
		{"planner-blocked desired policy", func() *runtime.CPUPartitionSpec {
			spec := narrowed()
			spec.TypedSpec().Slices[0].CPUs = "4"

			return spec
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newCPUPartitionSuite(t)
				defer s.TearDownTest()

				s.vm("db", "database")
				s.converge(acceptedPolicy())
				s.claim("db")
				synctest.Wait()

				// The write lands, but the kernel keeps the old effective set.
				s.fs.do(func(fs *fakeCgroupFS) { fs.effective["system"] = "0-1" })
				s.publish(narrowed())
				synctest.Wait()

				status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
				system, _ := status.Target("system")
				s.Require().Equal("0", system.Intended)
				s.Require().Equal("0-1", system.LastApplied)

				s.publish(tc.desired())
				synctest.Wait()
				synctest.Sleep(3 * time.Second)
				synctest.Wait()

				status = s.status()
				s.Require().NotNil(status)
				s.Assert().False(status.Phase.AdmissionOpen(), "pending verification outranks the desired rejection (phase %s, blocked %v)", status.Phase, status.Blocked)
				s.Assert().Contains(status.Error, "system", "the verification failure stays reported")

				system, _ = status.Target("system")
				s.Assert().Equal("0", system.Intended, "the unverified intent stays owned")
				s.Assert().Equal("0-1", system.LastApplied)

				// Polling keeps verifying; once in effect, the rejection is published over a verified state.
				s.fs.do(func(fs *fakeCgroupFS) { delete(fs.effective, "system") })
				synctest.Sleep(time.Second)
				synctest.Wait()

				status = s.requirePhase(runtime.CPUPartitionPhaseBlocked)
				system, _ = status.Target("system")
				s.Assert().Equal("0", system.LastApplied)
				s.Assert().NotEmpty(status.Blocked)
				s.Assert().Empty(status.Error)
			})
		})
	}
}

func TestCPUPartitionBarrierIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.kubelet("0-1,4-7")
		s.startWith(&runtimectrls.CPUPartitionController{V1Alpha1Mode: machineruntime.ModeMetal, BarrierTimeout: 10 * time.Second})
		s.publish(acceptedPolicy())
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves["burstable/pod-a/app"] = mustCPUs("2") })
		s.kubelet("0-2,4-7")

		shrink := acceptedPolicy()
		shrink.TypedSpec().Roots["kubepods"] = "3"
		s.publish(shrink)
		synctest.Wait()

		s.vm("db", "database")
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Equal(`kubepods leaf "burstable/pod-a/app" still runs on CPUs 2`, status.Waiting)
		s.Assert().Empty(status.Error)
		s.Assert().Nil(s.placement("db"), "nothing is granted while the transition is pending")

		synctest.Sleep(11 * time.Second)
		synctest.Wait()

		status = s.requirePhase(runtime.CPUPartitionPhaseConverging)
		s.Assert().Contains(status.Error, "did not converge within 10s")
		s.Assert().Equal("2-3", s.applied()["kubepods"])

		s.fs.do(func(fs *fakeCgroupFS) { fs.leaves["burstable/pod-a/app"] = mustCPUs("3") })
		synctest.Sleep(time.Second)
		synctest.Wait()

		s.requirePhase(runtime.CPUPartitionPhaseReady)
		s.Assert().Empty(s.status().Error)
		s.Assert().Equal("3", s.applied()["kubepods"])
		s.Assert().NotNil(s.placement("db"))

		calls := s.fs.callCount()

		synctest.Sleep(time.Hour)
		synctest.Wait()
		s.Assert().Equal(calls, s.fs.callCount(), "no polling once nothing is pending")
	})
}

func TestCPUPartitionInvalidDesiredPolicyIsRejected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.converge(acceptedPolicy())
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		before := s.fs.recordedWrites()

		invalid := acceptedPolicy()
		invalid.TypedSpec().Roots["init"] = "0-"
		s.publish(invalid)
		synctest.Wait()

		status := s.requirePhase(runtime.CPUPartitionPhaseBlocked)
		s.Assert().Contains(status.Blocked[0].Reason, `root "init": invalid CPU list "0-"`)
		s.Assert().Equal(before, s.fs.recordedWrites())
	})
}

func TestCPUPartitionSteadyStateIsQuiet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newCPUPartitionSuite(t)
		defer s.TearDownTest()

		s.vm("db", "database")
		s.vm("web", "")
		s.converge(acceptedPolicy())
		s.claim("db")
		synctest.Wait()
		s.requirePhase(runtime.CPUPartitionPhaseReady)

		status, reservation := runtime.NewCPUPartitionStatus(), k8s.NewKubeletCPUReservation()
		statusVersion, reservationVersion, dbVersion := s.version(status), s.version(reservation), s.version(s.placement("db"))
		writes := s.fs.recordedWrites()

		s.claim("web")
		synctest.Wait()
		s.wake("cpu3")
		synctest.Wait()
		s.kubelet("0-1,4-7")
		synctest.Wait()

		s.Assert().Equal(statusVersion, s.version(status))
		s.Assert().Equal(reservationVersion, s.version(reservation))
		s.Assert().Equal(dbVersion, s.version(s.placement("db")))
		s.Assert().Equal(writes, s.fs.recordedWrites())
	})
}
